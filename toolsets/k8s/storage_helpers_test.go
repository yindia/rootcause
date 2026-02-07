package k8s

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sfake "k8s.io/client-go/kubernetes/fake"

	"rootcause/internal/config"
	"rootcause/internal/kube"
	"rootcause/internal/mcp"
	"rootcause/internal/policy"
	"rootcause/internal/redact"
	"rootcause/internal/render"
)

func TestCollectPVCNamesFromPod(t *testing.T) {
	pod := &corev1.Pod{Spec: corev1.PodSpec{Volumes: []corev1.Volume{
		{Name: "data", VolumeSource: corev1.VolumeSource{PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: "pvc-a"}}},
		{Name: "logs", VolumeSource: corev1.VolumeSource{PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: "pvc-b"}}},
	}}}
	names := collectPVCNamesFromPod(pod)
	if len(names) != 2 {
		t.Fatalf("expected 2 pvc names, got %d", len(names))
	}
}

func TestFindMatchingPVs(t *testing.T) {
	mode := corev1.PersistentVolumeFilesystem
	pvc := &corev1.PersistentVolumeClaim{Spec: corev1.PersistentVolumeClaimSpec{AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce}, VolumeMode: &mode}}
	pv := &corev1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{Name: "pv-1"},
		Spec: corev1.PersistentVolumeSpec{StorageClassName: "standard", AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce}, VolumeMode: &mode},
		Status: corev1.PersistentVolumeStatus{Phase: corev1.VolumeAvailable},
	}
	client := k8sfake.NewSimpleClientset(pv)
	cfg := config.DefaultConfig()
	toolset := New()
	_ = toolset.Init(mcp.ToolsetContext{
		Config:   &cfg,
		Clients:  &kube.Clients{Typed: client},
		Policy:   policy.NewAuthorizer(),
		Renderer: render.NewRenderer(),
		Redactor: redact.New(),
	})
	matches, err := toolset.findMatchingPVs(context.Background(), pvc, "standard")
	if err != nil {
		t.Fatalf("findMatchingPVs: %v", err)
	}
	if len(matches) != 1 || matches[0] != "pv-1" {
		t.Fatalf("unexpected matches: %#v", matches)
	}
	if !accessModesMatch([]corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce}, []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce}) {
		t.Fatalf("expected access modes to match")
	}
}

func TestHandleStorageDebugPod(t *testing.T) {
	namespace := "default"
	storageClass := "standard"
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{Name: "data", Namespace: namespace, UID: "pvc-uid"},
		Spec: corev1.PersistentVolumeClaimSpec{
			StorageClassName: &storageClass,
			AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
		},
		Status: corev1.PersistentVolumeClaimStatus{Phase: corev1.ClaimPending},
	}
	sc := &storagev1.StorageClass{ObjectMeta: metav1.ObjectMeta{Name: storageClass}, Provisioner: "kubernetes.io/no-provisioner"}
	pv := &corev1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{Name: "pv-1"},
		Spec:       corev1.PersistentVolumeSpec{StorageClassName: storageClass, AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce}},
		Status:     corev1.PersistentVolumeStatus{Phase: corev1.VolumeAvailable},
	}
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "app", Namespace: namespace},
		Spec:       corev1.PodSpec{Volumes: []corev1.Volume{{Name: "data", VolumeSource: corev1.VolumeSource{PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: "data"}}}}},
	}
	event := &corev1.Event{ObjectMeta: metav1.ObjectMeta{Name: "pvc-event", Namespace: namespace}, InvolvedObject: corev1.ObjectReference{UID: pvc.UID}, Reason: "ProvisioningFailed"}

	client := k8sfake.NewSimpleClientset(pvc, pv, pod, sc, event)
	clients := &kube.Clients{Typed: client}
	cfg := config.DefaultConfig()
	toolset := New()
	_ = toolset.Init(mcp.ToolsetContext{Config: &cfg, Clients: clients, Policy: policy.NewAuthorizer(), Renderer: render.NewRenderer(), Redactor: redact.New()})

	_, err := toolset.handleStorageDebug(context.Background(), mcp.ToolRequest{
		User: policy.User{Role: policy.RoleCluster},
		Arguments: map[string]any{
			"namespace": namespace,
			"pod":       "app",
		},
	})
	if err != nil {
		t.Fatalf("handleStorageDebug: %v", err)
	}
}

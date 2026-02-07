package k8s

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
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

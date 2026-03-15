package k8s

import (
	"context"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sfake "k8s.io/client-go/kubernetes/fake"

	"rootcause/internal/config"
	"rootcause/internal/kube"
	"rootcause/internal/mcp"
	"rootcause/internal/policy"
	"rootcause/internal/redact"
	"rootcause/internal/render"
)

func TestHandleBestPracticeDeployment(t *testing.T) {
	replicas := int32(1)
	client := k8sfake.NewSimpleClientset(&appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "api", Namespace: "default"},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "api"}},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "api"}},
				Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "api", Image: "example/api:latest"}}},
			},
		},
	})

	cfg := config.DefaultConfig()
	toolset := New()
	_ = toolset.Init(mcp.ToolsetContext{Config: &cfg, Clients: &kube.Clients{Typed: client}, Policy: policy.NewAuthorizer(), Renderer: render.NewRenderer(), Redactor: redact.New()})

	result, err := toolset.handleBestPractice(context.Background(), mcp.ToolRequest{User: policy.User{Role: policy.RoleCluster, AllowedNamespaces: []string{"*"}}, Arguments: map[string]any{"kind": "Deployment", "name": "api", "namespace": "default"}})
	if err != nil {
		t.Fatalf("handleBestPractice: %v", err)
	}
	root := result.Data.(map[string]any)
	if compliant, ok := root["compliant"].(bool); !ok || compliant {
		t.Fatalf("expected non-compliant deployment result")
	}
	if score := toInt(root["score"], 0); score >= 100 {
		t.Fatalf("expected degraded score, got %d", score)
	}
}

func TestHandleBestPracticeDeploymentPVCResilience(t *testing.T) {
	replicas := int32(3)
	grace := int64(5)
	immediate := storagev1.VolumeBindingImmediate
	client := k8sfake.NewSimpleClientset(
		&appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{Name: "api", Namespace: "default"},
			Spec: appsv1.DeploymentSpec{
				Replicas: &replicas,
				Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "api"}},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "api"}},
					Spec: corev1.PodSpec{
						TerminationGracePeriodSeconds: &grace,
						NodeSelector:                  map[string]string{"topology.kubernetes.io/zone": "us-east-1a"},
						Volumes: []corev1.Volume{{
							Name:         "data",
							VolumeSource: corev1.VolumeSource{PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: "shared-data"}},
						}},
						Containers: []corev1.Container{{
							Name:  "api",
							Image: "example/api:v1.2.3",
							VolumeMounts: []corev1.VolumeMount{{
								Name:      "data",
								MountPath: "/data",
							}},
							ReadinessProbe: &corev1.Probe{},
							LivenessProbe:  &corev1.Probe{},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{corev1.ResourceCPU: resourceMustParse("100m"), corev1.ResourceMemory: resourceMustParse("128Mi")},
								Limits:   corev1.ResourceList{corev1.ResourceCPU: resourceMustParse("500m"), corev1.ResourceMemory: resourceMustParse("512Mi")},
							},
							SecurityContext: &corev1.SecurityContext{AllowPrivilegeEscalation: boolPtr(false), ReadOnlyRootFilesystem: boolPtr(true), Capabilities: &corev1.Capabilities{Drop: []corev1.Capability{"ALL"}}},
						}},
						SecurityContext: &corev1.PodSecurityContext{RunAsNonRoot: boolPtr(true), SeccompProfile: &corev1.SeccompProfile{Type: corev1.SeccompProfileTypeRuntimeDefault}},
					},
				},
			},
		},
		&corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{Name: "shared-data", Namespace: "default"},
			Spec: corev1.PersistentVolumeClaimSpec{
				AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
				StorageClassName: strPtr("gp2"),
				VolumeName:       "pv-shared",
			},
		},
		&storagev1.StorageClass{
			ObjectMeta:        metav1.ObjectMeta{Name: "gp2"},
			Provisioner:       "ebs.csi.aws.com",
			VolumeBindingMode: &immediate,
		},
		&storagev1.VolumeAttachment{
			ObjectMeta: metav1.ObjectMeta{Name: "va-old"},
			Spec: storagev1.VolumeAttachmentSpec{
				NodeName: "node-a",
				Attacher: "ebs.csi.aws.com",
				Source:   storagev1.VolumeAttachmentSource{PersistentVolumeName: strPtr("pv-shared")},
			},
			Status: storagev1.VolumeAttachmentStatus{Attached: true},
		},
		&storagev1.VolumeAttachment{
			ObjectMeta: metav1.ObjectMeta{Name: "va-new"},
			Spec: storagev1.VolumeAttachmentSpec{
				NodeName: "node-b",
				Attacher: "ebs.csi.aws.com",
				Source:   storagev1.VolumeAttachmentSource{PersistentVolumeName: strPtr("pv-shared")},
			},
			Status: storagev1.VolumeAttachmentStatus{DetachError: &storagev1.VolumeError{Message: "timed out detaching"}},
		},
	)

	cfg := config.DefaultConfig()
	toolset := New()
	_ = toolset.Init(mcp.ToolsetContext{Config: &cfg, Clients: &kube.Clients{Typed: client}, Policy: policy.NewAuthorizer(), Renderer: render.NewRenderer(), Redactor: redact.New()})

	result, err := toolset.handleBestPractice(context.Background(), mcp.ToolRequest{User: policy.User{Role: policy.RoleCluster, AllowedNamespaces: []string{"*"}}, Arguments: map[string]any{"kind": "Deployment", "name": "api", "namespace": "default"}})
	if err != nil {
		t.Fatalf("handleBestPractice: %v", err)
	}

	root := result.Data.(map[string]any)
	assertCheckStatus(t, root["checks"], "pvc-termination-grace", "fail")
	assertCheckStatus(t, root["checks"], "container:api:pvc-prestop", "warn")
	assertCheckStatus(t, root["checks"], "pvc-shared-rwo", "fail")
	assertCheckStatus(t, root["checks"], "storageclass:gp2:binding-mode", "fail")
	assertCheckStatus(t, root["checks"], "pvc-detach-error:shared-data", "fail")
	assertCheckStatus(t, root["checks"], "pvc-attach-churn:shared-data", "warn")
	assertCheckStatus(t, root["checks"], "node-spread", "warn")
}

func assertCheckStatus(t *testing.T, checksRaw any, id, status string) {
	t.Helper()
	switch checks := checksRaw.(type) {
	case []bestPracticeCheck:
		for _, item := range checks {
			if item.ID != id {
				continue
			}
			if item.Status != status {
				t.Fatalf("expected check %s status %s, got %s", id, status, item.Status)
			}
			return
		}
	case []any:
		for _, raw := range checks {
			item, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			if toString(item["id"]) != id {
				continue
			}
			if got := toString(item["status"]); got != status {
				t.Fatalf("expected check %s status %s, got %s", id, status, got)
			}
			return
		}
	default:
		t.Fatalf("unexpected checks type: %T", checksRaw)
	}
	t.Fatalf("expected check %s", id)
}

func boolPtr(v bool) *bool {
	return &v
}

func strPtr(v string) *string {
	return &v
}

func resourceMustParse(v string) resource.Quantity {
	q, err := resource.ParseQuantity(v)
	if err != nil {
		panic(err)
	}
	return q
}

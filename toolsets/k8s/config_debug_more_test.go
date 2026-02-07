package k8s

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"rootcause/internal/config"
	"rootcause/internal/evidence"
	"rootcause/internal/kube"
	"rootcause/internal/mcp"
	"rootcause/internal/policy"
	"rootcause/internal/redact"
	"rootcause/internal/render"
)

func TestHandleConfigDebugErrors(t *testing.T) {
	toolset := New()
	cfg := config.DefaultConfig()
	client := fake.NewSimpleClientset()
	clients := &kube.Clients{Typed: client}
	_ = toolset.Init(mcp.ToolsetContext{
		Config:   &cfg,
		Clients:  clients,
		Policy:   policy.NewAuthorizer(),
		Renderer: render.NewRenderer(),
		Redactor: redact.New(),
		Evidence: evidence.NewCollector(clients),
	})

	if _, err := toolset.handleConfigDebug(context.Background(), mcp.ToolRequest{
		User:      policy.User{Role: policy.RoleCluster},
		Arguments: map[string]any{},
	}); err == nil {
		t.Fatalf("expected namespace error")
	}

	if _, err := toolset.handleConfigDebug(context.Background(), mcp.ToolRequest{
		User:      policy.User{Role: policy.RoleCluster},
		Arguments: map[string]any{"namespace": "default"},
	}); err == nil {
		t.Fatalf("expected pod or name error")
	}

	if _, err := toolset.handleConfigDebug(context.Background(), mcp.ToolRequest{
		User: policy.User{Role: policy.RoleCluster},
		Arguments: map[string]any{
			"namespace": "default",
			"kind":      "widget",
			"name":      "demo",
		},
	}); err == nil {
		t.Fatalf("expected unsupported kind error")
	}
}

func TestInspectPodConfigRefsProjected(t *testing.T) {
	namespace := "default"
	optional := true
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "cfg", Namespace: namespace},
		Data:       map[string]string{"foo": "bar"},
	}
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "sec", Namespace: namespace},
		Data:       map[string][]byte{"token": []byte("value")},
	}
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "app", Namespace: namespace},
		Spec: corev1.PodSpec{
			InitContainers: []corev1.Container{
				{
					Name: "init",
					EnvFrom: []corev1.EnvFromSource{
						{ConfigMapRef: &corev1.ConfigMapEnvSource{
							LocalObjectReference: corev1.LocalObjectReference{Name: "missing-cm"},
							Optional:             &optional,
						}},
					},
				},
			},
			Containers: []corev1.Container{
				{
					Name: "app",
					Env: []corev1.EnvVar{
						{
							Name: "CFG_KEY",
							ValueFrom: &corev1.EnvVarSource{
								ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
									LocalObjectReference: corev1.LocalObjectReference{Name: "cfg"},
									Key:                  "missing-key",
								},
							},
						},
						{
							Name: "SECRET_KEY",
							ValueFrom: &corev1.EnvVarSource{
								SecretKeyRef: &corev1.SecretKeySelector{
									LocalObjectReference: corev1.LocalObjectReference{Name: "sec"},
									Key:                  "missing-secret-key",
								},
							},
						},
					},
				},
			},
			Volumes: []corev1.Volume{
				{
					Name: "config-projected",
					VolumeSource: corev1.VolumeSource{
						Projected: &corev1.ProjectedVolumeSource{
							Sources: []corev1.VolumeProjection{
								{
									ConfigMap: &corev1.ConfigMapProjection{
										LocalObjectReference: corev1.LocalObjectReference{Name: "cfg"},
										Items:                []corev1.KeyToPath{{Key: "foo"}, {Key: "missing"}},
									},
								},
								{
									Secret: &corev1.SecretProjection{
										LocalObjectReference: corev1.LocalObjectReference{Name: "sec"},
										Items:                []corev1.KeyToPath{{Key: "token"}, {Key: "missing"}},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	client := fake.NewSimpleClientset(cm, secret, pod)
	clients := &kube.Clients{Typed: client}
	cfg := config.DefaultConfig()
	toolset := New()
	_ = toolset.Init(mcp.ToolsetContext{
		Config:   &cfg,
		Clients:  clients,
		Policy:   policy.NewAuthorizer(),
		Renderer: render.NewRenderer(),
		Redactor: redact.New(),
		Evidence: evidence.NewCollector(clients),
	})

	issues := toolset.inspectPodConfigRefs(context.Background(), namespace, pod)
	if len(issues) == 0 {
		t.Fatalf("expected config issues")
	}

	missing := toolset.checkConfigMapKeys(context.Background(), namespace, "", nil, "direct", false, "")
	if !missing.Missing {
		t.Fatalf("expected missing configmap when name empty")
	}
	missingSecret := toolset.checkSecretKeys(context.Background(), namespace, "missing-sec", []string{"token"}, "direct", false, "")
	if !missingSecret.Missing {
		t.Fatalf("expected missing secret")
	}
}

package k8s

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
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

func TestHandlePermissionDebug(t *testing.T) {
	namespace := "default"
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{Name: "app", Namespace: namespace},
		AutomountServiceAccountToken: func() *bool { v := false; return &v }(),
	}
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "app", Namespace: namespace},
		Spec:       corev1.PodSpec{ServiceAccountName: "app"},
	}
	role := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{Name: "role", Namespace: namespace},
		Rules:      []rbacv1.PolicyRule{{APIGroups: []string{""}, Resources: []string{"pods"}, Verbs: []string{"get"}}},
	}
	roleBinding := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{Name: "rb", Namespace: namespace},
		Subjects:   []rbacv1.Subject{{Kind: "ServiceAccount", Name: "app", Namespace: namespace}},
		RoleRef:    rbacv1.RoleRef{Kind: "Role", Name: "role"},
	}
	clusterRole := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{Name: "cr"},
		Rules:      []rbacv1.PolicyRule{{APIGroups: []string{""}, Resources: []string{"nodes"}, Verbs: []string{"list"}}},
	}
	clusterRoleBinding := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{Name: "crb"},
		Subjects:   []rbacv1.Subject{{Kind: "ServiceAccount", Name: "app", Namespace: namespace}},
		RoleRef:    rbacv1.RoleRef{Kind: "ClusterRole", Name: "cr"},
	}

	client := fake.NewSimpleClientset(sa, pod, role, roleBinding, clusterRole, clusterRoleBinding)
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
		Registry: mcp.NewRegistry(&cfg),
	})

	_, err := toolset.handlePermissionDebug(context.Background(), mcp.ToolRequest{
		User: policy.User{Role: policy.RoleCluster},
		Arguments: map[string]any{
			"namespace": namespace,
			"pod":       "app",
		},
	})
	if err != nil {
		t.Fatalf("handlePermissionDebug: %v", err)
	}
}

func TestRoleNameFromARN(t *testing.T) {
	if _, err := roleNameFromARN("bad"); err == nil {
		t.Fatalf("expected error for invalid arn")
	}
	name, err := roleNameFromARN("arn:aws:iam::123:role/path/demo")
	if err != nil || name != "demo" {
		t.Fatalf("unexpected role name: %s err=%v", name, err)
	}
}

func TestAddAWSRoleEvidenceRegistryMissing(t *testing.T) {
	analysis := render.NewAnalysis()
	toolset := New()
	toolset.ctx = mcp.ToolsetContext{}
	toolset.addAWSRoleEvidence(context.Background(), mcp.ToolRequest{}, &analysis, "demo", "", "default", "sa")
	if len(analysis.Evidence) == 0 {
		t.Fatalf("expected evidence to be added")
	}
}

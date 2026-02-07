package k8s

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"rootcause/internal/config"
	"rootcause/internal/kube"
	"rootcause/internal/mcp"
)

func TestToolsetInitAndRegister(t *testing.T) {
	toolset := New()
	if err := toolset.Init(mcp.ToolsetContext{}); err == nil {
		t.Fatalf("expected error for missing clients")
	}
	cfg := config.DefaultConfig()
	ctx := mcp.ToolsetContext{Clients: &kube.Clients{}, Config: &cfg}
	if err := toolset.Init(ctx); err != nil {
		t.Fatalf("init: %v", err)
	}
	reg := mcp.NewRegistry(&cfg)
	if err := toolset.Register(reg); err != nil {
		t.Fatalf("register: %v", err)
	}
	if _, ok := reg.Get("k8s.get"); !ok {
		t.Fatalf("expected k8s.get to be registered")
	}
}

func TestToolsetRegisterExecReadonly(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Exec.Enabled = true
	cfg.Exec.AllowedCommands = []string{"ls"}
	toolset := New()
	if err := toolset.Init(mcp.ToolsetContext{Clients: &kube.Clients{}, Config: &cfg}); err != nil {
		t.Fatalf("init: %v", err)
	}
	reg := mcp.NewRegistry(&cfg)
	if err := toolset.Register(reg); err != nil {
		t.Fatalf("register: %v", err)
	}
	if _, ok := reg.Get("k8s.exec_readonly"); !ok {
		t.Fatalf("expected exec_readonly tool")
	}
}

func TestConfirmAndArgsHelpers(t *testing.T) {
	if err := requireConfirm(map[string]any{"confirm": true}); err != nil {
		t.Fatalf("expected confirm to pass: %v", err)
	}
	if err := requireConfirm(map[string]any{}); err == nil {
		t.Fatalf("expected confirm error")
	}
	toolset := New()
	if err := toolset.requireArgs(map[string]any{"a": 1}, "a", "b"); err == nil {
		t.Fatalf("expected missing arg error")
	}
}

func TestStringHelpers(t *testing.T) {
	if got := toString(nil); got != "" {
		t.Fatalf("expected empty string")
	}
	if got := toString(7); got != "7" {
		t.Fatalf("unexpected toString: %q", got)
	}
	if got := toStringSlice([]any{"a", 1}); len(got) != 1 || got[0] != "a" {
		t.Fatalf("unexpected toStringSlice: %#v", got)
	}
	if got := toStringSlice("bad"); got != nil {
		t.Fatalf("expected nil slice for non-list input")
	}
}

func TestPortParsing(t *testing.T) {
	local, remote, err := parsePortSpec("8080:80")
	if err != nil || local != "8080" || remote != "80" {
		t.Fatalf("unexpected port spec result: %s %s %v", local, remote, err)
	}
	local, remote, err = parsePortSpec("9090")
	if err != nil || local != "9090" || remote != "9090" {
		t.Fatalf("unexpected single port result: %s %s %v", local, remote, err)
	}
	if _, _, err := parsePortSpec(""); err == nil {
		t.Fatalf("expected error for empty port spec")
	}
	if _, ok := parsePortNumber("123"); !ok {
		t.Fatalf("expected numeric port")
	}
	if _, ok := parsePortNumber("abc"); ok {
		t.Fatalf("expected non-numeric port rejection")
	}
}

func TestNamespaceHelpers(t *testing.T) {
	toolset := New()
	if err := toolset.checkAllowedNamespace([]string{"default"}, "default"); err != nil {
		t.Fatalf("expected allowed namespace")
	}
	if err := toolset.checkAllowedNamespace([]string{"default"}, "other"); err == nil {
		t.Fatalf("expected namespace restriction")
	}
	if got := resourceRef("pods", "default", "api"); got != "pods/default/api" {
		t.Fatalf("unexpected resourceRef: %s", got)
	}
	if got := resourceRef("namespaces", "", "default"); got != "namespaces/default" {
		t.Fatalf("unexpected cluster resourceRef: %s", got)
	}
}

func TestWorkloadHelpers(t *testing.T) {
	if !isWorkloadKind("Deployment") {
		t.Fatalf("expected workload kind")
	}
	if isWorkloadKind("Job") {
		t.Fatalf("did not expect job as workload kind")
	}
	if got := sliceIf(""); got != nil {
		t.Fatalf("expected nil slice")
	}
	if got := sliceIf("ns"); len(got) != 1 || got[0] != "ns" {
		t.Fatalf("unexpected sliceIf: %#v", got)
	}
}

func TestPodHelpers(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "demo",
			Annotations: map[string]string{corev1.MirrorPodAnnotationKey: "true"},
			OwnerReferences: []metav1.OwnerReference{
				{Kind: "DaemonSet"},
			},
		},
		Status: corev1.PodStatus{
			Conditions: []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue}},
		},
	}
	if !isPodReady(pod) {
		t.Fatalf("expected pod ready")
	}
	if !isDaemonSetPod(pod) {
		t.Fatalf("expected daemonset pod")
	}
	if !isMirrorPod(pod) {
		t.Fatalf("expected mirror pod")
	}
	if _, err := toUnstructured(pod); err != nil {
		t.Fatalf("toUnstructured: %v", err)
	}
}

func TestIstioPrincipalParsing(t *testing.T) {
	ns, sa, ok := parseIstioServiceAccountPrincipal("spiffe://cluster.local/ns/default/sa/app")
	if !ok || ns != "default" || sa != "app" {
		t.Fatalf("unexpected principal parse: %s %s %v", ns, sa, ok)
	}
	if _, _, ok := parseIstioServiceAccountPrincipal("invalid"); ok {
		t.Fatalf("expected invalid principal")
	}
	obj := &unstructured.Unstructured{Object: map[string]any{
		"spec": map[string]any{
			"rules": []any{
				map[string]any{
					"from": []any{
						map[string]any{"source": map[string]any{"principals": []any{"spiffe://cluster.local/ns/default/sa/app"}}},
					},
				},
			},
		},
	}}
	principals := istioAuthorizationPolicyPrincipals(obj)
	if len(principals) != 1 {
		t.Fatalf("expected principals, got %#v", principals)
	}
}

func TestMetricsHelpers(t *testing.T) {
	if got := toBool(nil, true); got != true {
		t.Fatalf("expected fallback toBool")
	}
	if got := toInt(float64(3), 1); got != 3 {
		t.Fatalf("unexpected toInt: %d", got)
	}
}

func TestIsDeploymentTarget(t *testing.T) {
	if !isDeploymentTarget("apps/v1", "Deployment", "") {
		t.Fatalf("expected deployment target")
	}
	if !isDeploymentTarget("", "", "") {
		t.Fatalf("expected default deployment target")
	}
	if isDeploymentTarget("v1", "Service", "service") {
		t.Fatalf("did not expect service target")
	}
}

func TestGraphHelpers(t *testing.T) {
	graphData := map[string]any{
		"nodes": []any{
			map[string]any{"id": "svc", "kind": "Service", "name": "svc", "namespace": "ns"},
			map[string]any{"id": "ep", "kind": "Endpoints", "name": "svc", "namespace": "ns"},
			map[string]any{"id": "pod", "kind": "Pod", "name": "pod-1", "namespace": "ns"},
			map[string]any{"id": "deploy", "kind": "Deployment", "name": "deploy", "namespace": "ns"},
		},
		"edges": []any{
			map[string]any{"from": "svc", "to": "ep", "relation": "selects"},
			map[string]any{"from": "ep", "to": "pod", "relation": "selects"},
			map[string]any{"from": "pod", "to": "deploy", "relation": "owned-by"},
		},
		"warnings": []any{"warn"},
	}
	view, warnings, err := parseGraph(graphData)
	if err != nil {
		t.Fatalf("parseGraph: %v", err)
	}
	if len(warnings) != 1 {
		t.Fatalf("expected warnings")
	}
	if id := findNodeByName(view.nodes, "Service", "ns", "svc"); id != "svc" {
		t.Fatalf("unexpected node id: %s", id)
	}
	services := []flowNode{view.nodes["svc"]}
	pods := relatedPodsForServices(view, services)
	if len(pods) != 1 || pods[0].ID != "pod" {
		t.Fatalf("unexpected pods: %#v", pods)
	}
	workloads := relatedWorkloadsForPods(view, pods)
	if len(workloads) != 1 || workloads[0].ID != "deploy" {
		t.Fatalf("unexpected workloads: %#v", workloads)
	}
	_, _, err = parseGraph("bad")
	if err == nil {
		t.Fatalf("expected error for invalid graph payload")
	}
}

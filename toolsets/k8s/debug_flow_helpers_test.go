package k8s

import "testing"

func TestParseGraphAndHelpers(t *testing.T) {
	_, _, err := parseGraph("bad")
	if err == nil {
		t.Fatalf("expected parseGraph error for invalid payload")
	}

	graphData := map[string]any{
		"nodes": []any{
			map[string]any{"id": "svc", "kind": "Service", "name": "api", "namespace": "default"},
			map[string]any{"id": "pod", "kind": "Pod", "name": "api-1", "namespace": "default"},
			map[string]any{"id": "deploy", "kind": "Deployment", "name": "api", "namespace": "default"},
		},
		"edges": []any{
			map[string]any{"from": "svc", "to": "pod", "relation": "targets"},
			map[string]any{"from": "pod", "to": "deploy", "relation": "owned-by"},
		},
		"warnings": []any{"partial discovery"},
	}
	view, warnings, err := parseGraph(graphData)
	if err != nil {
		t.Fatalf("parseGraph: %v", err)
	}
	if len(warnings) != 1 {
		t.Fatalf("expected warnings parsed")
	}
	if id := findNodeByName(view.nodes, "Deployment", "default", "api"); id == "" {
		t.Fatalf("expected findNodeByName match")
	}
	services := relatedByKind(view, []string{"svc"}, "Service")
	if len(services) != 0 {
		t.Fatalf("expected no service children from svc edge")
	}
	pods := relatedByKind(view, []string{"svc"}, "Pod")
	if len(pods) != 1 {
		t.Fatalf("expected pod related to service")
	}
	owners := relatedWorkloadsForPods(view, pods)
	if len(owners) != 1 || owners[0].Kind != "Deployment" {
		t.Fatalf("expected workload owner")
	}
	combined := appendUnique(pods, []flowNode{pods[0]})
	if len(combined) != 1 {
		t.Fatalf("expected appendUnique to dedupe")
	}
}

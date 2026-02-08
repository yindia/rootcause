package k8s

import "testing"

func TestBuildPermissionFlowBranches(t *testing.T) {
	toolset := New()
	added := []string{}
	addStep := func(node flowNode, tool string, args map[string]any, notes map[string]string) {
		added = append(added, tool)
	}

	graph := graphView{nodes: map[string]flowNode{
		"sa": {ID: "sa", Kind: "ServiceAccount", Name: "sa", Namespace: "default"},
	}}
	toolset.buildPermissionFlow(graph, "sa", "default", addStep)
	if len(added) != 1 || added[0] != "k8s.permission_debug" {
		t.Fatalf("expected serviceaccount permission step")
	}

	added = nil
	graph = graphView{nodes: map[string]flowNode{
		"pod": {ID: "pod", Kind: "Pod", Name: "pod-1", Namespace: "default"},
	}}
	toolset.buildPermissionFlow(graph, "pod", "default", addStep)
	if len(added) != 1 {
		t.Fatalf("expected pod permission step")
	}

	added = nil
	graph = graphView{nodes: map[string]flowNode{
		"svc": {ID: "svc", Kind: "Service", Name: "svc", Namespace: "default"},
	}}
	toolset.buildPermissionFlow(graph, "svc", "default", addStep)
	if len(added) != 1 {
		t.Fatalf("expected default serviceaccount step")
	}

	added = nil
	graph = graphView{nodes: map[string]flowNode{
		"cfg":  {ID: "cfg", Kind: "ConfigMap", Name: "cfg", Namespace: "default"},
		"pod1": {ID: "pod1", Kind: "Pod", Name: "pod-1", Namespace: "default"},
		"pod2": {ID: "pod2", Kind: "Pod", Name: "pod-2", Namespace: "default"},
	}}
	toolset.buildPermissionFlow(graph, "cfg", "default", addStep)
	if len(added) == 0 {
		t.Fatalf("expected pod steps for non-workload entry")
	}
}

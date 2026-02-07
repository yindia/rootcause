package karpenter

import (
	"context"
	"errors"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	k8sfake "k8s.io/client-go/kubernetes/fake"

	"rootcause/internal/config"
	"rootcause/internal/kube"
	"rootcause/internal/mcp"
	"rootcause/internal/policy"
	"rootcause/internal/redact"
	"rootcause/internal/render"
)

func TestKarpenterCRStatusBranches(t *testing.T) {
	toolset := newKarpenterToolset(t)
	clusterUser := policy.User{Role: policy.RoleCluster}
	if _, err := toolset.handleCRStatus(context.Background(), mcp.ToolRequest{
		User:      clusterUser,
		Arguments: map[string]any{"kind": "Provisioner", "name": "prov", "namespace": "default"},
	}); err != nil {
		t.Fatalf("handleCRStatus namespaced: %v", err)
	}
	if _, err := toolset.handleCRStatus(context.Background(), mcp.ToolRequest{
		User:      clusterUser,
		Arguments: map[string]any{"kind": "Provisioner", "name": "missing", "namespace": "default"},
	}); err != nil {
		t.Fatalf("handleCRStatus missing: %v", err)
	}
	if _, err := toolset.handleCRStatus(context.Background(), mcp.ToolRequest{
		User:      clusterUser,
		Arguments: map[string]any{"kind": "Provisioner"},
	}); err != nil {
		t.Fatalf("handleCRStatus list all: %v", err)
	}
	if _, err := toolset.handleCRStatus(context.Background(), mcp.ToolRequest{
		User:      clusterUser,
		Arguments: map[string]any{"kind": "NodePool"},
	}); err != nil {
		t.Fatalf("handleCRStatus cluster list: %v", err)
	}
	namespaceUser := policy.User{Role: policy.RoleNamespace, AllowedNamespaces: []string{"default"}}
	if _, err := toolset.handleCRStatus(context.Background(), mcp.ToolRequest{
		User:      namespaceUser,
		Arguments: map[string]any{"kind": "Provisioner"},
	}); err != nil {
		t.Fatalf("handleCRStatus namespace role: %v", err)
	}
	if _, err := toolset.handleCRStatus(context.Background(), mcp.ToolRequest{
		User:      clusterUser,
		Arguments: map[string]any{"kind": "Provisioner", "name": "prov"},
	}); err == nil {
		t.Fatalf("expected multiple namespace error")
	}
}

func TestKarpenterListResourceObjectsMultipleNamespaces(t *testing.T) {
	toolset := newKarpenterToolset(t)
	matchNamespaced := resourceMatch{
		GVR:        provisionerGVR(),
		Kind:       "Provisioner",
		Namespaced: true,
	}
	_, _, err := toolset.listResourceObjects(context.Background(), policy.User{Role: policy.RoleNamespace, AllowedNamespaces: []string{"default", "other"}}, matchNamespaced, "", "prov", "")
	if err == nil {
		t.Fatalf("expected multiple namespace error")
	}

	clusterUser := policy.User{Role: policy.RoleCluster}
	items, namespaces, err := toolset.listResourceObjects(context.Background(), clusterUser, matchNamespaced, "", "", "")
	if err != nil {
		t.Fatalf("listResourceObjects cluster list: %v", err)
	}
	if len(items) == 0 || len(namespaces) != 0 {
		t.Fatalf("expected cluster list results")
	}
	items, _, err = toolset.listResourceObjects(context.Background(), clusterUser, matchNamespaced, "", "prov", "")
	if err != nil || len(items) == 0 {
		t.Fatalf("expected cluster name filter results")
	}

	namespaceUser := policy.User{Role: policy.RoleNamespace, AllowedNamespaces: []string{"default"}}
	items, namespaces, err = toolset.listResourceObjects(context.Background(), namespaceUser, matchNamespaced, "", "", "")
	if err != nil {
		t.Fatalf("listResourceObjects namespace list: %v", err)
	}
	if len(items) == 0 || len(namespaces) != 1 {
		t.Fatalf("expected namespace list results")
	}
}

func TestAWSHelpers(t *testing.T) {
	role := awsNameFromARN("arn:aws:iam::123456789012:role/Path/RoleName", ":role/")
	if role != "RoleName" {
		t.Fatalf("unexpected role name: %s", role)
	}
	if awsNameFromARN("plain-name", ":role/") != "plain-name" {
		t.Fatalf("expected passthrough name")
	}
	if awsNameFromARN("arn:aws:iam::123456789012:instance-profile/Profile", ":instance-profile/") != "Profile" {
		t.Fatalf("expected instance profile name")
	}
	if out := stringMapFromAny(map[string]any{"key": "value", "": ""}); out["key"] != "value" {
		t.Fatalf("expected string map output")
	}
	if stringMapFromAny(map[string]string{"": "skip"}) != nil {
		t.Fatalf("expected nil for empty string map")
	}
	ids := selectorIDs(map[string]any{"ids": []any{"subnet-1", "subnet-2"}}, []string{"ids"})
	if len(ids) != 2 {
		t.Fatalf("expected selector ids")
	}
	obj := &unstructured.Unstructured{Object: map[string]any{
		"spec": map[string]any{
			"role":            "arn:aws:iam::123456789012:role/NodeRole",
			"instanceProfile": "arn:aws:iam::123456789012:instance-profile/NodeProfile",
			"subnetSelectorTerms": []any{
				map[string]any{"ids": []any{"subnet-1"}},
			},
			"securityGroupSelectorTerms": []any{
				map[string]any{"tags": map[string]any{"env": "dev"}},
			},
		},
	}}
	selectors := extractAWSNodeClassSelectors(obj)
	if selectors.isEmpty() {
		t.Fatalf("expected selectors")
	}
	if len(uniqueStrings([]string{"a", "a", ""})) != 1 {
		t.Fatalf("expected unique strings")
	}
}

func TestAddAWSNodeClassEvidenceWithRegistry(t *testing.T) {
	cfg := config.DefaultConfig()
	reg := mcp.NewRegistry(&cfg)
	addTool := func(name string) {
		_ = reg.Add(mcp.ToolSpec{
			Name:      name,
			ToolsetID: "aws",
			Safety:    mcp.SafetyReadOnly,
			Handler: func(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
				return mcp.ToolResult{Data: map[string]any{"ok": true}}, nil
			},
		})
	}
	addTool("aws.vpc.list_subnets")
	addTool("aws.vpc.list_security_groups")
	addTool("aws.iam.get_role")
	addTool("aws.iam.get_instance_profile")

	toolset := New()
	ctx := mcp.ToolsetContext{
		Config:   &cfg,
		Clients:  &kube.Clients{},
		Registry: reg,
		Policy:   policy.NewAuthorizer(),
		Renderer: render.NewRenderer(),
		Redactor: redact.New(),
	}
	ctx.Invoker = mcp.NewToolInvoker(reg, mcp.ToolContext(ctx))
	_ = toolset.Init(ctx)

	obj := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "karpenter.k8s.aws/v1beta1",
		"kind":       "EC2NodeClass",
		"metadata": map[string]any{
			"name": "default",
		},
		"spec": map[string]any{
			"role":            "arn:aws:iam::123456789012:role/NodeRole",
			"instanceProfile": "arn:aws:iam::123456789012:instance-profile/NodeProfile",
			"subnetSelectorTerms": []any{
				map[string]any{"ids": []any{"subnet-1"}},
			},
			"securityGroupSelectorTerms": []any{
				map[string]any{"ids": []any{"sg-1"}},
			},
		},
	}}
	match := resourceMatch{Group: "karpenter.k8s.aws", Kind: "EC2NodeClass"}
	analysis := render.NewAnalysis()
	toolset.addAWSNodeClassEvidence(context.Background(), mcp.ToolRequest{User: policy.User{Role: policy.RoleCluster}}, &analysis, match, obj)
	if len(analysis.Evidence) == 0 {
		t.Fatalf("expected aws evidence")
	}
}

func provisionerGVR() schema.GroupVersionResource {
	return schema.GroupVersionResource{Group: "karpenter.sh", Version: "v1beta1", Resource: "provisioners"}
}

func TestKarpenterInitAndVersion(t *testing.T) {
	toolset := New()
	if err := toolset.Init(mcp.ToolsetContext{}); err == nil {
		t.Fatalf("expected init error")
	}
	if toolset.Version() == "" {
		t.Fatalf("expected version string")
	}
}

func TestKarpenterStatusNotDetected(t *testing.T) {
	client := k8sfake.NewSimpleClientset(&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "default"}})
	cfg := config.DefaultConfig()
	toolset := New()
	_ = toolset.Init(mcp.ToolsetContext{
		Config:   &cfg,
		Clients:  &kube.Clients{Typed: client, Discovery: &fakeCachedDiscovery{groups: &metav1.APIGroupList{}}},
		Policy:   policy.NewAuthorizer(),
		Renderer: render.NewRenderer(),
		Redactor: redact.New(),
	})
	if _, err := toolset.handleStatus(context.Background(), mcp.ToolRequest{User: policy.User{Role: policy.RoleCluster}}); err != nil {
		t.Fatalf("handleStatus not detected: %v", err)
	}
}

func TestAWSNodeClassEvidenceToolMissing(t *testing.T) {
	cfg := config.DefaultConfig()
	reg := mcp.NewRegistry(&cfg)
	toolset := New()
	ctx := mcp.ToolsetContext{
		Config:   &cfg,
		Clients:  &kube.Clients{},
		Registry: reg,
		Policy:   policy.NewAuthorizer(),
		Renderer: render.NewRenderer(),
		Redactor: redact.New(),
	}
	_ = toolset.Init(ctx)
	obj := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "karpenter.k8s.aws/v1beta1",
		"kind":       "EC2NodeClass",
		"metadata":   map[string]any{"name": "default"},
		"spec": map[string]any{
			"role": "arn:aws:iam::123456789012:role/NodeRole",
			"subnetSelectorTerms": []any{
				map[string]any{"ids": []any{"subnet-1"}},
			},
		},
	}}
	match := resourceMatch{Group: "karpenter.k8s.aws", Kind: "EC2NodeClass"}
	analysis := render.NewAnalysis()
	toolset.addAWSNodeClassEvidence(context.Background(), mcp.ToolRequest{User: policy.User{Role: policy.RoleCluster}}, &analysis, match, obj)
	if len(analysis.Evidence) == 0 {
		t.Fatalf("expected aws evidence with missing tools")
	}
}

func TestKarpenterHelperBranches(t *testing.T) {
	match := resourceMatch{Group: "other.group", Kind: "Other"}
	if isAWSNodeClass(match, nil) {
		t.Fatalf("expected non-aws nodeclass")
	}
	if len(selectorIDs(map[string]any{"id": "subnet-1"}, []string{"id"})) == 0 {
		t.Fatalf("expected selector id")
	}
	if _, ok := nestedMap(nil, "spec"); ok {
		t.Fatalf("expected nil nested map")
	}
	if nestedString(nil, "spec") != "" {
		t.Fatalf("expected empty nested string")
	}
	if nestedInt(nil, "spec") != 0 {
		t.Fatalf("expected zero nested int")
	}
	if isConditionFalse(map[string]any{"type": 5}, []string{"Ready"}) {
		t.Fatalf("expected non-matching condition false")
	}
	if isConditionTrue(map[string]any{"type": "Ready", "status": "False"}, []string{"Ready"}) {
		t.Fatalf("expected non-matching condition true")
	}
}

func TestKarpenterNodePoolDebugNamespaceRole(t *testing.T) {
	toolset := newKarpenterToolset(t)
	user := policy.User{Role: policy.RoleNamespace, AllowedNamespaces: []string{"default"}}
	if _, err := toolset.handleNodePoolDebug(context.Background(), mcp.ToolRequest{User: user}); err == nil {
		t.Fatalf("expected namespace role error")
	}
}

func TestKarpenterResolveNodeClassRefAndUnstructured(t *testing.T) {
	toolset := New()
	index := &nodeClassIndex{
		byKind: map[string]map[string]struct{}{"ec2nodeclass": {"default": {}}},
		byName: map[string][]string{"default": {"EC2NodeClass"}},
	}
	out := toolset.resolveNodeClassRef(map[string]any{}, index)
	if out["found"] != false {
		t.Fatalf("expected missing name resolution")
	}
	out = toolset.resolveNodeClassRef(map[string]any{"name": "default"}, index)
	if out["found"] != true {
		t.Fatalf("expected name-only resolution")
	}
	if _, err := toUnstructured(&corev1.Pod{}); err != nil {
		t.Fatalf("toUnstructured: %v", err)
	}
}

func TestKarpenterNoNodeClassOrNodeClaim(t *testing.T) {
	client := k8sfake.NewSimpleClientset(&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "karpenter"}})
	discovery := &fakeCachedDiscovery{
		groups: &metav1.APIGroupList{Groups: []metav1.APIGroup{{Name: "karpenter.sh"}}},
	}
	cfg := config.DefaultConfig()
	toolset := New()
	_ = toolset.Init(mcp.ToolsetContext{
		Config:   &cfg,
		Clients:  &kube.Clients{Typed: client, Discovery: discovery},
		Policy:   policy.NewAuthorizer(),
		Renderer: render.NewRenderer(),
		Redactor: redact.New(),
	})
	if _, err := toolset.handleNodeClassDebug(context.Background(), mcp.ToolRequest{User: policy.User{Role: policy.RoleCluster}}); err != nil {
		t.Fatalf("handleNodeClassDebug no resources: %v", err)
	}
	if _, err := toolset.handleInterruptionDebug(context.Background(), mcp.ToolRequest{User: policy.User{Role: policy.RoleCluster}}); err != nil {
		t.Fatalf("handleInterruptionDebug no resources: %v", err)
	}
}

func TestKarpenterCRStatusSingleNamespaceAndMissingKind(t *testing.T) {
	toolset := newKarpenterToolset(t)
	namespaceUser := policy.User{Role: policy.RoleNamespace, AllowedNamespaces: []string{"default"}}
	if _, err := toolset.handleCRStatus(context.Background(), mcp.ToolRequest{
		User:      namespaceUser,
		Arguments: map[string]any{"kind": "Provisioner", "name": "prov"},
	}); err != nil {
		t.Fatalf("handleCRStatus single namespace: %v", err)
	}
	if _, err := toolset.handleCRStatus(context.Background(), mcp.ToolRequest{
		User:      namespaceUser,
		Arguments: map[string]any{},
	}); err == nil {
		t.Fatalf("expected missing kind error")
	}
}

func TestAddAWSNodeClassEvidenceToolError(t *testing.T) {
	cfg := config.DefaultConfig()
	reg := mcp.NewRegistry(&cfg)
	_ = reg.Add(mcp.ToolSpec{
		Name:      "aws.vpc.list_subnets",
		ToolsetID: "aws",
		Safety:    mcp.SafetyReadOnly,
		Handler: func(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
			return mcp.ToolResult{Data: map[string]any{"error": "fail"}}, errors.New("fail")
		},
	})
	toolset := New()
	ctx := mcp.ToolsetContext{
		Config:   &cfg,
		Clients:  &kube.Clients{},
		Registry: reg,
		Policy:   policy.NewAuthorizer(),
		Renderer: render.NewRenderer(),
		Redactor: redact.New(),
	}
	ctx.Invoker = mcp.NewToolInvoker(reg, mcp.ToolContext(ctx))
	_ = toolset.Init(ctx)

	obj := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "karpenter.k8s.aws/v1beta1",
		"kind":       "EC2NodeClass",
		"metadata":   map[string]any{"name": "default"},
		"spec": map[string]any{
			"subnetSelectorTerms": []any{
				map[string]any{"ids": []any{"subnet-1"}},
			},
		},
	}}
	match := resourceMatch{Group: "karpenter.k8s.aws", Kind: "EC2NodeClass"}
	analysis := render.NewAnalysis()
	toolset.addAWSNodeClassEvidence(context.Background(), mcp.ToolRequest{User: policy.User{Role: policy.RoleCluster}}, &analysis, match, obj)
	if len(analysis.Evidence) == 0 {
		t.Fatalf("expected evidence on error")
	}
}

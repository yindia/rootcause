package k8s

import (
	"context"
	"errors"
	"io"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/yaml"

	"rootcause/internal/kube"
	"rootcause/internal/mcp"
)

func (t *Toolset) handleSafeMutationPreflight(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	args := req.Arguments
	operation := strings.ToLower(strings.TrimSpace(toString(args["operation"])))
	if operation == "" {
		err := errors.New("operation is required")
		return errorResult(err), err
	}

	checks := make([]map[string]any, 0)
	addCheck := func(id, status, message, recommendation string, details map[string]any) {
		entry := map[string]any{"id": id, "status": status, "message": message}
		if recommendation != "" {
			entry["recommendation"] = recommendation
		}
		if len(details) > 0 {
			entry["details"] = details
		}
		checks = append(checks, entry)
	}

	namespace := strings.TrimSpace(toString(args["namespace"]))
	if namespace != "" {
		if err := t.ctx.Policy.CheckNamespace(req.User, namespace, true); err != nil {
			addCheck("namespace-policy", "fail", err.Error(), "Use an allowed namespace for this user role.", nil)
		} else {
			addCheck("namespace-policy", "pass", "Namespace is allowed by policy", "", map[string]any{"namespace": namespace})
		}
		if _, err := t.ctx.Clients.Typed.CoreV1().Namespaces().Get(ctx, namespace, metav1.GetOptions{}); err != nil {
			addCheck("namespace-exists", "fail", "Namespace does not exist", "Create the namespace before applying mutating changes.", map[string]any{"namespace": namespace, "error": err.Error()})
		} else {
			addCheck("namespace-exists", "pass", "Namespace exists", "", map[string]any{"namespace": namespace})
		}
	}

	apiVersion := toString(args["apiVersion"])
	kind := toString(args["kind"])
	resource := toString(args["resource"])
	name := toString(args["name"])

	switch operation {
	case "rollout":
		if name == "" || namespace == "" {
			addCheck("rollout-target", "fail", "name and namespace are required for rollout preflight", "Set deployment name and namespace.", nil)
			break
		}
		if err := t.ctx.Policy.CheckNamespace(req.User, namespace, true); err != nil {
			addCheck("namespace-policy", "fail", err.Error(), "Use an allowed namespace for this user role.", nil)
			break
		}
		if _, err := t.ctx.Clients.Typed.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{}); err != nil {
			addCheck("deployment-exists", "fail", "Deployment target not found", "Verify deployment name and namespace before rollout.", map[string]any{"namespace": namespace, "name": name, "error": err.Error()})
		} else {
			addCheck("deployment-exists", "pass", "Deployment target exists", "", map[string]any{"namespace": namespace, "name": name})
		}
	case "delete", "patch", "scale":
		if name == "" {
			addCheck("target-name", "fail", "name is required for target mutation preflight", "Provide the target object name.", nil)
			break
		}
		gvr, namespaced, err := kube.ResolveResource(t.ctx.Clients.Mapper, apiVersion, kind, resource)
		if err != nil {
			addCheck("resource-resolution", "fail", "Unable to resolve target resource", "Provide valid apiVersion/kind/resource.", map[string]any{"error": err.Error()})
			break
		}
		if err := t.ctx.Policy.CheckNamespace(req.User, namespace, namespaced); err != nil {
			addCheck("namespace-policy", "fail", err.Error(), "Use an allowed namespace and namespaced resource target.", nil)
			break
		}
		if namespaced && namespace == "" {
			addCheck("namespace-required", "fail", "Namespace required for namespaced target", "Set namespace for this operation.", map[string]any{"resource": gvr.Resource})
			break
		}
		var getErr error
		if namespaced {
			_, getErr = t.ctx.Clients.Dynamic.Resource(gvr).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
		} else {
			_, getErr = t.ctx.Clients.Dynamic.Resource(gvr).Get(ctx, name, metav1.GetOptions{})
		}
		if getErr != nil {
			addCheck("target-exists", "fail", "Target object not found", "Verify kind/resource/name before mutation.", map[string]any{"error": getErr.Error()})
		} else {
			addCheck("target-exists", "pass", "Target object exists", "", map[string]any{"resource": gvr.Resource, "name": name})
		}
		if operation == "scale" {
			replicas := toInt(args["replicas"], -1)
			if replicas < 0 {
				addCheck("scale-replicas", "fail", "replicas must be >= 0", "Set a non-negative replicas value.", nil)
			} else if replicas < 2 {
				addCheck("scale-ha", "warn", "Scaling below 2 replicas reduces availability", "Use >=2 replicas for HA workloads when possible.", map[string]any{"replicas": replicas})
			} else {
				addCheck("scale-ha", "pass", "Replica count supports basic HA", "", map[string]any{"replicas": replicas})
			}
		}
	case "apply", "create":
		manifest := toString(args["manifest"])
		if strings.TrimSpace(manifest) == "" {
			addCheck("manifest-required", "fail", "manifest is required", "Provide manifest YAML for preflight validation.", nil)
			break
		}
		decoder := yaml.NewYAMLOrJSONDecoder(strings.NewReader(manifest), 4096)
		count := 0
		for {
			var raw map[string]any
			if err := decoder.Decode(&raw); err != nil {
				if errors.Is(err, io.EOF) {
					break
				}
				addCheck("manifest-parse", "fail", "Failed to parse manifest", "Fix YAML/JSON syntax in manifest.", map[string]any{"error": err.Error()})
				break
			}
			if len(raw) == 0 {
				continue
			}
			count++
			obj := &unstructured.Unstructured{Object: raw}
			gvr, namespaced, err := kube.ResolveResource(t.ctx.Clients.Mapper, obj.GetAPIVersion(), obj.GetKind(), "")
			if err != nil {
				addCheck("manifest-resource", "fail", "Manifest contains unresolved resource kind", "Verify CRDs and apiVersion/kind values are installed.", map[string]any{"kind": obj.GetKind(), "apiVersion": obj.GetAPIVersion(), "error": err.Error()})
				continue
			}
			targetNamespace := obj.GetNamespace()
			if targetNamespace == "" {
				targetNamespace = namespace
			}
			if namespaced && targetNamespace == "" {
				addCheck("manifest-namespace", "fail", "Namespaced manifest object is missing namespace", "Set namespace in manifest or request.", map[string]any{"resource": gvr.Resource, "name": obj.GetName()})
				continue
			}
			if err := t.ctx.Policy.CheckNamespace(req.User, targetNamespace, namespaced); err != nil {
				addCheck("manifest-namespace-policy", "fail", "Manifest object violates namespace policy", "Use allowed namespace for all namespaced objects.", map[string]any{"namespace": targetNamespace, "resource": gvr.Resource, "name": obj.GetName()})
				continue
			}
			addCheck("manifest-object", "pass", "Manifest object resolves and passes policy checks", "", map[string]any{"resource": gvr.Resource, "name": obj.GetName(), "namespace": targetNamespace})
		}
		if count == 0 {
			addCheck("manifest-empty", "fail", "Manifest does not contain any Kubernetes objects", "Provide at least one valid Kubernetes object in manifest.", nil)
		}
	case "cleanup_pods", "node_management":
		addCheck("operation-scope", "warn", "Operation can evict/delete running workloads", "Run during maintenance windows and validate PDB/restart safety first.", map[string]any{"operation": operation})
	default:
		addCheck("operation-supported", "fail", "Unsupported operation for preflight", "Use one of: create, apply, patch, delete, scale, rollout, cleanup_pods, node_management.", map[string]any{"operation": operation})
	}

	failures := 0
	warnings := 0
	for _, check := range checks {
		status := toString(check["status"])
		if status == "fail" {
			failures++
		}
		if status == "warn" {
			warnings++
		}
	}
	safeToProceed := failures == 0
	risk := "low"
	if failures > 0 {
		risk = "high"
	} else if warnings > 0 {
		risk = "medium"
	}

	return mcp.ToolResult{Data: map[string]any{
		"operation":     operation,
		"safeToProceed": safeToProceed,
		"risk":          risk,
		"summary": map[string]any{
			"checks":   len(checks),
			"failures": failures,
			"warnings": warnings,
		},
		"checks": checks,
	}}, nil
}

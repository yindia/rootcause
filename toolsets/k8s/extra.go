package k8s

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/tools/clientcmd"

	"rootcause/internal/kube"
	"rootcause/internal/mcp"
	"rootcause/internal/render"
)

func (t *Toolset) handleCreate(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	args := req.Arguments
	if err := requireConfirm(args); err != nil {
		return errorResult(err), err
	}
	manifest := toString(args["manifest"])
	namespace := toString(args["namespace"])
	if manifest == "" {
		return errorResult(errors.New("manifest is required")), errors.New("manifest is required")
	}
	decoder := yaml.NewYAMLOrJSONDecoder(strings.NewReader(manifest), 4096)
	var created []map[string]any
	for {
		var raw map[string]any
		if err := decoder.Decode(&raw); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return errorResult(err), err
		}
		if len(raw) == 0 {
			continue
		}
		obj := &unstructured.Unstructured{Object: raw}
		gvr, namespaced, err := kube.ResolveResource(t.ctx.Clients.Mapper, obj.GetAPIVersion(), obj.GetKind(), "")
		if err != nil {
			return errorResult(err), err
		}
		objNamespace := obj.GetNamespace()
		if namespaced {
			if objNamespace == "" && namespace != "" {
				objNamespace = namespace
				obj.SetNamespace(namespace)
			}
			if objNamespace == "" {
				return errorResult(errors.New("namespace required in manifest or input")), errors.New("namespace required in manifest or input")
			}
			if namespace != "" && objNamespace != namespace {
				return errorResult(errors.New("manifest namespace does not match input")), errors.New("manifest namespace does not match input")
			}
		}
		if err := t.ctx.Policy.CheckNamespace(req.User, objNamespace, namespaced); err != nil {
			return errorResult(err), err
		}
		var resource *unstructured.Unstructured
		if namespaced {
			resource, err = t.ctx.Clients.Dynamic.Resource(gvr).Namespace(objNamespace).Create(ctx, obj, metav1.CreateOptions{})
		} else {
			resource, err = t.ctx.Clients.Dynamic.Resource(gvr).Create(ctx, obj, metav1.CreateOptions{})
		}
		if err != nil {
			return errorResult(err), err
		}
		created = append(created, map[string]any{
			"resource": t.ctx.Evidence.ResourceRef(gvr, objNamespace, obj.GetName()),
			"object":   t.redactUnstructured(resource),
		})
	}
	return mcp.ToolResult{Data: map[string]any{"created": created}, Metadata: mcp.ToolMetadata{Namespaces: sliceIf(namespace)}}, nil
}

func (t *Toolset) handleScale(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	args := req.Arguments
	if err := requireConfirm(args); err != nil {
		return errorResult(err), err
	}
	apiVersion := toString(args["apiVersion"])
	kind := toString(args["kind"])
	resource := toString(args["resource"])
	name := toString(args["name"])
	namespace := toString(args["namespace"])
	replicasVal, ok := args["replicas"].(float64)
	if name == "" || !ok {
		return errorResult(errors.New("name and replicas are required")), errors.New("name and replicas are required")
	}
	replicas := int64(replicasVal)
	gvr, namespaced, err := kube.ResolveResource(t.ctx.Clients.Mapper, apiVersion, kind, resource)
	if err != nil {
		return errorResult(err), err
	}
	if namespaced && namespace == "" {
		return errorResult(errors.New("namespace required for namespaced resource")), errors.New("namespace required for namespaced resource")
	}
	if err := t.ctx.Policy.CheckNamespace(req.User, namespace, namespaced); err != nil {
		return errorResult(err), err
	}
	patch := fmt.Sprintf(`{"spec":{"replicas":%d}}`, replicas)
	var obj *unstructured.Unstructured
	if namespaced {
		obj, err = t.ctx.Clients.Dynamic.Resource(gvr).Namespace(namespace).Patch(ctx, name, types.MergePatchType, []byte(patch), metav1.PatchOptions{})
	} else {
		obj, err = t.ctx.Clients.Dynamic.Resource(gvr).Patch(ctx, name, types.MergePatchType, []byte(patch), metav1.PatchOptions{})
	}
	if err != nil {
		return errorResult(err), err
	}
	return mcp.ToolResult{Data: t.redactUnstructured(obj), Metadata: mcp.ToolMetadata{Namespaces: sliceIf(namespace), Resources: []string{t.ctx.Evidence.ResourceRef(gvr, namespace, name)}}}, nil
}

func (t *Toolset) handleRollout(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	args := req.Arguments
	if err := requireConfirm(args); err != nil {
		return errorResult(err), err
	}
	action := strings.ToLower(toString(args["action"]))
	if action == "" {
		action = "status"
	}
	name := toString(args["name"])
	namespace := toString(args["namespace"])
	if name == "" || namespace == "" {
		return errorResult(errors.New("name and namespace are required")), errors.New("name and namespace are required")
	}
	if err := t.ctx.Policy.CheckNamespace(req.User, namespace, true); err != nil {
		return errorResult(err), err
	}
	if !isDeploymentTarget(toString(args["apiVersion"]), toString(args["kind"]), toString(args["resource"])) {
		return errorResult(errors.New("only Deployment rollouts are supported")), errors.New("only Deployment rollouts are supported")
	}

	switch action {
	case "status":
		deployment, err := t.ctx.Clients.Typed.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return errorResult(err), err
		}
		analysis := render.NewAnalysis()
		desired := int32(0)
		if deployment.Spec.Replicas != nil {
			desired = *deployment.Spec.Replicas
		}
		analysis.AddEvidence("deploymentStatus", map[string]any{
			"desiredReplicas":   desired,
			"updatedReplicas":   deployment.Status.UpdatedReplicas,
			"readyReplicas":     deployment.Status.ReadyReplicas,
			"availableReplicas": deployment.Status.AvailableReplicas,
			"conditions":        deployment.Status.Conditions,
		})
		analysis.AddResource(fmt.Sprintf("deployments/%s/%s", namespace, deployment.Name))
		if deployment.Status.ReadyReplicas < desired {
			analysis.AddCause("Deployment not ready", "Deployment has unavailable replicas", "high")
			analysis.AddNextCheck("Inspect pod status, events, and controller logs")
		} else {
			analysis.AddEvidence("status", "deployment is ready")
		}
		return mcp.ToolResult{Data: t.ctx.Renderer.Render(analysis), Metadata: mcp.ToolMetadata{Namespaces: []string{namespace}}}, nil
	case "restart":
		patch := fmt.Sprintf(`{"spec":{"template":{"metadata":{"annotations":{"kubectl.kubernetes.io/restartedAt":"%s"}}}}}`, time.Now().Format(time.RFC3339))
		obj, err := t.ctx.Clients.Typed.AppsV1().Deployments(namespace).Patch(ctx, name, types.MergePatchType, []byte(patch), metav1.PatchOptions{})
		if err != nil {
			return errorResult(err), err
		}
		return mcp.ToolResult{Data: map[string]any{"restarted": true, "deployment": obj.Name}, Metadata: mcp.ToolMetadata{Namespaces: []string{namespace}, Resources: []string{fmt.Sprintf("deployments/%s/%s", namespace, name)}}}, nil
	default:
		return errorResult(errors.New("unsupported rollout action")), errors.New("unsupported rollout action")
	}
}

func (t *Toolset) handleContext(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	_ = ctx
	action := strings.ToLower(toString(req.Arguments["action"]))
	if action == "" {
		action = "list"
	}
	if action == "use" {
		return errorResult(errors.New("use action not supported; restart server with --context")), errors.New("use action not supported; restart server with --context")
	}

	rules := clientcmd.NewDefaultClientConfigLoadingRules()
	if t.ctx.Config != nil && t.ctx.Config.Kubeconfig != "" {
		rules.ExplicitPath = t.ctx.Config.Kubeconfig
	}
	config, err := rules.Load()
	if err != nil {
		return errorResult(err), err
	}
	current := config.CurrentContext
	var contexts []map[string]any
	for name, ctx := range config.Contexts {
		contexts = append(contexts, map[string]any{
			"name":      name,
			"cluster":   ctx.Cluster,
			"user":      ctx.AuthInfo,
			"namespace": ctx.Namespace,
		})
	}

	if action == "current" {
		return mcp.ToolResult{Data: map[string]any{"current": current}}, nil
	}
	return mcp.ToolResult{Data: map[string]any{"current": current, "contexts": contexts}}, nil
}

func (t *Toolset) handleExplain(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	args := req.Arguments
	apiVersion := toString(args["apiVersion"])
	kind := toString(args["kind"])
	resource := toString(args["resource"])
	gvr, namespaced, err := kube.ResolveResource(t.ctx.Clients.Mapper, apiVersion, kind, resource)
	if err != nil {
		return errorResult(err), err
	}
	gv := gvr.GroupVersion().String()
	resources, err := t.ctx.Clients.Discovery.ServerResourcesForGroupVersion(gv)
	if err != nil {
		return errorResult(err), err
	}
	var found *metav1.APIResource
	for _, res := range resources.APIResources {
		if res.Name == gvr.Resource {
			copy := res
			found = &copy
			break
		}
	}
	data := map[string]any{
		"groupVersion": gv,
		"resource":     gvr.Resource,
		"namespaced":   namespaced,
	}
	if found != nil {
		data["kind"] = found.Kind
		data["verbs"] = found.Verbs
		data["shortNames"] = found.ShortNames
		data["categories"] = found.Categories
		data["singularName"] = found.SingularName
	}
	return mcp.ToolResult{Data: data}, nil
}

func (t *Toolset) handleGeneric(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	verb := strings.ToLower(toString(req.Arguments["verb"]))
	switch verb {
	case "get":
		return t.handleGet(ctx, req)
	case "list":
		return t.handleList(ctx, req)
	case "describe":
		return t.handleDescribe(ctx, req)
	case "create":
		return t.handleCreate(ctx, req)
	case "apply":
		return t.handleApply(ctx, req)
	case "patch":
		return t.handlePatch(ctx, req)
	case "delete":
		return t.handleDelete(ctx, req)
	case "logs":
		return t.handleLogs(ctx, req)
	case "events":
		return t.handleEvents(ctx, req)
	case "scale":
		return t.handleScale(ctx, req)
	case "rollout":
		return t.handleRollout(ctx, req)
	case "context":
		return t.handleContext(ctx, req)
	default:
		return errorResult(fmt.Errorf("unsupported verb: %s", verb)), fmt.Errorf("unsupported verb: %s", verb)
	}
}

func (t *Toolset) handlePing(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	_ = req
	version, err := t.ctx.Clients.Discovery.ServerVersion()
	if err != nil {
		return errorResult(err), err
	}
	return mcp.ToolResult{Data: map[string]any{"ok": true, "version": version}}, nil
}

func isDeploymentTarget(apiVersion, kind, resource string) bool {
	if kind != "" && strings.EqualFold(kind, "Deployment") {
		return true
	}
	if resource != "" && strings.Contains(strings.ToLower(resource), "deployment") {
		return true
	}
	if apiVersion == "" && kind == "" && resource == "" {
		return true
	}
	return false
}

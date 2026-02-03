package k8s

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/yaml"

	"rootcause/internal/kube"
	"rootcause/internal/mcp"
	"rootcause/internal/policy"
	"rootcause/internal/render"
)

func (t *Toolset) handleGet(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	args := req.Arguments
	apiVersion := toString(args["apiVersion"])
	kind := toString(args["kind"])
	resource := toString(args["resource"])
	name := toString(args["name"])
	namespace := toString(args["namespace"])
	if name == "" {
		return errorResult(errors.New("name is required")), errors.New("name is required")
	}
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
	var obj *unstructured.Unstructured
	if namespaced {
		obj, err = t.ctx.Clients.Dynamic.Resource(gvr).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
	} else {
		obj, err = t.ctx.Clients.Dynamic.Resource(gvr).Get(ctx, name, metav1.GetOptions{})
	}
	if err != nil {
		return errorResult(err), err
	}
	data := t.redactUnstructured(obj)
	return mcp.ToolResult{Data: data, Metadata: mcp.ToolMetadata{Namespaces: sliceIf(namespace), Resources: []string{t.ctx.Evidence.ResourceRef(gvr, namespace, name)}}}, nil
}

func (t *Toolset) handleList(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	args := req.Arguments
	resources := args["resources"]
	var resourceList []map[string]any
	if list, ok := resources.([]any); ok {
		for _, item := range list {
			if m, ok := item.(map[string]any); ok {
				resourceList = append(resourceList, m)
			}
		}
	}
	if len(resourceList) == 0 {
		return errorResult(errors.New("resources list required")), errors.New("resources list required")
	}
	namespace := toString(args["namespace"])
	labelSelector := toString(args["labelSelector"])
	fieldSelector := toString(args["fieldSelector"])

	results := make([]map[string]any, 0, len(resourceList))
	for _, res := range resourceList {
		apiVersion := toString(res["apiVersion"])
		kind := toString(res["kind"])
		resourceName := toString(res["resource"])
		gvr, namespaced, err := kube.ResolveResource(t.ctx.Clients.Mapper, apiVersion, kind, resourceName)
		if err != nil {
			return errorResult(err), err
		}
		items := []any{}
		if namespaced {
			if namespace != "" {
				if err := t.ctx.Policy.CheckNamespace(req.User, namespace, true); err != nil {
					return errorResult(err), err
				}
				list, err := t.ctx.Clients.Dynamic.Resource(gvr).Namespace(namespace).List(ctx, metav1.ListOptions{LabelSelector: labelSelector, FieldSelector: fieldSelector})
				if err != nil {
					return errorResult(err), err
				}
				for _, item := range list.Items {
					items = append(items, t.redactUnstructured(&item))
				}
			} else {
				namespaces, err := t.allowedNamespaces(ctx, req.User)
				if err != nil {
					return errorResult(err), err
				}
				for _, ns := range namespaces {
					list, err := t.ctx.Clients.Dynamic.Resource(gvr).Namespace(ns).List(ctx, metav1.ListOptions{LabelSelector: labelSelector, FieldSelector: fieldSelector})
					if err != nil {
						return errorResult(err), err
					}
					for _, item := range list.Items {
						items = append(items, t.redactUnstructured(&item))
					}
				}
			}
		} else {
			if err := t.ctx.Policy.CheckNamespace(req.User, "", false); err != nil {
				return errorResult(err), err
			}
			list, err := t.ctx.Clients.Dynamic.Resource(gvr).List(ctx, metav1.ListOptions{LabelSelector: labelSelector, FieldSelector: fieldSelector})
			if err != nil {
				return errorResult(err), err
			}
			for _, item := range list.Items {
				items = append(items, t.redactUnstructured(&item))
			}
		}
		results = append(results, map[string]any{
			"resource": gvr.Resource,
			"items":    items,
		})
	}
	return mcp.ToolResult{Data: map[string]any{"results": results}, Metadata: mcp.ToolMetadata{Namespaces: sliceIf(namespace)}}, nil
}

func (t *Toolset) handleDescribe(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	args := req.Arguments
	apiVersion := toString(args["apiVersion"])
	kind := toString(args["kind"])
	resource := toString(args["resource"])
	name := toString(args["name"])
	namespace := toString(args["namespace"])
	if name == "" {
		return errorResult(errors.New("name is required")), errors.New("name is required")
	}
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
	var obj *unstructured.Unstructured
	if namespaced {
		obj, err = t.ctx.Clients.Dynamic.Resource(gvr).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
	} else {
		obj, err = t.ctx.Clients.Dynamic.Resource(gvr).Get(ctx, name, metav1.GetOptions{})
	}
	if err != nil {
		return errorResult(err), err
	}
	analysis := t.ctx.Renderer.Render(render.DescribeAnalysis(ctx, t.ctx.Evidence, t.ctx.Redactor, gvr, obj))
	return mcp.ToolResult{Data: analysis, Metadata: mcp.ToolMetadata{Namespaces: sliceIf(namespace), Resources: []string{t.ctx.Evidence.ResourceRef(gvr, namespace, name)}}}, nil
}

func (t *Toolset) handleDelete(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	args := req.Arguments
	if err := requireConfirm(args); err != nil {
		return errorResult(err), err
	}
	apiVersion := toString(args["apiVersion"])
	kind := toString(args["kind"])
	resource := toString(args["resource"])
	name := toString(args["name"])
	namespace := toString(args["namespace"])
	if name == "" {
		return errorResult(errors.New("name is required")), errors.New("name is required")
	}
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
	if namespaced {
		err = t.ctx.Clients.Dynamic.Resource(gvr).Namespace(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	} else {
		err = t.ctx.Clients.Dynamic.Resource(gvr).Delete(ctx, name, metav1.DeleteOptions{})
	}
	if err != nil {
		return errorResult(err), err
	}
	return mcp.ToolResult{Data: map[string]any{"deleted": true}, Metadata: mcp.ToolMetadata{Namespaces: sliceIf(namespace), Resources: []string{t.ctx.Evidence.ResourceRef(gvr, namespace, name)}}}, nil
}

func (t *Toolset) handleApply(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	args := req.Arguments
	if err := requireConfirm(args); err != nil {
		return errorResult(err), err
	}
	manifest := toString(args["manifest"])
	namespace := toString(args["namespace"])
	fieldManager := toString(args["fieldManager"])
	if fieldManager == "" {
		fieldManager = "rootcause"
	}
	force := false
	if val, ok := args["force"].(bool); ok {
		force = val
	}
	if manifest == "" {
		return errorResult(errors.New("manifest is required")), errors.New("manifest is required")
	}
	decoder := yaml.NewYAMLOrJSONDecoder(strings.NewReader(manifest), 4096)
	var applied []map[string]any
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
		apiVersion := obj.GetAPIVersion()
		kind := obj.GetKind()
		gvr, namespaced, err := kube.ResolveResource(t.ctx.Clients.Mapper, apiVersion, kind, "")
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
		data, err := obj.MarshalJSON()
		if err != nil {
			return errorResult(err), err
		}
		var resource *unstructured.Unstructured
		if namespaced {
			resource, err = t.ctx.Clients.Dynamic.Resource(gvr).Namespace(objNamespace).Patch(ctx, obj.GetName(), types.ApplyPatchType, data, metav1.PatchOptions{FieldManager: fieldManager, Force: &force})
		} else {
			resource, err = t.ctx.Clients.Dynamic.Resource(gvr).Patch(ctx, obj.GetName(), types.ApplyPatchType, data, metav1.PatchOptions{FieldManager: fieldManager, Force: &force})
		}
		if err != nil {
			return errorResult(err), err
		}
		applied = append(applied, map[string]any{"resource": t.ctx.Evidence.ResourceRef(gvr, objNamespace, obj.GetName()), "object": t.redactUnstructured(resource)})
	}
	return mcp.ToolResult{Data: map[string]any{"applied": applied}, Metadata: mcp.ToolMetadata{Namespaces: sliceIf(namespace)}}, nil
}

func (t *Toolset) handlePatch(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	args := req.Arguments
	if err := requireConfirm(args); err != nil {
		return errorResult(err), err
	}
	apiVersion := toString(args["apiVersion"])
	kind := toString(args["kind"])
	resource := toString(args["resource"])
	name := toString(args["name"])
	namespace := toString(args["namespace"])
	patch := toString(args["patch"])
	patchType := strings.ToLower(toString(args["patchType"]))
	if patchType == "" {
		patchType = "merge"
	}
	if name == "" || patch == "" {
		return errorResult(errors.New("name and patch are required")), errors.New("name and patch are required")
	}
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
	var patchTypeVal types.PatchType
	if patchType == "json" {
		patchTypeVal = types.JSONPatchType
	} else if patchType == "strategic" {
		patchTypeVal = types.StrategicMergePatchType
	} else {
		patchTypeVal = types.MergePatchType
	}
	var obj *unstructured.Unstructured
	if namespaced {
		obj, err = t.ctx.Clients.Dynamic.Resource(gvr).Namespace(namespace).Patch(ctx, name, patchTypeVal, []byte(patch), metav1.PatchOptions{})
	} else {
		obj, err = t.ctx.Clients.Dynamic.Resource(gvr).Patch(ctx, name, patchTypeVal, []byte(patch), metav1.PatchOptions{})
	}
	if err != nil {
		return errorResult(err), err
	}
	return mcp.ToolResult{Data: t.redactUnstructured(obj), Metadata: mcp.ToolMetadata{Namespaces: sliceIf(namespace), Resources: []string{t.ctx.Evidence.ResourceRef(gvr, namespace, name)}}}, nil
}

func (t *Toolset) handleLogs(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	args := req.Arguments
	namespace := toString(args["namespace"])
	pod := toString(args["pod"])
	container := toString(args["container"])
	if namespace == "" || pod == "" {
		return errorResult(errors.New("namespace and pod are required")), errors.New("namespace and pod are required")
	}
	if err := t.ctx.Policy.CheckNamespace(req.User, namespace, true); err != nil {
		return errorResult(err), err
	}
	options := &corev1.PodLogOptions{}
	if container != "" {
		options.Container = container
	}
	if val, ok := args["tailLines"].(float64); ok {
		lines := int64(val)
		options.TailLines = &lines
	}
	if val, ok := args["sinceSeconds"].(float64); ok {
		sec := int64(val)
		options.SinceSeconds = &sec
	}
	stream, err := t.ctx.Clients.Typed.CoreV1().Pods(namespace).GetLogs(pod, options).Stream(ctx)
	if err != nil {
		return errorResult(err), err
	}
	defer stream.Close()
	buf := new(bytes.Buffer)
	if _, err := io.Copy(buf, stream); err != nil {
		return errorResult(err), err
	}
	output := t.ctx.Redactor.RedactString(buf.String())
	return mcp.ToolResult{Data: map[string]any{"logs": output}, Metadata: mcp.ToolMetadata{Namespaces: []string{namespace}, Resources: []string{fmt.Sprintf("pods/%s/%s", namespace, pod)}}}, nil
}

func (t *Toolset) handleEvents(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	args := req.Arguments
	namespace := toString(args["namespace"])
	fieldSelector := ""
	if name := toString(args["involvedObjectName"]); name != "" {
		fieldSelector = fmt.Sprintf("involvedObject.name=%s", name)
	}
	if kind := toString(args["involvedObjectKind"]); kind != "" {
		if fieldSelector != "" {
			fieldSelector += ","
		}
		fieldSelector += fmt.Sprintf("involvedObject.kind=%s", kind)
	}
	if uid := toString(args["involvedObjectUID"]); uid != "" {
		if fieldSelector != "" {
			fieldSelector += ","
		}
		fieldSelector += fmt.Sprintf("involvedObject.uid=%s", uid)
	}

	results := []any{}
	if namespace != "" {
		if err := t.ctx.Policy.CheckNamespace(req.User, namespace, true); err != nil {
			return errorResult(err), err
		}
		list, err := t.ctx.Clients.Typed.CoreV1().Events(namespace).List(ctx, metav1.ListOptions{FieldSelector: fieldSelector})
		if err != nil {
			return errorResult(err), err
		}
		for _, event := range list.Items {
			results = append(results, event)
		}
	} else {
		namespaces, err := t.allowedNamespaces(ctx, req.User)
		if err != nil {
			return errorResult(err), err
		}
		for _, ns := range namespaces {
			list, err := t.ctx.Clients.Typed.CoreV1().Events(ns).List(ctx, metav1.ListOptions{FieldSelector: fieldSelector})
			if err != nil {
				return errorResult(err), err
			}
			for _, event := range list.Items {
				results = append(results, event)
			}
		}
	}
	return mcp.ToolResult{Data: map[string]any{"events": results}, Metadata: mcp.ToolMetadata{Namespaces: sliceIf(namespace)}}, nil
}

func (t *Toolset) handleAPIResources(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	if err := t.ctx.Policy.CheckNamespace(req.User, "", false); err != nil {
		return errorResult(err), err
	}
	query := strings.ToLower(toString(req.Arguments["query"]))
	var limit int
	if val, ok := req.Arguments["limit"].(float64); ok {
		limit = int(val)
	}
	groups, err := t.ctx.Clients.Discovery.ServerPreferredResources()
	if err != nil {
		return errorResult(err), err
	}
	var results []map[string]any
	var matched int
	for _, group := range groups {
		var resources []metav1.APIResource
		entry := map[string]any{
			"groupVersion": group.GroupVersion,
		}
		for _, resource := range group.APIResources {
			if query != "" && !apiResourceMatches(query, group.GroupVersion, resource) {
				continue
			}
			resources = append(resources, resource)
			matched++
			if limit > 0 && matched >= limit {
				break
			}
		}
		if len(resources) == 0 {
			continue
		}
		entry["resources"] = resources
		results = append(results, entry)
		if limit > 0 && matched >= limit {
			break
		}
	}
	return mcp.ToolResult{Data: map[string]any{"apiResources": results, "matched": matched}}, nil
}

func apiResourceMatches(query, groupVersion string, resource metav1.APIResource) bool {
	if query == "" {
		return true
	}
	if strings.Contains(strings.ToLower(groupVersion), query) {
		return true
	}
	if strings.Contains(strings.ToLower(resource.Name), query) {
		return true
	}
	if strings.Contains(strings.ToLower(resource.Kind), query) {
		return true
	}
	if strings.Contains(strings.ToLower(resource.SingularName), query) {
		return true
	}
	for _, short := range resource.ShortNames {
		if strings.Contains(strings.ToLower(short), query) {
			return true
		}
	}
	for _, category := range resource.Categories {
		if strings.Contains(strings.ToLower(category), query) {
			return true
		}
	}
	return false
}

func (t *Toolset) handleExecReadonly(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	args := req.Arguments
	namespace := toString(args["namespace"])
	pod := toString(args["pod"])
	container := toString(args["container"])
	command := toStringSlice(args["command"])
	if namespace == "" || pod == "" || len(command) == 0 {
		return errorResult(errors.New("namespace, pod, and command are required")), errors.New("namespace, pod, and command are required")
	}
	if err := t.ctx.Policy.CheckNamespace(req.User, namespace, true); err != nil {
		return errorResult(err), err
	}
	if !t.commandAllowed(command) {
		return errorResult(errors.New("command not allowed")), errors.New("command not allowed")
	}
	output, errOutput, err := t.execCommand(ctx, namespace, pod, container, command)
	if err != nil {
		return errorResult(err), err
	}
	return mcp.ToolResult{Data: map[string]any{"stdout": output, "stderr": errOutput}, Metadata: mcp.ToolMetadata{Namespaces: []string{namespace}, Resources: []string{fmt.Sprintf("pods/%s/%s", namespace, pod)}}}, nil
}

func (t *Toolset) allowedNamespaces(ctx context.Context, user policy.User) ([]string, error) {
	if user.Role == policy.RoleCluster {
		list, err := t.ctx.Clients.Typed.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, err
		}
		var namespaces []string
		for _, item := range list.Items {
			namespaces = append(namespaces, item.Name)
		}
		return namespaces, nil
	}
	return append([]string{}, user.AllowedNamespaces...), nil
}

func (t *Toolset) commandAllowed(command []string) bool {
	if len(command) == 0 {
		return false
	}
	base := strings.ToLower(command[0])
	if strings.Contains(base, "sh") || strings.Contains(base, "bash") || strings.Contains(base, "zsh") {
		return false
	}
	for _, allowed := range t.ctx.Config.Exec.AllowedCommands {
		if strings.ToLower(allowed) == base {
			return true
		}
	}
	return false
}

func sliceIf(value string) []string {
	if value == "" {
		return nil
	}
	return []string{value}
}

func (t *Toolset) redactUnstructured(obj *unstructured.Unstructured) map[string]any {
	if obj == nil {
		return map[string]any{}
	}
	data := obj.UnstructuredContent()
	if obj.GetKind() == "Secret" || strings.Contains(obj.GetKind(), "Secret") {
		if dataObj, ok := data["data"].(map[string]any); ok {
			for k := range dataObj {
				dataObj[k] = "[REDACTED]"
			}
		}
		if _, ok := data["stringData"]; ok {
			data["stringData"] = "[REDACTED]"
		}
	}
	return t.ctx.Redactor.RedactMap(data)
}

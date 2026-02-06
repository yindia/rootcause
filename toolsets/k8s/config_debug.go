package k8s

import (
	"context"
	"errors"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"rootcause/internal/mcp"
	"rootcause/internal/render"
)

type configIssue struct {
	Kind         string   `json:"kind"`
	Name         string   `json:"name"`
	Source       string   `json:"source"`
	Optional     bool     `json:"optional"`
	MissingKeys  []string `json:"missingKeys,omitempty"`
	Missing      bool     `json:"missing"`
	Namespace    string   `json:"namespace,omitempty"`
	Container    string   `json:"container,omitempty"`
	ReferenceKey string   `json:"referenceKey,omitempty"`
}

func (t *Toolset) handleConfigDebug(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	namespace := toString(req.Arguments["namespace"])
	podName := toString(req.Arguments["pod"])
	kind := strings.ToLower(toString(req.Arguments["kind"]))
	name := toString(req.Arguments["name"])
	requiredKeys := toStringSlice(req.Arguments["requiredKeys"])
	if namespace == "" {
		return errorResult(errors.New("namespace is required")), errors.New("namespace is required")
	}
	if err := t.ctx.Policy.CheckNamespace(req.User, namespace, true); err != nil {
		return errorResult(err), err
	}
	if podName == "" && name == "" {
		return errorResult(errors.New("pod or name is required")), errors.New("pod or name is required")
	}

	analysis := render.NewAnalysis()
	var issues []configIssue

	if podName != "" {
		pod, err := t.ctx.Clients.Typed.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
		if err != nil {
			return errorResult(err), err
		}
		analysis.AddResource(fmt.Sprintf("pods/%s/%s", namespace, pod.Name))
		issues = append(issues, t.inspectPodConfigRefs(ctx, namespace, pod)...)
	}

	if name != "" {
		if kind == "" {
			kind = "configmap"
		}
		switch kind {
		case "configmap":
			issue := t.checkConfigMapKeys(ctx, namespace, name, requiredKeys, "direct", false, "")
			issues = append(issues, issue)
		case "secret":
			issue := t.checkSecretKeys(ctx, namespace, name, requiredKeys, "direct", false, "")
			issues = append(issues, issue)
		default:
			return errorResult(fmt.Errorf("unsupported kind: %s", kind)), fmt.Errorf("unsupported kind: %s", kind)
		}
	}

	if len(issues) == 0 {
		analysis.AddEvidence("status", "no config references found")
	} else {
		analysis.AddEvidence("issues", issues)
		for _, issue := range issues {
			if issue.Missing || len(issue.MissingKeys) > 0 {
				if issue.Optional {
					continue
				}
				msg := issue.Name
				if issue.Namespace != "" {
					msg = fmt.Sprintf("%s/%s", issue.Namespace, issue.Name)
				}
				if issue.Missing {
					analysis.AddCause("Config missing", fmt.Sprintf("%s %s missing", strings.Title(issue.Kind), msg), "high")
				} else {
					analysis.AddCause("Missing keys", fmt.Sprintf("%s %s missing keys: %s", strings.Title(issue.Kind), msg, strings.Join(issue.MissingKeys, ", ")), "medium")
				}
			}
		}
	}

	analysis.AddNextCheck("Ensure ConfigMap/Secret keys match pod references and redeploy pods")
	return mcp.ToolResult{Data: t.ctx.Renderer.Render(analysis), Metadata: mcp.ToolMetadata{Namespaces: []string{namespace}}}, nil
}

func (t *Toolset) inspectPodConfigRefs(ctx context.Context, namespace string, pod *corev1.Pod) []configIssue {
	var issues []configIssue
	if pod == nil {
		return issues
	}
	containers := append([]corev1.Container{}, pod.Spec.InitContainers...)
	containers = append(containers, pod.Spec.Containers...)
	for _, container := range containers {
		cName := container.Name
		for _, envFrom := range container.EnvFrom {
			if envFrom.ConfigMapRef != nil {
				issues = append(issues, t.checkConfigMapKeys(ctx, namespace, envFrom.ConfigMapRef.Name, nil, "envFrom", optionalRef(envFrom.ConfigMapRef.Optional), cName))
			}
			if envFrom.SecretRef != nil {
				issues = append(issues, t.checkSecretKeys(ctx, namespace, envFrom.SecretRef.Name, nil, "envFrom", optionalRef(envFrom.SecretRef.Optional), cName))
			}
		}
		for _, envVar := range container.Env {
			if envVar.ValueFrom == nil {
				continue
			}
			if envVar.ValueFrom.ConfigMapKeyRef != nil {
				ref := envVar.ValueFrom.ConfigMapKeyRef
				issues = append(issues, t.checkConfigMapKeys(ctx, namespace, ref.Name, []string{ref.Key}, "env", optionalRef(ref.Optional), cName))
			}
			if envVar.ValueFrom.SecretKeyRef != nil {
				ref := envVar.ValueFrom.SecretKeyRef
				issues = append(issues, t.checkSecretKeys(ctx, namespace, ref.Name, []string{ref.Key}, "env", optionalRef(ref.Optional), cName))
			}
		}
	}

	for _, volume := range pod.Spec.Volumes {
		if volume.ConfigMap != nil {
			keys := volumeItems(volume.ConfigMap.Items)
			issues = append(issues, t.checkConfigMapKeys(ctx, namespace, volume.ConfigMap.Name, keys, "volume", optionalRef(volume.ConfigMap.Optional), ""))
		}
		if volume.Secret != nil {
			keys := volumeItems(volume.Secret.Items)
			issues = append(issues, t.checkSecretKeys(ctx, namespace, volume.Secret.SecretName, keys, "volume", optionalRef(volume.Secret.Optional), ""))
		}
		if volume.Projected != nil {
			for _, source := range volume.Projected.Sources {
				if source.ConfigMap != nil {
					keys := volumeItems(source.ConfigMap.Items)
					issues = append(issues, t.checkConfigMapKeys(ctx, namespace, source.ConfigMap.Name, keys, "projected", optionalRef(source.ConfigMap.Optional), ""))
				}
				if source.Secret != nil {
					keys := volumeItems(source.Secret.Items)
					issues = append(issues, t.checkSecretKeys(ctx, namespace, source.Secret.Name, keys, "projected", optionalRef(source.Secret.Optional), ""))
				}
			}
		}
	}
	return issues
}

func (t *Toolset) checkConfigMapKeys(ctx context.Context, namespace, name string, required []string, source string, optional bool, container string) configIssue {
	issue := configIssue{
		Kind:      "configmap",
		Name:      name,
		Source:    source,
		Optional:  optional,
		Namespace: namespace,
		Container: container,
	}
	if name == "" {
		issue.Missing = true
		return issue
	}
	cm, err := t.ctx.Clients.Typed.CoreV1().ConfigMaps(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			issue.Missing = true
			return issue
		}
		issue.Missing = true
		return issue
	}
	issue.MissingKeys = missingKeys(required, cm.Data)
	return issue
}

func (t *Toolset) checkSecretKeys(ctx context.Context, namespace, name string, required []string, source string, optional bool, container string) configIssue {
	issue := configIssue{
		Kind:      "secret",
		Name:      name,
		Source:    source,
		Optional:  optional,
		Namespace: namespace,
		Container: container,
	}
	if name == "" {
		issue.Missing = true
		return issue
	}
	secret, err := t.ctx.Clients.Typed.CoreV1().Secrets(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			issue.Missing = true
			return issue
		}
		issue.Missing = true
		return issue
	}
	keys := mapKeys(secret.Data)
	issue.MissingKeys = missingKeys(required, keys)
	return issue
}

func missingKeys(required []string, present map[string]string) []string {
	if len(required) == 0 {
		return nil
	}
	var missing []string
	for _, key := range required {
		if _, ok := present[key]; !ok {
			missing = append(missing, key)
		}
	}
	return missing
}

func mapKeys(data map[string][]byte) map[string]string {
	out := map[string]string{}
	for k := range data {
		out[k] = ""
	}
	return out
}

func volumeItems(items []corev1.KeyToPath) []string {
	if len(items) == 0 {
		return nil
	}
	keys := make([]string, 0, len(items))
	for _, item := range items {
		if item.Key != "" {
			keys = append(keys, item.Key)
		}
	}
	return keys
}

func optionalRef(optional *bool) bool {
	if optional == nil {
		return false
	}
	return *optional
}

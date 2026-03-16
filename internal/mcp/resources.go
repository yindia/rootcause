package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/client-go/util/homedir"
	"sigs.k8s.io/yaml"

	"rootcause/internal/policy"
)

var staticResources = []*sdkmcp.Resource{
	{Name: "Kubeconfig Contexts", URI: "kubeconfig://contexts", Description: "List all available kubeconfig contexts", MIMEType: "application/json"},
	{Name: "Kubeconfig Current Context", URI: "kubeconfig://current-context", Description: "Get current kubeconfig context", MIMEType: "application/json"},
	{Name: "Current Namespace", URI: "namespace://current", Description: "Get current namespace", MIMEType: "application/json"},
	{Name: "Namespace List", URI: "namespace://list", Description: "List namespaces", MIMEType: "application/json"},
	{Name: "Cluster Info", URI: "cluster://info", Description: "Cluster connection info", MIMEType: "application/json"},
	{Name: "Cluster Nodes", URI: "cluster://nodes", Description: "Cluster nodes", MIMEType: "application/json"},
	{Name: "Cluster Version", URI: "cluster://version", Description: "Kubernetes server version", MIMEType: "application/json"},
	{Name: "Cluster API Resources", URI: "cluster://api-resources", Description: "Preferred API resources", MIMEType: "application/json"},
}

var manifestTemplates = []*sdkmcp.ResourceTemplate{
	{Name: "Deployment Manifest", URITemplate: "manifest://deployments/{namespace}/{name}", Description: "Get deployment manifest", MIMEType: "application/yaml"},
	{Name: "Service Manifest", URITemplate: "manifest://services/{namespace}/{name}", Description: "Get service manifest", MIMEType: "application/yaml"},
	{Name: "Pod Manifest", URITemplate: "manifest://pods/{namespace}/{name}", Description: "Get pod manifest", MIMEType: "application/yaml"},
	{Name: "ConfigMap Manifest", URITemplate: "manifest://configmaps/{namespace}/{name}", Description: "Get configmap manifest", MIMEType: "application/yaml"},
	{Name: "Secret Manifest", URITemplate: "manifest://secrets/{namespace}/{name}", Description: "Get redacted secret manifest", MIMEType: "application/yaml"},
	{Name: "Ingress Manifest", URITemplate: "manifest://ingresses/{namespace}/{name}", Description: "Get ingress manifest", MIMEType: "application/yaml"},
}

func RegisterSDKResources(server *sdkmcp.Server, ctx ToolContext) ([]string, []string, error) {
	if server == nil {
		return nil, nil, errors.New("server is required")
	}
	handler := resourceHandler(ctx)
	resourceURIs := make([]string, 0, len(staticResources))
	templates := make([]string, 0, len(manifestTemplates))
	for _, r := range staticResources {
		server.AddResource(r, handler)
		resourceURIs = append(resourceURIs, r.URI)
	}
	for _, t := range manifestTemplates {
		server.AddResourceTemplate(t, handler)
		templates = append(templates, t.URITemplate)
	}
	return resourceURIs, templates, nil
}

func resourceHandler(ctx ToolContext) sdkmcp.ResourceHandler {
	return func(callCtx context.Context, req *sdkmcp.ReadResourceRequest) (*sdkmcp.ReadResourceResult, error) {
		if req == nil || req.Params == nil {
			return nil, errors.New("invalid resource request")
		}
		apiKey := apiKeyFromMeta(req.Params.Meta)
		if apiKey == "" && req.Extra != nil && req.Extra.Header != nil {
			apiKey = strings.TrimSpace(req.Extra.Header.Get("X-Api-Key"))
			if apiKey == "" {
				authHeader := strings.TrimSpace(req.Extra.Header.Get("Authorization"))
				if strings.HasPrefix(strings.ToLower(authHeader), "bearer ") {
					apiKey = strings.TrimSpace(authHeader[len("bearer "):])
				}
			}
		}
		user, err := ctx.Policy.Authenticate(apiKey)
		if err != nil {
			return nil, err
		}
		uri := req.Params.URI
		parsed, err := url.Parse(uri)
		if err != nil {
			return nil, err
		}

		switch parsed.Scheme {
		case "kubeconfig":
			if err := ctx.Policy.CheckNamespace(user, "", false); err != nil {
				return nil, err
			}
			data, err := readKubeconfigResource(ctx, parsed.Host)
			if err != nil {
				return nil, err
			}
			return jsonResourceResult(uri, data)
		case "namespace":
			data, err := readNamespaceResource(callCtx, ctx, user, parsed.Host)
			if err != nil {
				return nil, err
			}
			return jsonResourceResult(uri, data)
		case "cluster":
			if err := ctx.Policy.CheckNamespace(user, "", false); err != nil {
				return nil, err
			}
			data, err := readClusterResource(callCtx, ctx, parsed.Host)
			if err != nil {
				return nil, err
			}
			return jsonResourceResult(uri, data)
		case "manifest":
			text, err := readManifestResource(callCtx, ctx, user, parsed)
			if err != nil {
				return nil, err
			}
			return &sdkmcp.ReadResourceResult{Contents: []*sdkmcp.ResourceContents{{URI: uri, MIMEType: "application/yaml", Text: text}}}, nil
		default:
			return nil, sdkmcp.ResourceNotFoundError(uri)
		}
	}
}

func jsonResourceResult(uri string, data any) (*sdkmcp.ReadResourceResult, error) {
	b, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	return &sdkmcp.ReadResourceResult{Contents: []*sdkmcp.ResourceContents{{URI: uri, MIMEType: "application/json", Text: string(b)}}}, nil
}

func readKubeconfigResource(ctx ToolContext, resource string) (any, error) {
	raw, err := loadRawKubeconfig(ctx)
	if err != nil {
		return nil, err
	}
	switch resource {
	case "contexts":
		names := make([]string, 0, len(raw.Contexts))
		for name := range raw.Contexts {
			names = append(names, name)
		}
		sort.Strings(names)
		return map[string]any{"contexts": names, "currentContext": raw.CurrentContext}, nil
	case "current-context":
		return map[string]any{"currentContext": raw.CurrentContext}, nil
	default:
		return nil, sdkmcp.ResourceNotFoundError("kubeconfig://" + resource)
	}
}

func readNamespaceResource(callCtx context.Context, ctx ToolContext, user policy.User, resource string) (any, error) {
	switch resource {
	case "current":
		raw, err := loadRawKubeconfig(ctx)
		if err != nil {
			return nil, err
		}
		name := raw.CurrentContext
		if ctx.Config != nil && strings.TrimSpace(ctx.Config.Context) != "" {
			name = strings.TrimSpace(ctx.Config.Context)
		}
		ns := "default"
		if c, ok := raw.Contexts[name]; ok && c != nil && strings.TrimSpace(c.Namespace) != "" {
			ns = c.Namespace
		}
		if err := ctx.Policy.CheckNamespace(user, ns, true); err != nil {
			return nil, err
		}
		return map[string]any{"namespace": ns, "context": name}, nil
	case "list":
		list, err := ctx.Clients.Typed.CoreV1().Namespaces().List(callCtx, metav1.ListOptions{})
		if err != nil {
			return nil, err
		}
		names := make([]string, 0, len(list.Items))
		for _, item := range list.Items {
			names = append(names, item.Name)
		}
		names = ctx.Policy.FilterNamespaces(user, names)
		sort.Strings(names)
		return map[string]any{"namespaces": names}, nil
	default:
		return nil, sdkmcp.ResourceNotFoundError("namespace://" + resource)
	}
}

func readClusterResource(callCtx context.Context, ctx ToolContext, resource string) (any, error) {
	switch resource {
	case "info":
		raw, _ := loadRawKubeconfig(ctx)
		currentContext := ""
		if raw != nil {
			currentContext = raw.CurrentContext
		}
		if ctx.Config != nil && strings.TrimSpace(ctx.Config.Context) != "" {
			currentContext = strings.TrimSpace(ctx.Config.Context)
		}
		return map[string]any{"host": ctx.Clients.RestConfig.Host, "context": currentContext}, nil
	case "nodes":
		list, err := ctx.Clients.Typed.CoreV1().Nodes().List(callCtx, metav1.ListOptions{})
		if err != nil {
			return nil, err
		}
		return list, nil
	case "version":
		ver, err := ctx.Clients.Discovery.ServerVersion()
		if err != nil {
			return nil, err
		}
		return ver, nil
	case "api-resources":
		resources, err := ctx.Clients.Discovery.ServerPreferredResources()
		if err != nil {
			return nil, err
		}
		return resources, nil
	default:
		return nil, sdkmcp.ResourceNotFoundError("cluster://" + resource)
	}
}

func readManifestResource(callCtx context.Context, ctx ToolContext, user policy.User, parsed *url.URL) (string, error) {
	resource := strings.TrimSpace(parsed.Host)
	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(parts) != 2 {
		return "", sdkmcp.ResourceNotFoundError(parsed.String())
	}
	namespace := strings.TrimSpace(parts[0])
	name := strings.TrimSpace(parts[1])
	if namespace == "" || name == "" {
		return "", sdkmcp.ResourceNotFoundError(parsed.String())
	}
	if err := ctx.Policy.CheckNamespace(user, namespace, true); err != nil {
		return "", err
	}

	var obj any
	switch resource {
	case "deployments":
		v, err := ctx.Clients.Typed.AppsV1().Deployments(namespace).Get(callCtx, name, metav1.GetOptions{})
		if err != nil {
			return "", err
		}
		obj = v
	case "services":
		v, err := ctx.Clients.Typed.CoreV1().Services(namespace).Get(callCtx, name, metav1.GetOptions{})
		if err != nil {
			return "", err
		}
		obj = v
	case "pods":
		v, err := ctx.Clients.Typed.CoreV1().Pods(namespace).Get(callCtx, name, metav1.GetOptions{})
		if err != nil {
			return "", err
		}
		obj = v
	case "configmaps":
		v, err := ctx.Clients.Typed.CoreV1().ConfigMaps(namespace).Get(callCtx, name, metav1.GetOptions{})
		if err != nil {
			return "", err
		}
		obj = v
	case "secrets":
		v, err := ctx.Clients.Typed.CoreV1().Secrets(namespace).Get(callCtx, name, metav1.GetOptions{})
		if err != nil {
			return "", err
		}
		obj = secretToMaskedMap(v)
	case "ingresses":
		v, err := ctx.Clients.Typed.NetworkingV1().Ingresses(namespace).Get(callCtx, name, metav1.GetOptions{})
		if err != nil {
			return "", err
		}
		obj = v
	default:
		return "", sdkmcp.ResourceNotFoundError(parsed.String())
	}
	b, err := yaml.Marshal(obj)
	if err != nil {
		return "", err
	}
	if ctx.Redactor != nil {
		return ctx.Redactor.RedactString(string(b)), nil
	}
	return string(b), nil
}

func secretToMaskedMap(secret *corev1.Secret) map[string]any {
	if secret == nil {
		return map[string]any{}
	}
	data := map[string]string{}
	for key := range secret.Data {
		data[key] = "[REDACTED]"
	}
	for key := range secret.StringData {
		data[key] = "[REDACTED]"
	}
	metadata := map[string]any{
		"name":      secret.Name,
		"namespace": secret.Namespace,
	}
	if len(secret.Labels) > 0 {
		metadata["labels"] = secret.Labels
	}
	out := map[string]any{
		"apiVersion": "v1",
		"kind":       "Secret",
		"metadata":   metadata,
		"type":       secret.Type,
	}
	if len(data) > 0 {
		out["data"] = data
	}
	return out
}

func loadRawKubeconfig(ctx ToolContext) (*clientcmdapi.Config, error) {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	if ctx.Config != nil {
		if explicit := kubeconfigPath(ctx.Config.Kubeconfig); explicit != "" {
			loadingRules.ExplicitPath = explicit
		}
	}
	raw, err := loadingRules.Load()
	if err != nil {
		return nil, err
	}
	if ctx.Config != nil && strings.TrimSpace(ctx.Config.Context) != "" {
		raw.CurrentContext = strings.TrimSpace(ctx.Config.Context)
	}
	return raw, nil
}

func kubeconfigPath(path string) string {
	if path == "" {
		return ""
	}
	if strings.HasPrefix(path, "~") {
		home := homedir.HomeDir()
		if home == "" {
			return path
		}
		rest := strings.TrimPrefix(path, "~")
		rest = strings.TrimPrefix(rest, string(os.PathSeparator))
		if rest == "" {
			return home
		}
		return filepath.Join(home, rest)
	}
	return os.ExpandEnv(path)
}

package gcp

import (
	"os"
	"strings"

	"k8s.io/client-go/tools/clientcmd"
)

const (
	envProject     = "GOOGLE_CLOUD_PROJECT"
	envProjectAlt  = "GCP_PROJECT"
	envCredentials = "GOOGLE_APPLICATION_CREDENTIALS"
)

func ResolveProject(explicit string) string {
	if v := strings.TrimSpace(explicit); v != "" {
		return v
	}
	if v := strings.TrimSpace(os.Getenv(envProject)); v != "" {
		return v
	}
	if v := strings.TrimSpace(os.Getenv(envProjectAlt)); v != "" {
		return v
	}
	return ""
}

func CredentialsFile() string {
	return strings.TrimSpace(os.Getenv(envCredentials))
}

// ResolveProjectWithKubeconfig extends ResolveProject with a final fallback that
// reads the current kubeconfig context. When the context name follows GKE's
// canonical `gke_PROJECT_REGION_CLUSTER` pattern (created by
// `gcloud container clusters get-credentials`), the project segment is returned.
func ResolveProjectWithKubeconfig(explicit string) string {
	if v := ResolveProject(explicit); v != "" {
		return v
	}
	return ResolveProjectFromKubeconfig()
}

// ResolveProjectFromKubeconfig loads the default kubeconfig and returns the GKE
// project encoded in the current context name, or "" if not available.
func ResolveProjectFromKubeconfig() string {
	rules := clientcmd.NewDefaultClientConfigLoadingRules()
	raw, err := rules.Load()
	if err != nil || raw == nil {
		return ""
	}
	return ExtractGKEProject(raw.CurrentContext)
}

// ExtractGKEProject parses a kubeconfig context name in the form
// `gke_PROJECT_REGION_CLUSTER` and returns the PROJECT segment. Returns "" when
// the name does not match that pattern.
func ExtractGKEProject(contextName string) string {
	name := strings.TrimSpace(contextName)
	if !strings.HasPrefix(name, "gke_") {
		return ""
	}
	parts := strings.SplitN(name, "_", 4)
	if len(parts) < 4 {
		return ""
	}
	project := strings.TrimSpace(parts[1])
	if project == "" {
		return ""
	}
	return project
}

package gcp

import (
	"os"
	"strings"
)

const (
	envProject     = "GOOGLE_CLOUD_PROJECT"
	envProjectAlt  = "GCP_PROJECT"
	envCredentials = "GOOGLE_APPLICATION_CREDENTIALS"
)

// ResolveProject returns the explicit project id when non-empty, else the
// value of GOOGLE_CLOUD_PROJECT, else GCP_PROJECT, else "". Use
// ResolveProjectWithConfig when a [gcp].project config-file fallback is
// available.
//
// Project resolution is intentionally decoupled from the active kubeconfig
// context: EKS/AKS/GKE clusters can all ship telemetry to GCP, so the
// observability project must come from the user, not from cluster identity.
func ResolveProject(explicit string) string {
	return ResolveProjectWithConfig(explicit, "")
}

// ResolveProjectWithConfig adds a config-file fallback to the resolution
// chain. Order: explicit per-call arg > GOOGLE_CLOUD_PROJECT env > GCP_PROJECT
// env > [gcp].project config field > "".
func ResolveProjectWithConfig(explicit, cfgProject string) string {
	if v := strings.TrimSpace(explicit); v != "" {
		return v
	}
	if v := strings.TrimSpace(os.Getenv(envProject)); v != "" {
		return v
	}
	if v := strings.TrimSpace(os.Getenv(envProjectAlt)); v != "" {
		return v
	}
	if v := strings.TrimSpace(cfgProject); v != "" {
		return v
	}
	return ""
}

// CredentialsFile returns the path read from GOOGLE_APPLICATION_CREDENTIALS,
// falling back to "" when unset. Use CredentialsFileWithConfig to add a
// [gcp].credentials_file config fallback.
func CredentialsFile() string {
	return CredentialsFileWithConfig("")
}

// CredentialsFileWithConfig returns GOOGLE_APPLICATION_CREDENTIALS when set,
// else the cfgFile value (typically [gcp].credentials_file). Returns "" when
// neither is set; the SDK will then fall back to Application Default
// Credentials.
func CredentialsFileWithConfig(cfgFile string) string {
	if v := strings.TrimSpace(os.Getenv(envCredentials)); v != "" {
		return v
	}
	return strings.TrimSpace(cfgFile)
}

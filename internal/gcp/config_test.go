package gcp

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveProjectExplicit(t *testing.T) {
	if got := ResolveProject("my-project"); got != "my-project" {
		t.Fatalf("expected explicit value, got %q", got)
	}
}

func TestResolveProjectFromEnv(t *testing.T) {
	t.Setenv(envProject, "from-env")
	t.Setenv(envProjectAlt, "")
	if got := ResolveProject(""); got != "from-env" {
		t.Fatalf("expected env value, got %q", got)
	}
}

func TestResolveProjectFromAlt(t *testing.T) {
	t.Setenv(envProject, "")
	t.Setenv(envProjectAlt, "from-alt")
	if got := ResolveProject(""); got != "from-alt" {
		t.Fatalf("expected alt env value, got %q", got)
	}
}

func TestResolveProjectEmpty(t *testing.T) {
	t.Setenv(envProject, "")
	t.Setenv(envProjectAlt, "")
	if got := ResolveProject("   "); got != "" {
		t.Fatalf("expected empty, got %q", got)
	}
}

func TestExtractGKEProject(t *testing.T) {
	cases := map[string]string{
		"gke_my-project_us-central1_my-cluster":          "my-project",
		"gke_p_us-central1-c_cluster-name":               "p",
		"gke_proj-123_europe-west4-a_cluster_with_under": "proj-123",
		"gke_":                          "",
		"gke_only-two":                  "",
		"gke_only_three":                "",
		"arn:aws:eks:us-east-1:cluster": "",
		"":                              "",
		"  gke_p_r_c  ":                 "p",
	}
	for in, want := range cases {
		if got := ExtractGKEProject(in); got != want {
			t.Errorf("ExtractGKEProject(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestResolveProjectFromKubeconfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "kubeconfig")
	const data = `apiVersion: v1
kind: Config
clusters:
- name: gke_kube-proj_us-central1_my-cluster
  cluster:
    server: https://example.invalid
contexts:
- name: gke_kube-proj_us-central1_my-cluster
  context:
    cluster: gke_kube-proj_us-central1_my-cluster
    user: gke_kube-proj_us-central1_my-cluster
users:
- name: gke_kube-proj_us-central1_my-cluster
  user: {}
current-context: gke_kube-proj_us-central1_my-cluster
`
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatalf("write kubeconfig: %v", err)
	}
	t.Setenv("KUBECONFIG", path)
	if got := ResolveProjectFromKubeconfig(); got != "kube-proj" {
		t.Fatalf("ResolveProjectFromKubeconfig = %q, want %q", got, "kube-proj")
	}
}

func TestResolveProjectWithKubeconfigPrefersEnv(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "kubeconfig")
	const data = `apiVersion: v1
kind: Config
contexts:
- name: gke_other-proj_us-central1_c
  context: {cluster: c, user: u}
current-context: gke_other-proj_us-central1_c
`
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatalf("write kubeconfig: %v", err)
	}
	t.Setenv("KUBECONFIG", path)
	t.Setenv(envProject, "env-proj")
	t.Setenv(envProjectAlt, "")
	if got := ResolveProjectWithKubeconfig(""); got != "env-proj" {
		t.Fatalf("expected env priority, got %q", got)
	}
}

func TestResolveProjectWithKubeconfigNonGKEContextReturnsEmpty(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "kubeconfig")
	const data = `apiVersion: v1
kind: Config
contexts:
- name: minikube
  context: {cluster: c, user: u}
current-context: minikube
`
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatalf("write kubeconfig: %v", err)
	}
	t.Setenv("KUBECONFIG", path)
	t.Setenv(envProject, "")
	t.Setenv(envProjectAlt, "")
	if got := ResolveProjectWithKubeconfig(""); got != "" {
		t.Fatalf("expected empty (non-GKE context), got %q", got)
	}
}

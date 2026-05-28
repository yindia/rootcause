package gcp

import "testing"

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

func TestResolveProjectPrefersExplicitOverEnv(t *testing.T) {
	t.Setenv(envProject, "env-proj")
	t.Setenv(envProjectAlt, "alt-proj")
	if got := ResolveProject("explicit"); got != "explicit" {
		t.Fatalf("expected explicit to win, got %q", got)
	}
}

func TestResolveProjectWithConfigPrecedence(t *testing.T) {
	// Explicit beats env beats config.
	t.Setenv(envProject, "env-proj")
	t.Setenv(envProjectAlt, "")
	if got := ResolveProjectWithConfig("explicit", "cfg-proj"); got != "explicit" {
		t.Errorf("explicit should win, got %q", got)
	}
	if got := ResolveProjectWithConfig("", "cfg-proj"); got != "env-proj" {
		t.Errorf("env should beat config when both set, got %q", got)
	}
	// Config fallback when nothing else is set.
	t.Setenv(envProject, "")
	t.Setenv(envProjectAlt, "")
	if got := ResolveProjectWithConfig("", "cfg-proj"); got != "cfg-proj" {
		t.Errorf("config should win when explicit and env empty, got %q", got)
	}
	// Truly empty.
	if got := ResolveProjectWithConfig("", ""); got != "" {
		t.Errorf("expected empty result, got %q", got)
	}
}

func TestCredentialsFileWithConfigPrecedence(t *testing.T) {
	t.Setenv(envCredentials, "/env/creds.json")
	if got := CredentialsFileWithConfig("/cfg/creds.json"); got != "/env/creds.json" {
		t.Errorf("env should win over config, got %q", got)
	}
	t.Setenv(envCredentials, "")
	if got := CredentialsFileWithConfig("/cfg/creds.json"); got != "/cfg/creds.json" {
		t.Errorf("config should be returned when env empty, got %q", got)
	}
	if got := CredentialsFileWithConfig(""); got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}

package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	rcconfig "rootcause/internal/config"
	"rootcause/pkg/server"
)

func TestHomeConfigPathUsesOSPathJoin(t *testing.T) {
	home := filepath.Join(string(filepath.Separator), "Users", "tester")
	got := homeConfigPath(home)
	want := filepath.Join(home, ".rootcause", "config.yaml")
	if got != want {
		t.Fatalf("expected home config path %q, got %q", want, got)
	}
}

func TestInitHomeConfigWritesAllEnabledConfigAndSkillsDir(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".rootcause", "config.yaml")
	written, err := initHomeConfig(path, false)
	if err != nil {
		t.Fatalf("initHomeConfig: %v", err)
	}
	if written != path {
		t.Fatalf("expected written path %q, got %q", path, written)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat config: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("expected config mode 0600, got %v", info.Mode().Perm())
	}
	_, err = os.Stat(filepath.Join(filepath.Dir(path), "skills"))
	if err != nil {
		t.Fatalf("expected custom skills directory: %v", err)
	}
	_, err = os.Stat(filepath.Join(filepath.Dir(path), "prompts"))
	if err != nil {
		t.Fatalf("expected custom prompts directory: %v", err)
	}

	var cfg rcconfig.Config
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("decode config: %v", err)
	}
	expectedToolsets := []string{"k8s", "linkerd", "karpenter", "istio", "helm", "aws", "terraform", "observability", "rootcause"}
	if strings.Join(cfg.Toolsets, ",") != strings.Join(expectedToolsets, ",") {
		t.Fatalf("expected all toolsets enabled, got %#v", cfg.Toolsets)
	}
	if cfg.ReadOnly {
		t.Fatalf("expected read_only=false")
	}
	if cfg.DisableDestructive {
		t.Fatalf("expected disable_destructive=false")
	}
	if len(cfg.Skills.CustomDirs) != 1 || cfg.Skills.CustomDirs[0] != "~/.rootcause/skills" {
		t.Fatalf("expected default custom skills dir, got %#v", cfg.Skills.CustomDirs)
	}
	if cfg.Prompts.Dir != "~/.rootcause/prompts" {
		t.Fatalf("expected default prompts.dir to be set, got %q", cfg.Prompts.Dir)
	}
}

func TestInitHomeConfigRefusesExistingWithoutOverwrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".rootcause", "config.yaml")
	_, err := initHomeConfig(path, false)
	if err != nil {
		t.Fatalf("initHomeConfig first: %v", err)
	}
	_, err = initHomeConfig(path, false)
	if err == nil {
		t.Fatalf("expected existing config error")
	}
	if !strings.Contains(err.Error(), "--overwrite") {
		t.Fatalf("expected overwrite guidance, got %v", err)
	}
	_, err = initHomeConfig(path, true)
	if err != nil {
		t.Fatalf("expected overwrite to succeed: %v", err)
	}
}

func TestExecuteInitConfigDoesNotRunServer(t *testing.T) {
	called := false
	run := func(context.Context, server.Options) error {
		called = true
		return nil
	}
	path := filepath.Join(t.TempDir(), ".rootcause", "config.yaml")
	var out bytes.Buffer
	err := Execute(context.Background(), []string{"init-config", "--path", path}, run, "test", &out)
	if err != nil {
		t.Fatalf("execute init-config: %v", err)
	}
	if called {
		t.Fatalf("expected runServer not to be called for init-config")
	}
	if !strings.Contains(out.String(), path) {
		t.Fatalf("expected output to mention path %q, got %q", path, out.String())
	}
}

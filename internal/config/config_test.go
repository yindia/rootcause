package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadWithOverridesAndDropIns(t *testing.T) {
	dir := t.TempDir()
	mainCfg := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(mainCfg, []byte(`
toolsets = ["k8s"]
read_only = true
log_level = "debug"
`), 0600); err != nil {
		t.Fatalf("write main config: %v", err)
	}

	dropInDir := filepath.Join(dir, "dropins")
	if err := os.MkdirAll(dropInDir, 0700); err != nil {
		t.Fatalf("mkdir dropins: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dropInDir, "10-base.toml"), []byte(`
disable_destructive = true
log_level = "info"
`), 0600); err != nil {
		t.Fatalf("write dropin: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dropInDir, "20-override.toml"), []byte(`
log_level = "warn"
toolsets = ["k8s","aws"]
`), 0600); err != nil {
		t.Fatalf("write dropin: %v", err)
	}

	overrideReadOnly := false
	overrideContext := "demo"
	cfg, err := Load(mainCfg, dropInDir, Overrides{ReadOnly: &overrideReadOnly, Context: &overrideContext})
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.ReadOnly {
		t.Fatalf("expected override read_only false")
	}
	if cfg.DisableDestructive != true {
		t.Fatalf("expected disable_destructive from drop-in")
	}
	if cfg.LogLevel != "warn" {
		t.Fatalf("expected drop-in override log_level, got %q", cfg.LogLevel)
	}
	if cfg.Context != "demo" {
		t.Fatalf("expected override context, got %q", cfg.Context)
	}
	if len(cfg.Toolsets) != 2 || cfg.Toolsets[0] != "k8s" || cfg.Toolsets[1] != "aws" {
		t.Fatalf("expected toolsets overridden from drop-in, got %#v", cfg.Toolsets)
	}
}

func TestLoadExecAndSafetyConfig(t *testing.T) {
	dir := t.TempDir()
	mainCfg := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(mainCfg, []byte(`
[exec_readonly]
enabled = true
allowed_commands = ["echo"]

[safety]
allow_destructive_tools = ["k8s.delete"]
`), 0600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	cfg, err := Load(mainCfg, "", Overrides{})
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if !cfg.Exec.Enabled || len(cfg.Exec.AllowedCommands) != 1 || cfg.Exec.AllowedCommands[0] != "echo" {
		t.Fatalf("unexpected exec config: %#v", cfg.Exec)
	}
	if len(cfg.Safety.AllowDestructiveTools) != 1 || cfg.Safety.AllowDestructiveTools[0] != "k8s.delete" {
		t.Fatalf("unexpected safety config: %#v", cfg.Safety)
	}
}

func TestDropInFilesMissingDir(t *testing.T) {
	files, err := dropInFiles(filepath.Join(t.TempDir(), "missing"))
	if err != nil {
		t.Fatalf("dropInFiles: %v", err)
	}
	if len(files) != 0 {
		t.Fatalf("expected no files, got %#v", files)
	}
}

func TestReadFileMissing(t *testing.T) {
	_, err := readFile(filepath.Join(t.TempDir(), "missing.toml"))
	if err == nil {
		t.Fatalf("expected error for missing file")
	}
}

func TestReadFileInvalidTOML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.toml")
	if err := os.WriteFile(path, []byte("invalid = ["), 0600); err != nil {
		t.Fatalf("write file: %v", err)
	}
	_, err := readFile(path)
	if err == nil {
		t.Fatalf("expected error for invalid toml")
	}
}

func TestMergeTimeoutsAndCache(t *testing.T) {
	dst := Config{}
	src := Config{
		ReadOnly: true,
		Timeouts: TimeoutConfig{
			DefaultSeconds: 10,
			MaxSeconds:     20,
			PerTool:        map[string]int{"k8s.get": 5},
		},
		Cache: CacheConfig{
			DiscoveryTTLSeconds: 11,
			GraphTTLSeconds:     12,
			AWSListTTLSeconds:   13,
		},
		Exec: ExecConfig{
			Enabled:         true,
			AllowedCommands: []string{"echo"},
		},
	}
	merge(&dst, src)
	if !dst.ReadOnly {
		t.Fatalf("expected read_only to be set")
	}
	if dst.Timeouts.DefaultSeconds != 10 || dst.Timeouts.MaxSeconds != 20 {
		t.Fatalf("unexpected timeouts: %#v", dst.Timeouts)
	}
	if dst.Timeouts.PerTool["k8s.get"] != 5 {
		t.Fatalf("expected per-tool timeout")
	}
	if dst.Cache.DiscoveryTTLSeconds != 11 || dst.Cache.GraphTTLSeconds != 12 || dst.Cache.AWSListTTLSeconds != 13 {
		t.Fatalf("unexpected cache config: %#v", dst.Cache)
	}
	if !dst.Exec.Enabled || len(dst.Exec.AllowedCommands) != 1 {
		t.Fatalf("unexpected exec config: %#v", dst.Exec)
	}
}

func TestApplyOverrides(t *testing.T) {
	cfg := DefaultConfig()
	toolsets := []string{"k8s"}
	readOnly := true
	disable := true
	logLevel := "warn"
	kubeconfig := "/tmp/kubeconfig"
	context := "demo"
	applyOverrides(&cfg, Overrides{
		Kubeconfig:         &kubeconfig,
		Context:            &context,
		Toolsets:           &toolsets,
		ReadOnly:           &readOnly,
		DisableDestructive: &disable,
		LogLevel:           &logLevel,
	})
	if cfg.Kubeconfig != kubeconfig || cfg.Context != context {
		t.Fatalf("unexpected overrides: %#v", cfg)
	}
	if len(cfg.Toolsets) != 1 || cfg.Toolsets[0] != "k8s" {
		t.Fatalf("unexpected toolsets: %#v", cfg.Toolsets)
	}
	if !cfg.ReadOnly || !cfg.DisableDestructive || cfg.LogLevel != "warn" {
		t.Fatalf("unexpected overrides applied: %#v", cfg)
	}
}

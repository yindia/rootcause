package config

import (
	"errors"
	"path/filepath"
	"sort"

	"os"

	"github.com/BurntSushi/toml"
)

type Config struct {
	Kubeconfig         string       `toml:"kubeconfig"`
	Context            string       `toml:"context"`
	Toolsets           []string     `toml:"toolsets"`
	ReadOnly           bool         `toml:"read_only"`
	DisableDestructive bool         `toml:"disable_destructive"`
	LogLevel           string       `toml:"log_level"`
	Safety             SafetyConfig `toml:"safety"`
	Exec               ExecConfig   `toml:"exec_readonly"`
}

type SafetyConfig struct {
	AllowDestructiveTools []string `toml:"allow_destructive_tools"`
}

type ExecConfig struct {
	Enabled         bool     `toml:"enabled"`
	AllowedCommands []string `toml:"allowed_commands"`
}

type Overrides struct {
	Kubeconfig         *string
	Context            *string
	Toolsets           *[]string
	ReadOnly           *bool
	DisableDestructive *bool
	LogLevel           *string
}

func DefaultConfig() Config {
	return Config{
		Kubeconfig: "",
		Toolsets:   []string{"k8s", "linkerd", "karpenter", "istio"},
		LogLevel:   "info",
	}
}

func Load(path string, dir string, overrides Overrides) (Config, error) {
	cfg := DefaultConfig()

	if path != "" {
		fileCfg, err := readFile(path)
		if err != nil {
			return cfg, err
		}
		merge(&cfg, fileCfg)
	}

	if dir != "" {
		files, err := dropInFiles(dir)
		if err != nil {
			return cfg, err
		}
		for _, file := range files {
			fileCfg, err := readFile(file)
			if err != nil {
				return cfg, err
			}
			merge(&cfg, fileCfg)
		}
	}

	applyOverrides(&cfg, overrides)
	return cfg, nil
}

func readFile(path string) (Config, error) {
	var cfg Config
	if _, err := os.Stat(path); err != nil {
		return cfg, err
	}
	if _, err := toml.DecodeFile(path, &cfg); err != nil {
		return cfg, err
	}
	return cfg, nil
}

func dropInFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	var files []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		files = append(files, filepath.Join(dir, entry.Name()))
	}
	sort.Strings(files)
	return files, nil
}

func merge(dst *Config, src Config) {
	if src.Kubeconfig != "" {
		dst.Kubeconfig = src.Kubeconfig
	}
	if src.Context != "" {
		dst.Context = src.Context
	}
	if len(src.Toolsets) > 0 {
		dst.Toolsets = append([]string{}, src.Toolsets...)
	}
	if src.ReadOnly {
		dst.ReadOnly = src.ReadOnly
	}
	if src.DisableDestructive {
		dst.DisableDestructive = src.DisableDestructive
	}
	if src.LogLevel != "" {
		dst.LogLevel = src.LogLevel
	}
	if len(src.Safety.AllowDestructiveTools) > 0 {
		dst.Safety.AllowDestructiveTools = append([]string{}, src.Safety.AllowDestructiveTools...)
	}
	if src.Exec.Enabled {
		dst.Exec.Enabled = src.Exec.Enabled
	}
	if len(src.Exec.AllowedCommands) > 0 {
		dst.Exec.AllowedCommands = append([]string{}, src.Exec.AllowedCommands...)
	}
}

func applyOverrides(cfg *Config, overrides Overrides) {
	if overrides.Kubeconfig != nil {
		cfg.Kubeconfig = *overrides.Kubeconfig
	}
	if overrides.Context != nil {
		cfg.Context = *overrides.Context
	}
	if overrides.Toolsets != nil {
		cfg.Toolsets = append([]string{}, (*overrides.Toolsets)...)
	}
	if overrides.ReadOnly != nil {
		cfg.ReadOnly = *overrides.ReadOnly
	}
	if overrides.DisableDestructive != nil {
		cfg.DisableDestructive = *overrides.DisableDestructive
	}
	if overrides.LogLevel != nil {
		cfg.LogLevel = *overrides.LogLevel
	}
}

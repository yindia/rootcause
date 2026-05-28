package config

import (
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"sort"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Kubeconfig         string          `yaml:"kubeconfig"`
	Context            string          `yaml:"context"`
	Toolsets           []string        `yaml:"toolsets"`
	ReadOnly           bool            `yaml:"read_only"`
	DisableDestructive bool            `yaml:"disable_destructive"`
	LogLevel           string          `yaml:"log_level"`
	Safety             SafetyConfig    `yaml:"safety"`
	Exec               ExecConfig      `yaml:"exec_readonly"`
	Timeouts           TimeoutConfig   `yaml:"timeouts"`
	Cache              CacheConfig     `yaml:"cache"`
	Transport          TransportConfig `yaml:"transport"`
	Prompts            PromptsConfig   `yaml:"prompts"`
	Skills             SkillsConfig    `yaml:"skills"`
	Limits             LimitsConfig    `yaml:"limits"`
	GCP                GCPConfig            `yaml:"gcp"`
	AWS                AWSConfig            `yaml:"aws"`
	Observability      ObservabilityConfig  `yaml:"observability"`
}

// GCPConfig holds baseline GCP auth defaults that apply to every gcp.* tool
// regardless of which project is targeted. Project IDs themselves are NOT
// here — observability projects live under observability.gcp because
// observability is a cross-cutting concern (a team may route GCP logs to a
// different project than where the cluster lives, or use a non-GCP backend
// entirely).
type GCPConfig struct {
	// CredentialsFile is the path to a service-account JSON key used by all
	// gcp.* tools. Falls back to GOOGLE_APPLICATION_CREDENTIALS env, then
	// Application Default Credentials.
	CredentialsFile string `yaml:"credentials_file"`
}

// ObservabilityConfig declares the telemetry backends RootCause should query.
// Observability is intentionally separated from per-cloud auth config: a team
// may have GCP workloads but ship logs to Datadog, or have an EKS cluster
// shipping to a centralized GCP project. The cloud the workload runs in is
// often not the cloud where its telemetry lives.
//
// Today only the GCP backend is implemented (see observability.gcp). When
// CloudWatch / Datadog / Grafana support lands, they slot in as siblings
// here without rearranging existing config.
type ObservabilityConfig struct {
	// GCP backend defaults (Cloud Monitoring + Cloud Logging).
	GCP ObservabilityGCPConfig `yaml:"gcp"`
}

// ObservabilityGCPConfig holds defaults for the GCP-backed observability
// tools (gcp.metrics.*, gcp.logs.*, gcp.metrics.slo_list).
type ObservabilityGCPConfig struct {
	// Project is the default GCP project hosting Cloud Monitoring metrics,
	// Cloud Logging entries, and Service Monitoring SLOs. Falls back through
	// the standard chain: per-call projectId arg > GOOGLE_CLOUD_PROJECT env
	// > GCP_PROJECT env > this field.
	Project string `yaml:"project"`
	// CredentialsFile optionally overrides the gcp section.credentials_file just for
	// observability calls. Useful when telemetry lives in a different account
	// than the rest of your GCP estate. Leave empty to reuse the gcp section
	// credentials.
	CredentialsFile string `yaml:"credentials_file"`
}

// AWSConfig holds AWS defaults consumed by the aws toolset. Values here are
// used as a fallback when explicit per-call arguments and the standard
// AWS_REGION / AWS_PROFILE env vars are not set.
//
// SSO setups do NOT need credentials_file: the SDK picks up SSO sessions via
// the named profile in ~/.aws/config after `aws sso login` (or `aws
// configure sso`). Only set credentials_file for static-key workflows that
// need a non-standard shared-credentials path.
type AWSConfig struct {
	// Region is the default region (e.g. "us-east-1"). Used when no
	// AWS_REGION / AWS_DEFAULT_REGION env var is set.
	Region string `yaml:"region"`
	// Profile is the default shared-config profile name. Used when no
	// AWS_PROFILE / AWS_DEFAULT_PROFILE env var is set. For SSO, this is the
	// profile that contains the sso_session / sso_account_id / sso_role_name
	// entries in ~/.aws/config.
	Profile string `yaml:"profile"`
	// CredentialsFile is an explicit path to the shared credentials file.
	// Optional. Leave empty for SSO, instance metadata, environment
	// credentials, and the SDK default discovery chain.
	CredentialsFile string `yaml:"credentials_file"`
}

type LimitsConfig struct {
	MaxCallDepth   int  `yaml:"max_call_depth"`
	MaxResultBytes int  `yaml:"max_result_bytes"`
	MaxCallGraph   int  `yaml:"max_call_graph"`
	StrictSchema   bool `yaml:"strict_schema"`
}

type SafetyConfig struct {
	AllowDestructiveTools []string `yaml:"allow_destructive_tools"`
}

type ExecConfig struct {
	Enabled         bool     `yaml:"enabled"`
	AllowedCommands []string `yaml:"allowed_commands"`
}

type TimeoutConfig struct {
	DefaultSeconds int            `yaml:"default_seconds"`
	MaxSeconds     int            `yaml:"max_seconds"`
	PerTool        map[string]int `yaml:"per_tool"`
}

type CacheConfig struct {
	DiscoveryTTLSeconds int `yaml:"discovery_ttl_seconds"`
	GraphTTLSeconds     int `yaml:"graph_ttl_seconds"`
	AWSListTTLSeconds   int `yaml:"aws_list_ttl_seconds"`
}

type TransportConfig struct {
	Mode string `yaml:"mode"`
	Host string `yaml:"host"`
	Port int    `yaml:"port"`
	Path string `yaml:"path"`
}

type PromptsConfig struct {
	// File points at a single YAML file containing a top-level `prompts:`
	// list (one entry per prompt). Optional; the directory layout below is
	// the recommended modern format.
	File string `yaml:"file"`
	// Dir points at a directory containing one prompt per file. Files ending
	// in .md are parsed as YAML front-matter + body template (recommended).
	// Files ending in .yaml/.yml are parsed as a top-level `prompts:` list
	// (legacy multi-prompt single-file format).
	Dir string `yaml:"dir"`
}

type SkillsConfig struct {
	CustomDirs           []string `yaml:"custom_dirs"`
	AllowCustomOverrides bool     `yaml:"allow_custom_overrides"`
}

type Overrides struct {
	Kubeconfig         *string
	Context            *string
	Toolsets           *[]string
	ReadOnly           *bool
	DisableDestructive *bool
	LogLevel           *string
	TransportMode      *string
	TransportHost      *string
	TransportPort      *int
	TransportPath      *string
}

func DefaultConfig() Config {
	return Config{
		Kubeconfig: "",
		Toolsets:   []string{"k8s", "linkerd", "karpenter", "istio", "helm", "aws", "terraform", "observability", "rootcause"},
		LogLevel:   "info",
		Timeouts: TimeoutConfig{
			DefaultSeconds: 60,
			MaxSeconds:     900,
			PerTool: map[string]int{
				"k8s.port_forward": 600,
				"helm.install":     300,
				"helm.upgrade":     300,
				"helm.uninstall":   180,
			},
		},
		Cache: CacheConfig{
			DiscoveryTTLSeconds: 300,
			GraphTTLSeconds:     30,
			AWSListTTLSeconds:   60,
		},
		Transport: TransportConfig{
			Mode: "stdio",
			Host: "127.0.0.1",
			Port: 8000,
			Path: "/mcp",
		},
		Skills: SkillsConfig{
			CustomDirs: []string{"~/.rootcause/skills"},
		},
		Limits: LimitsConfig{
			MaxCallDepth:   8,
			MaxResultBytes: 8 * 1024 * 1024,
			MaxCallGraph:   10000,
		},
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
	data, err := os.ReadFile(path)
	if err != nil {
		return cfg, err
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("decode %s: %w", path, err)
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
	if src.Timeouts.DefaultSeconds > 0 {
		dst.Timeouts.DefaultSeconds = src.Timeouts.DefaultSeconds
	}
	if src.Timeouts.MaxSeconds > 0 {
		dst.Timeouts.MaxSeconds = src.Timeouts.MaxSeconds
	}
	if len(src.Timeouts.PerTool) > 0 {
		if dst.Timeouts.PerTool == nil {
			dst.Timeouts.PerTool = map[string]int{}
		}
		maps.Copy(dst.Timeouts.PerTool, src.Timeouts.PerTool)
	}
	if src.Cache.DiscoveryTTLSeconds > 0 {
		dst.Cache.DiscoveryTTLSeconds = src.Cache.DiscoveryTTLSeconds
	}
	if src.Cache.GraphTTLSeconds > 0 {
		dst.Cache.GraphTTLSeconds = src.Cache.GraphTTLSeconds
	}
	if src.Cache.AWSListTTLSeconds > 0 {
		dst.Cache.AWSListTTLSeconds = src.Cache.AWSListTTLSeconds
	}
	if src.Transport.Mode != "" {
		dst.Transport.Mode = src.Transport.Mode
	}
	if src.Transport.Host != "" {
		dst.Transport.Host = src.Transport.Host
	}
	if src.Transport.Port > 0 {
		dst.Transport.Port = src.Transport.Port
	}
	if src.Transport.Path != "" {
		dst.Transport.Path = src.Transport.Path
	}
	if src.Prompts.File != "" {
		dst.Prompts.File = src.Prompts.File
	}
	if len(src.Skills.CustomDirs) > 0 {
		dst.Skills.CustomDirs = append([]string{}, src.Skills.CustomDirs...)
	}
	if src.Skills.AllowCustomOverrides {
		dst.Skills.AllowCustomOverrides = src.Skills.AllowCustomOverrides
	}
	if src.Limits.MaxCallDepth > 0 {
		dst.Limits.MaxCallDepth = src.Limits.MaxCallDepth
	}
	if src.Limits.MaxResultBytes > 0 {
		dst.Limits.MaxResultBytes = src.Limits.MaxResultBytes
	}
	if src.Limits.MaxCallGraph > 0 {
		dst.Limits.MaxCallGraph = src.Limits.MaxCallGraph
	}
	if src.Limits.StrictSchema {
		dst.Limits.StrictSchema = src.Limits.StrictSchema
	}
	if src.Prompts.Dir != "" {
		dst.Prompts.Dir = src.Prompts.Dir
	}
	if src.GCP.CredentialsFile != "" {
		dst.GCP.CredentialsFile = src.GCP.CredentialsFile
	}
	if src.Observability.GCP.Project != "" {
		dst.Observability.GCP.Project = src.Observability.GCP.Project
	}
	if src.Observability.GCP.CredentialsFile != "" {
		dst.Observability.GCP.CredentialsFile = src.Observability.GCP.CredentialsFile
	}
	if src.AWS.Region != "" {
		dst.AWS.Region = src.AWS.Region
	}
	if src.AWS.Profile != "" {
		dst.AWS.Profile = src.AWS.Profile
	}
	if src.AWS.CredentialsFile != "" {
		dst.AWS.CredentialsFile = src.AWS.CredentialsFile
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
	if overrides.TransportMode != nil {
		cfg.Transport.Mode = *overrides.TransportMode
	}
	if overrides.TransportHost != nil {
		cfg.Transport.Host = *overrides.TransportHost
	}
	if overrides.TransportPort != nil {
		cfg.Transport.Port = *overrides.TransportPort
	}
	if overrides.TransportPath != nil {
		cfg.Transport.Path = *overrides.TransportPath
	}
}

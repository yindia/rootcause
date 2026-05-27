package mcp

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"sigs.k8s.io/yaml"
)

type promptSpec struct {
	Name        string
	Title       string
	Description string
	Arguments   []sdkmcp.PromptArgument
	Template    string
}

type promptFileConfig struct {
	Prompts []promptFilePrompt `toml:"prompt"`
}

type promptFilePrompt struct {
	Name        string               `toml:"name"`
	Title       string               `toml:"title"`
	Description string               `toml:"description"`
	Template    string               `toml:"template"`
	Arguments   []promptFileArgument `toml:"arguments"`
	Argument    []promptFileArgument `toml:"argument"`
}

type promptFileArgument struct {
	Name        string `toml:"name"`
	Description string `toml:"description"`
	Required    bool   `toml:"required"`
}

var defaultPromptConfigPaths = []string{
	"~/.rootcause/prompts.toml",
	"~/.config/rootcause/prompts.toml",
	"./rootcause-prompts.toml",
}

// defaultPromptDirs are scanned (in order) when no explicit prompts directory is
// configured. The first directory that exists is used. Per-file prompts (.md /
// .toml) inside that directory are loaded as customs that override built-ins by
// name.
var defaultPromptDirs = []string{
	"~/.rootcause/prompts",
	"~/.config/rootcause/prompts",
	"./rootcause-prompts.d",
}

var builtinPrompts = []promptSpec{
	{
		Name:        "troubleshoot_workload",
		Title:       "Troubleshoot Kubernetes Workload",
		Description: "Comprehensive troubleshooting guide for pods/deployments",
		Arguments: []sdkmcp.PromptArgument{
			{Name: "workload", Description: "Workload name", Required: true},
			{Name: "namespace", Description: "Namespace", Required: false},
			{Name: "resource_type", Description: "pod/deployment/statefulset/daemonset", Required: false},
		},
		Template: `# Kubernetes Troubleshooting: {{workload}}

Target: {{resource_type|pod}} '{{workload}}' in namespace '{{namespace|default}}'

## Step 1 - Discovery
- List candidate resources and owners for '{{workload}}'
- Check ready status, restart counts, and age
- Identify control-plane object (Deployment/StatefulSet/DaemonSet)

## Step 2 - Evidence Collection
Use tools in this order and include concrete output snippets:
1. Pod events (scheduling, mount, probe, image pull)
2. Container logs (current + previous for crash loops)
3. Describe output (conditions, probes, resources, volumes)
4. Dependency checks (Service/Endpoint, ConfigMap, Secret, PVC)

## Step 3 - Failure Pattern Analysis
Map symptoms to likely causes:
- Pending -> scheduler pressure / affinity / PVC attach wait
- CrashLoopBackOff -> startup failure / bad config / missing dependency
- ImagePullBackOff -> image/tag/registry auth issues
- OOMKilled -> request-limit mismatch or memory leak
- Unready -> probe failures, dependency latency, or startup time too short

## Step 4 - Root Cause Output Contract
For each suspected issue, return:
1. Root cause statement
2. Evidence (event/log/condition)
3. Immediate remediation action
4. Verification command/check
5. Prevention hardening recommendation

Prioritize safety-first and avoid mutating suggestions unless preflight passes.`,
	},
	{
		Name:        "deploy_application",
		Title:       "Deploy Application Safely",
		Description: "Step-by-step deployment workflow",
		Arguments: []sdkmcp.PromptArgument{
			{Name: "app_name", Description: "Application name", Required: true},
			{Name: "namespace", Description: "Namespace", Required: false},
			{Name: "replicas", Description: "Desired replica count", Required: false},
		},
		Template: `# Deployment Workflow: {{app_name}}

Namespace: {{namespace|default}}
Desired Replicas: {{replicas|1}}

## Pre-Deploy Gate
- Verify namespace, existing workloads, and service conflicts
- Require requests/limits, readiness/liveness probes, and image tag pinning
- Check topology spread / anti-affinity and PodDisruptionBudget for resilience

## Safe Rollout Steps
1. Validate manifest quality and policy constraints
2. Run mutation preflight for intended apply/patch actions
3. Apply workload + service manifests
4. Watch rollout progress and pod readiness
5. Validate endpoints, logs, and error rates

## Output Contract
- Rollout status (pass/fail)
- Blocking findings with evidence
- Exact remediation steps
- Rollback trigger and rollback plan`,
	},
	{
		Name:        "security_audit",
		Title:       "Kubernetes Security Audit",
		Description: "Security scanning and RBAC analysis workflow",
		Arguments: []sdkmcp.PromptArgument{
			{Name: "namespace", Description: "Optional namespace", Required: false},
			{Name: "scope", Description: "quick/full", Required: false},
		},
		Template: `# Security Audit

Scope: {{scope|full}}
Target: {{namespace|all namespaces}}

## Audit Checklist
1. RBAC over-permissioning (cluster-admin, wildcards)
2. ServiceAccount token exposure and automount defaults
3. Pod/container security context gaps (root, privilege escalation, caps)
4. Secrets handling and ConfigMap misuse
5. Network policy coverage and default deny posture
6. Workload hardening (PDB, quotas, resource governance)

## Findings Format
For each finding include severity, resource, risk, evidence, remediation, and verification steps.
Return top critical issues first.`,
	},
	{
		Name:        "cost_optimization",
		Title:       "Kubernetes Cost Optimization",
		Description: "Resource optimization and cost analysis workflow",
		Arguments: []sdkmcp.PromptArgument{
			{Name: "namespace", Description: "Optional namespace", Required: false},
		},
		Template: `# Cost Optimization Analysis

Target: {{namespace|cluster-wide}}

## Phase 1 - Utilization Reality Check
- Compare requests/limits vs observed usage
- Identify underutilized pods, nodes, and storage volumes

## Phase 2 - Optimization Plan
- Right-size workload resources
- Recommend HPA/VPA candidates
- Identify node consolidation opportunities
- Flag expensive idle services/load balancers

## Output Contract
Provide quick wins, medium-term actions, and estimated impact in priority order.`,
	},
	{
		Name:        "disaster_recovery",
		Title:       "Disaster Recovery Plan",
		Description: "Backup and recovery planning workflow",
		Arguments: []sdkmcp.PromptArgument{
			{Name: "namespace", Description: "Optional namespace", Required: false},
			{Name: "dr_type", Description: "full/namespace/data-only", Required: false},
		},
		Template: `# Disaster Recovery Workflow

Target: {{namespace|cluster-wide}}
Mode: {{dr_type|full}}

## Prepare
1. Inventory workloads, services, configs, secrets, and PVCs
2. Define backup scope and restore dependencies
3. Verify backup artifacts and integrity

## Recover
1. Restore cluster prerequisites
2. Restore namespace and dependencies in safe order
3. Restore data volumes and verify application integrity

## Validate
- Workload health, endpoint readiness, and data consistency checks
- RTO/RPO notes and prevention actions`,
	},
	{
		Name:        "debug_networking",
		Title:       "Debug Service Networking",
		Description: "Network debugging for services and connectivity",
		Arguments: []sdkmcp.PromptArgument{
			{Name: "service", Description: "Service name", Required: true},
			{Name: "namespace", Description: "Namespace", Required: false},
		},
		Template: `# Network Debugging: {{service}}

Namespace: {{namespace|default}}

## Investigation Path
1. Validate Service spec and selector
2. Verify Endpoint population and backend pod readiness
3. Check DNS resolution chain
4. Inspect NetworkPolicy/mesh ingress-egress rules
5. Confirm port mappings and app listeners

## Output Contract
- Broken hop in network path
- Evidence from endpoints/events/config
- Exact fix and post-fix verification steps`,
	},
	{
		Name:        "scale_application",
		Title:       "Scale Application Safely",
		Description: "Scaling guide with HPA/VPA best practices",
		Arguments: []sdkmcp.PromptArgument{
			{Name: "workload", Description: "Workload name", Required: true},
			{Name: "namespace", Description: "Namespace", Required: false},
			{Name: "target_replicas", Description: "Target replicas", Required: false},
		},
		Template: `# Scaling Plan: {{workload}}

Namespace: {{namespace|default}}
Target Replicas: {{target_replicas|3}}

## Before Scaling
- Validate current utilization and node capacity
- Check PDB, affinity, spread, and dependency limits

## Execute
1. Scale incrementally
2. Watch rollout and readiness
3. Observe latency/error signals

## Output Contract
- Capacity verdict
- Scaling bottlenecks
- Recommended HPA/VPA settings
- Rollback trigger criteria`,
	},
	{
		Name:        "upgrade_cluster",
		Title:       "Kubernetes Upgrade Plan",
		Description: "Kubernetes cluster upgrade planning",
		Arguments: []sdkmcp.PromptArgument{
			{Name: "current_version", Description: "Current Kubernetes version", Required: false},
			{Name: "target_version", Description: "Target Kubernetes version", Required: false},
		},
		Template: `# Cluster Upgrade Plan

Current: {{current_version|current cluster version}}
Target: {{target_version|next supported version}}

## Plan
1. Validate version skew and API deprecations
2. Check addon and workload compatibility
3. Back up control plane state and manifests

## Execute
1. Upgrade control plane first
2. Upgrade nodes with controlled drain/uncordon sequence
3. Validate system and application health after each phase

## Output Contract
- Upgrade readiness verdict
- Blocking risks
- Rollback strategy
- Post-upgrade validation checklist`,
	},
	{
		Name:        "sre_incident_commander",
		Title:       "SRE Incident Commander Runbook",
		Description: "Severity-based incident coordination and triage workflow",
		Arguments: []sdkmcp.PromptArgument{
			{Name: "service", Description: "Primary impacted service", Required: true},
			{Name: "severity", Description: "sev1/sev2/sev3", Required: false},
			{Name: "namespace", Description: "Impacted namespace", Required: false},
		},
		Template: `# SRE Incident Commander: {{service}}

Severity: {{severity|sev2}}
Namespace: {{namespace|default}}

## Phase 1 - Triage (First 10 Minutes)
1. Confirm blast radius and customer impact
2. Build evidence bundle and timeline
3. Identify current mitigation options and risk

## Phase 2 - Stabilization
1. Stop further damage (rollback/traffic shift/feature flag)
2. Validate service recovery indicators
3. Track residual risk and unresolved symptoms

## Phase 3 - Communication + Follow-up
1. Publish concise status updates
2. Record root cause hypotheses with evidence
3. Define next actions for permanent fix and postmortem

## Output Contract
- Incident status
- Most likely root causes
- Active mitigation steps
- Verification and next checkpoints`,
	},
	{
		Name:        "istio_mesh_diagnose",
		Title:       "Istio Service Mesh Diagnosis",
		Description: "Diagnose Istio control-plane and traffic policy issues",
		Arguments: []sdkmcp.PromptArgument{
			{Name: "namespace", Description: "Target namespace", Required: false},
			{Name: "service", Description: "Optional service name", Required: false},
		},
		Template: `# Istio Mesh Diagnosis

Namespace: {{namespace|all namespaces}}
Service: {{service|all services}}

## Investigation Path
1. Verify Istio control-plane and sidecar injection health
2. Inspect destination rules, virtual services, and gateway bindings
3. Check mTLS policy alignment and certificate state
4. Correlate 4xx/5xx spikes with route and policy changes

## Output Contract
- Failing mesh component or policy
- Evidence from config/state/traffic signals
- Safe remediation plan and validation checks`,
	},
	{
		Name:        "linkerd_mesh_diagnose",
		Title:       "Linkerd Service Mesh Diagnosis",
		Description: "Diagnose Linkerd control-plane, proxy, and policy health",
		Arguments: []sdkmcp.PromptArgument{
			{Name: "namespace", Description: "Target namespace", Required: false},
			{Name: "workload", Description: "Optional workload name", Required: false},
		},
		Template: `# Linkerd Mesh Diagnosis

Namespace: {{namespace|all namespaces}}
Workload: {{workload|all workloads}}

## Investigation Path
1. Validate Linkerd control-plane components and CRDs
2. Verify proxy injection and data-plane connectivity
3. Check policy/TLS identity status and traffic failures
4. Correlate retries/timeouts with service behavior

## Output Contract
- Root issue location (control-plane/data-plane/policy)
- Supporting evidence
- Minimal-risk recovery actions`,
	},
	{
		Name:        "helm_release_recovery",
		Title:       "Helm Release Recovery",
		Description: "Recover failed Helm installs/upgrades with safe rollback strategy",
		Arguments: []sdkmcp.PromptArgument{
			{Name: "release", Description: "Helm release name", Required: true},
			{Name: "namespace", Description: "Release namespace", Required: false},
		},
		Template: `# Helm Release Recovery: {{release}}

Namespace: {{namespace|default}}

## Recovery Workflow
1. Inspect release status/history and failed hooks
2. Compare values/manifests against last healthy revision
3. Identify immutable field, policy, or dependency blockers
4. Choose forward-fix vs rollback based on risk

## Output Contract
- Failure cause with evidence
- Recommended rollback or patch plan
- Verification steps after remediation`,
	},
	{
		Name:        "terraform_drift_triage",
		Title:       "Terraform Drift Triage",
		Description: "Investigate Terraform drift and plan safety",
		Arguments: []sdkmcp.PromptArgument{
			{Name: "workspace", Description: "Terraform workspace/env", Required: false},
			{Name: "scope", Description: "module or stack scope", Required: false},
		},
		Template: `# Terraform Drift Triage

Workspace: {{workspace|default}}
Scope: {{scope|full stack}}

## Triage Flow
1. Collect plan diff and classify drift (expected/unexpected)
2. Identify high-risk changes (delete/replace/network/security)
3. Separate provider noise from real infrastructure drift
4. Propose staged remediation with rollback path

## Output Contract
- Drift summary by severity
- Unsafe plan actions
- Recommended apply strategy`,
	},
	{
		Name:        "aws_eks_operational_check",
		Title:       "AWS EKS Operational Check",
		Description: "EKS health, nodegroup, and IAM integration diagnostics",
		Arguments: []sdkmcp.PromptArgument{
			{Name: "cluster_name", Description: "EKS cluster name", Required: true},
			{Name: "region", Description: "AWS region", Required: false},
		},
		Template: `# AWS EKS Operational Check: {{cluster_name}}

Region: {{region|current configured region}}

## Verification Flow
1. Validate control-plane status and addon health
2. Check managed nodegroup capacity and upgrade state
3. Verify IAM/OIDC/IRSA dependencies for workloads
4. Correlate cluster events with AWS-side failures

## Output Contract
- Current operational status
- Blocking AWS/Kubernetes integration issues
- Prioritized remediation plan`,
	},
	{
		Name:        "karpenter_capacity_debug",
		Title:       "Karpenter Capacity Debug",
		Description: "Debug provisioning and scheduling issues in Karpenter clusters",
		Arguments: []sdkmcp.PromptArgument{
			{Name: "namespace", Description: "Workload namespace", Required: false},
			{Name: "workload", Description: "Optional workload name", Required: false},
		},
		Template: `# Karpenter Capacity Debug

Namespace: {{namespace|all namespaces}}
Workload: {{workload|all pending workloads}}

## Investigation Flow
1. Identify unschedulable pods and exact scheduling constraints
2. Inspect NodePool/NodeClass and capacity type requirements
3. Validate cloud quota, subnet/SG, and instance availability constraints
4. Check disruption/consolidation policies affecting stability

## Output Contract
- Capacity bottleneck root cause
- Evidence (events/config constraints)
- Fastest safe mitigation and long-term fix`,
	},
	{
		Name:        "gcp_workload_diagnose",
		Title:       "GCP Workload Diagnose",
		Description: "Triage a Kubernetes workload using GCP Cloud Monitoring metrics and Cloud Logging signals. Works for any cluster (GKE, EKS, AKS) shipping telemetry to a GCP project.",
		Arguments: []sdkmcp.PromptArgument{
			{Name: "namespace", Description: "Workload namespace", Required: true},
			{Name: "workload", Description: "Workload (Deployment / StatefulSet / DaemonSet) name", Required: true},
			{Name: "project_id", Description: "GCP observability project ID (falls back to GOOGLE_CLOUD_PROJECT)", Required: false},
			{Name: "duration", Description: "Lookback window (e.g. 30m, 1h)", Required: false},
		},
		Template: `# GCP Workload Diagnose: {{namespace}}/{{workload}}

Project: {{project_id|GOOGLE_CLOUD_PROJECT env}}
Window: {{duration|30m}}

## Investigation Flow
1. Build evidence with rootcause.incident_bundle (namespace, workload set so GCP steps trigger)
2. Inspect gcp.metrics.workload for CPU/memory/restart anomalies in the window
3. Run gcp.logs.error_timeline to find the inflection point (bucketSize 1m for tight windows)
4. Pull correlated logs via gcp.logs.correlated_with_bundle using the bundle's time range
5. Cross-reference k8s events and helm releases from the bundle for change correlation
6. If SLOs exist, list them with gcp.metrics.slo_list and check current goals against observed metrics

## Output Contract
- Time-aligned summary (k8s events vs GCP error spike vs deploy/release)
- Identified inflection point with bucket evidence
- Root-cause hypothesis with metric + log evidence references
- Remediation actions and validation checks`,
	},
}

func RegisterSDKPrompts(server *sdkmcp.Server, ctx ToolContext) ([]string, error) {
	if server == nil {
		return nil, fmt.Errorf("server is required")
	}
	specs, err := loadPromptSpecs(ctx)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(specs))
	for _, p := range specs {
		spec := p
		sdkArgs := make([]*sdkmcp.PromptArgument, 0, len(spec.Arguments))
		for _, a := range spec.Arguments {
			arg := a
			sdkArgs = append(sdkArgs, &arg)
		}
		server.AddPrompt(&sdkmcp.Prompt{Name: spec.Name, Title: spec.Title, Description: spec.Description, Arguments: sdkArgs}, buildPromptHandler(spec))
		names = append(names, spec.Name)
	}
	return names, nil
}

func loadPromptSpecs(ctx ToolContext) ([]promptSpec, error) {
	merged := map[string]promptSpec{}
	order := make([]string, 0, len(builtinPrompts))
	for _, spec := range builtinPrompts {
		order = append(order, spec.Name)
		merged[spec.Name] = spec
	}

	// Directory scan first (recommended layout, one prompt per file).
	dir := resolvePromptConfigDir(ctx)
	if dir != "" {
		dirSpecs, warnings, err := loadPromptSpecsFromDir(dir)
		if err != nil {
			return nil, err
		}
		for _, w := range warnings {
			fmt.Fprintf(os.Stderr, "rootcause: prompt load warning: %s\n", w)
		}
		for _, spec := range dirSpecs {
			if _, exists := merged[spec.Name]; !exists {
				order = append(order, spec.Name)
			}
			merged[spec.Name] = spec
		}
	}

	// Legacy single-file path (still supported; merges on top of dir results).
	path := resolvePromptConfigPath(ctx)
	if path != "" {
		fileSpecs, err := loadPromptSpecsFromTOML(path)
		if err != nil {
			return nil, err
		}
		for _, spec := range fileSpecs {
			if _, exists := merged[spec.Name]; !exists {
				order = append(order, spec.Name)
			}
			merged[spec.Name] = spec
		}
	}

	return promptSpecsInOrder(merged, order), nil
}

func promptSpecsInOrder(specs map[string]promptSpec, order []string) []promptSpec {
	out := make([]promptSpec, 0, len(order))
	for _, name := range order {
		spec, ok := specs[name]
		if ok {
			out = append(out, spec)
		}
	}
	return out
}

// resolvePromptConfigDir returns the first existing prompts directory. Search
// order: ROOTCAUSE_PROMPTS_DIR env var, [prompts].dir from config, then the
// defaultPromptDirs list. Returns "" when none exists.
func resolvePromptConfigDir(ctx ToolContext) string {
	if env := strings.TrimSpace(os.Getenv("ROOTCAUSE_PROMPTS_DIR")); env != "" {
		expanded := expandPromptPath(env)
		if dirExists(expanded) {
			return expanded
		}
	}
	if ctx.Config != nil {
		cfgDir := strings.TrimSpace(ctx.Config.Prompts.Dir)
		if cfgDir != "" {
			expanded := expandPromptPath(cfgDir)
			if dirExists(expanded) {
				return expanded
			}
		}
	}
	for _, p := range defaultPromptDirs {
		expanded := expandPromptPath(p)
		if dirExists(expanded) {
			return expanded
		}
	}
	return ""
}

// loadPromptSpecsFromDir scans dir for prompt files (alphabetical order for
// determinism). *.md files are parsed as markdown with YAML front-matter (one
// prompt per file). *.toml files are parsed via the legacy [[prompt]] format.
// Other files are ignored. Hidden files (leading dot) are skipped.
// A single malformed file never aborts the load: it is skipped and a warning
// is returned so the server still boots with the valid prompts. The returned
// error is non-nil only when the directory itself cannot be read.
func loadPromptSpecsFromDir(dir string) ([]promptSpec, []string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, nil, fmt.Errorf("read prompts dir %s: %w", dir, err)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })

	merged := map[string]promptSpec{}
	order := make([]string, 0, len(entries))
	var warnings []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		full := filepath.Join(dir, name)
		var specs []promptSpec
		switch strings.ToLower(filepath.Ext(name)) {
		case ".md":
			spec, err := loadPromptSpecFromMarkdown(full)
			if err != nil {
				warnings = append(warnings, fmt.Sprintf("skipped prompt file %s: %v", name, err))
				continue
			}
			specs = []promptSpec{spec}
		case ".toml":
			loaded, err := loadPromptSpecsFromTOML(full)
			if err != nil {
				warnings = append(warnings, fmt.Sprintf("skipped prompt file %s: %v", name, err))
				continue
			}
			specs = loaded
		default:
			continue
		}
		for _, s := range specs {
			if _, exists := merged[s.Name]; exists {
				warnings = append(warnings, fmt.Sprintf("duplicate prompt name %q in %s overrides an earlier definition", s.Name, name))
			} else {
				order = append(order, s.Name)
			}
			merged[s.Name] = s
		}
	}
	out := make([]promptSpec, 0, len(order))
	for _, n := range order {
		out = append(out, merged[n])
	}
	return out, warnings, nil
}

// promptMarkdownFront is the YAML front-matter schema for per-file prompts.
type promptMarkdownFront struct {
	Name        string                  `yaml:"name" json:"name"`
	Title       string                  `yaml:"title" json:"title"`
	Description string                  `yaml:"description" json:"description"`
	Arguments   []promptMarkdownArgYAML `yaml:"arguments" json:"arguments"`
}

type promptMarkdownArgYAML struct {
	Name        string `yaml:"name" json:"name"`
	Description string `yaml:"description" json:"description"`
	Required    bool   `yaml:"required" json:"required"`
}

// loadPromptSpecFromMarkdown parses one .md file as `--- YAML front-matter ---`
// followed by the template body. The body is used verbatim as the prompt's
// template (after trimming surrounding whitespace).
func loadPromptSpecFromMarkdown(path string) (promptSpec, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return promptSpec{}, fmt.Errorf("read %s: %w", path, err)
	}
	front, body, err := splitFrontMatter(data)
	if err != nil {
		return promptSpec{}, fmt.Errorf("parse front-matter in %s: %w", path, err)
	}
	if len(front) == 0 {
		// Require YAML front-matter so stray markdown files (READMEs, notes)
		// dropped into the prompts directory are not silently registered as
		// prompts.
		return promptSpec{}, fmt.Errorf("%s has no YAML front-matter; prompt files must start with a `---` block", path)
	}
	var meta promptMarkdownFront
	if err := yaml.Unmarshal(front, &meta); err != nil {
		return promptSpec{}, fmt.Errorf("decode front-matter in %s: %w", path, err)
	}
	name := strings.TrimSpace(meta.Name)
	if name == "" {
		// Fallback to filename without extension, with dashes converted to underscores.
		base := filepath.Base(path)
		ext := filepath.Ext(base)
		name = strings.ReplaceAll(strings.TrimSuffix(base, ext), "-", "_")
	}
	template := strings.TrimSpace(string(body))
	if template == "" {
		return promptSpec{}, fmt.Errorf("prompt %s in %s has empty body template", name, path)
	}
	args := make([]sdkmcp.PromptArgument, 0, len(meta.Arguments))
	for _, a := range meta.Arguments {
		argName := strings.TrimSpace(a.Name)
		if argName == "" {
			return promptSpec{}, fmt.Errorf("invalid argument in %s: name is required", path)
		}
		args = append(args, sdkmcp.PromptArgument{
			Name:        argName,
			Description: strings.TrimSpace(a.Description),
			Required:    a.Required,
		})
	}
	return promptSpec{
		Name:        name,
		Title:       strings.TrimSpace(meta.Title),
		Description: strings.TrimSpace(meta.Description),
		Arguments:   args,
		Template:    template,
	}, nil
}

// splitFrontMatter separates a leading `--- ... ---` YAML block from the body.
// When no front-matter is present, returns (nil, raw, nil). The closing `---`
// must appear on its own line.
func splitFrontMatter(data []byte) (front, body []byte, err error) {
	// Strip UTF-8 BOM if present.
	if len(data) >= 3 && data[0] == 0xEF && data[1] == 0xBB && data[2] == 0xBF {
		data = data[3:]
	}
	s := string(data)
	trimmed := strings.TrimLeft(s, " \t\r\n")
	if !strings.HasPrefix(trimmed, "---") {
		return nil, data, nil
	}
	// Position right after the opening "---" line.
	openLineEnd := strings.IndexByte(trimmed[3:], '\n')
	if openLineEnd < 0 {
		return nil, nil, fmt.Errorf("unterminated front-matter")
	}
	rest := trimmed[3+openLineEnd+1:]
	closeIdx := strings.Index(rest, "\n---")
	if closeIdx < 0 {
		return nil, nil, fmt.Errorf("front-matter missing closing ---")
	}
	frontText := rest[:closeIdx]
	after := rest[closeIdx+len("\n---"):]
	if nl := strings.IndexByte(after, '\n'); nl >= 0 {
		after = after[nl+1:]
	} else {
		after = ""
	}
	// Strip leading blank lines so callers see the body content directly.
	after = strings.TrimLeft(after, "\n\r")
	return []byte(frontText), []byte(after), nil
}

func dirExists(path string) bool {
	if path == "" {
		return false
	}
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}

func resolvePromptConfigPath(ctx ToolContext) string {
	if env := strings.TrimSpace(os.Getenv("MCP_PROMPTS_FILE")); env != "" {
		expanded := expandPromptPath(env)
		if fileExists(expanded) {
			return expanded
		}
	}
	if env := strings.TrimSpace(os.Getenv("ROOTCAUSE_PROMPTS_FILE")); env != "" {
		expanded := expandPromptPath(env)
		if fileExists(expanded) {
			return expanded
		}
	}
	if ctx.Config != nil {
		cfgPath := strings.TrimSpace(ctx.Config.Prompts.File)
		if cfgPath != "" {
			expanded := expandPromptPath(cfgPath)
			if fileExists(expanded) {
				return expanded
			}
		}
	}
	for _, path := range defaultPromptConfigPaths {
		expanded := expandPromptPath(path)
		if fileExists(expanded) {
			return expanded
		}
	}
	return ""
}

func expandPromptPath(path string) string {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return ""
	}
	if strings.HasPrefix(trimmed, "~/") {
		home, err := os.UserHomeDir()
		if err == nil && home != "" {
			return filepath.Join(home, strings.TrimPrefix(trimmed, "~/"))
		}
	}
	return trimmed
}

func fileExists(path string) bool {
	if path == "" {
		return false
	}
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}

func loadPromptSpecsFromTOML(path string) ([]promptSpec, error) {
	var cfg promptFileConfig
	if _, err := toml.DecodeFile(path, &cfg); err != nil {
		return nil, fmt.Errorf("decode prompt config %s: %w", path, err)
	}
	out := make([]promptSpec, 0, len(cfg.Prompts))
	for i := range cfg.Prompts {
		item := cfg.Prompts[i]
		name := strings.TrimSpace(item.Name)
		template := strings.TrimSpace(item.Template)
		if name == "" || template == "" {
			return nil, fmt.Errorf("invalid prompt in %s: name and template are required", path)
		}
		fileArgs := item.Arguments
		if len(fileArgs) == 0 && len(item.Argument) > 0 {
			fileArgs = item.Argument
		}
		args := make([]sdkmcp.PromptArgument, 0, len(fileArgs))
		for _, arg := range fileArgs {
			argName := strings.TrimSpace(arg.Name)
			if argName == "" {
				return nil, fmt.Errorf("invalid prompt %s in %s: argument name is required", name, path)
			}
			args = append(args, sdkmcp.PromptArgument{
				Name:        argName,
				Description: strings.TrimSpace(arg.Description),
				Required:    arg.Required,
			})
		}
		out = append(out, promptSpec{
			Name:        name,
			Title:       strings.TrimSpace(item.Title),
			Description: strings.TrimSpace(item.Description),
			Arguments:   args,
			Template:    template,
		})
	}
	return out, nil
}

func buildPromptHandler(spec promptSpec) sdkmcp.PromptHandler {
	return func(_ context.Context, req *sdkmcp.GetPromptRequest) (*sdkmcp.GetPromptResult, error) {
		args := map[string]string{}
		if req != nil && req.Params != nil && req.Params.Arguments != nil {
			for k, v := range req.Params.Arguments {
				args[k] = v
			}
		}
		text := renderPromptTemplate(spec.Template, args)
		return &sdkmcp.GetPromptResult{
			Description: spec.Description,
			Messages: []*sdkmcp.PromptMessage{{
				Role:    sdkmcp.Role("user"),
				Content: &sdkmcp.TextContent{Text: text},
			}},
		}, nil
	}
}

// PromptSpec is the exported view of a registered prompt used by external
// consumers (e.g. the sync-commands CLI). The struct mirrors the internal
// promptSpec but uses primitive types to avoid leaking the MCP SDK type.
type PromptSpec struct {
	Name        string
	Title       string
	Description string
	Arguments   []PromptArgument
	Template    string
}

// PromptArgument is the exported argument metadata for a PromptSpec.
type PromptArgument struct {
	Name        string
	Description string
	Required    bool
}

// BuiltinPromptSpecs returns a deep copy of the server's built-in prompt
// catalog. Callers can read but not mutate the live list.
func BuiltinPromptSpecs() []PromptSpec {
	out := make([]PromptSpec, 0, len(builtinPrompts))
	for _, p := range builtinPrompts {
		out = append(out, toExportedPromptSpec(p))
	}
	return out
}

// LoadPromptSpecsForCLI returns built-in plus any prompts discovered via the
// usual config path resolution (MCP_PROMPTS_FILE env, [prompts].file, the
// default search paths). Used by sync-commands so custom prompts also generate
// slash-command files.
func LoadPromptSpecsForCLI(ctx ToolContext) ([]PromptSpec, error) {
	specs, err := loadPromptSpecs(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]PromptSpec, 0, len(specs))
	for _, p := range specs {
		out = append(out, toExportedPromptSpec(p))
	}
	return out, nil
}

func toExportedPromptSpec(p promptSpec) PromptSpec {
	args := make([]PromptArgument, 0, len(p.Arguments))
	for _, a := range p.Arguments {
		args = append(args, PromptArgument{Name: a.Name, Description: a.Description, Required: a.Required})
	}
	return PromptSpec{
		Name:        p.Name,
		Title:       p.Title,
		Description: p.Description,
		Arguments:   args,
		Template:    p.Template,
	}
}

func renderPromptTemplate(template string, args map[string]string) string {
	out := template
	for {
		start := strings.Index(out, "{{")
		if start < 0 {
			break
		}
		end := strings.Index(out[start+2:], "}}")
		if end < 0 {
			break
		}
		end += start + 2
		token := strings.TrimSpace(out[start+2 : end])
		repl := ""
		parts := strings.SplitN(token, "|", 2)
		key := strings.TrimSpace(parts[0])
		if v, ok := args[key]; ok && strings.TrimSpace(v) != "" {
			repl = v
		} else if len(parts) == 2 {
			repl = strings.TrimSpace(parts[1])
		}
		out = out[:start] + repl + out[end+2:]
	}
	return out
}

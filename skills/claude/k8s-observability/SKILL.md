---
category: Cloud Observability
description: Vendor-neutral metrics + logs triage for Kubernetes workloads. Works with whichever backend RootCause is configured to use (GCP Stackdriver today; Prometheus / CloudWatch / Datadog as backends land).
tags: [observability, incident, rootcause]
---

# Skill: k8s-observability

Metrics and logs triage for Kubernetes incidents using RootCause's vendor-neutral `observability.*` tools.

## How this is decoupled from any specific vendor

RootCause's observability tools delegate to a configured **backend** (see `observability.gcp.*` in `config.yaml` for the GCP Stackdriver backend; future `observability.prometheus.*` etc. will plug in as siblings). Tool inputs and outputs are the same regardless of which backend serves them. Skill rules below apply to all backends.

The backend identifier appears in every tool response under the `"backend"` field — cite it in postmortems so reviewers know which provider's data was consulted.

## Purpose

Use this skill for:
- triaging workload health with CPU / memory / restart-count metrics,
- pulling errors and warnings for a workload,
- finding the inflection point of an incident via bucketed error timelines,
- correlating logs with a `rootcause.incident_bundle` event window,
- discovering metric descriptors and SLO configuration in the backend.

## Strict Tooling Contract

Use only these observability tool names:
- `observability.metrics.query`
- `observability.metrics.workload`
- `observability.metrics.list_descriptors`
- `observability.metrics.slo_list`
- `observability.logs.query`
- `observability.logs.workload`
- `observability.logs.error_timeline`
- `observability.logs.correlated_with_bundle`

Pair with these RootCause tools for evidence and correlation:
- `rootcause.incident_bundle` (pass both `namespace` and `workload` so observability steps trigger automatically)
- `rootcause.change_timeline`
- `rootcause.rca_generate`

## Triggers

Enable when user intent includes:
- "diagnose workload using observability data",
- "show me errors for service",
- "what's the error rate trend",
- "find the inflection point",
- "correlate logs with the incident timeline",
- "what SLOs do we have",
- "list available metrics".

## Workflow

1. **Confirm backend.** If the active backend is GCP, confirm `observability.gcp.project` is set (or that `GOOGLE_CLOUD_PROJECT` env supplies it). Do not infer from kubeconfig — observability config is intentionally decoupled from cluster identity.
2. **Build evidence.** Call `rootcause.incident_bundle` with `namespace` + `workload`. This auto-triggers `observability.metrics.workload` and `observability.logs.workload` when the observability toolset is enabled.
3. **Find the inflection point.** Call `observability.logs.error_timeline` with the same namespace + workload. Use `bucketSize: 1m` for narrow incidents (≤15m), `5m` for normal, `15m` for multi-hour. For non-GKE clusters where logs come from a different monitored resource type, pass `resourceType` (e.g. `generic_node`).
4. **Pull correlated logs.** Call `observability.logs.correlated_with_bundle` with the bundle from step 2 to get the exact log entries inside the bundle's event window.
5. **SLO context.** If the team has SLOs, call `observability.metrics.slo_list` to surface goal / period. Live burn-rate is out of scope — use `observability.metrics.query` with a backend-native burn-rate query when needed.
6. **Discovery.** When a metric type is unfamiliar, call `observability.metrics.list_descriptors` with the backend's filter syntax to enumerate available signals.

## Output Contract

- Time-aligned summary: k8s events vs metric anomalies vs error-timeline buckets.
- Identified inflection point with bucket evidence.
- Root-cause hypothesis citing specific metric + log entries with timestamps.
- The backend identifier (from the `backend` field in tool responses).
- Remediation actions and validation checks.

## Safety

All observability tools are read-only. They never mutate cloud resources or workloads. Log entries may contain PII or secrets — rely on the redactor pipeline and avoid echoing raw payloads in postmortems without review.

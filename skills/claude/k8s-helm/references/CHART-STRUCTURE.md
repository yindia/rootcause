# Helm Chart Structure Reference

This document is a practical chart-authoring guide for the `k8s-helm` skill.

Use this reference when building or reviewing Helm charts that will be deployed with RootCause tools.

Related docs:
- `skills/claude/k8s-helm/SKILL.md`
- `skills/claude/k8s-helm/TROUBLESHOOTING.md`

## Standard Chart Layout Diagram

```text
my-chart/
  Chart.yaml
  values.yaml
  values.schema.json
  charts/
    dependency-a-1.2.3.tgz
  templates/
    _helpers.tpl
    NOTES.txt
    deployment.yaml
    service.yaml
    ingress.yaml
    serviceaccount.yaml
    role.yaml
    rolebinding.yaml
    configmap.yaml
    secret.yaml
    hpa.yaml
    pdb.yaml
    networkpolicy.yaml
    tests/
      test-connection.yaml
```

Minimum recommended files:
- `Chart.yaml`
- `values.yaml`
- `templates/_helpers.tpl`
- one workload template (`deployment.yaml` or `statefulset.yaml`)
- one service template when app exposes network traffic
- `templates/NOTES.txt`

Strongly recommended:
- `values.schema.json` for values validation,
- `templates/tests/test-connection.yaml` for smoke validation,
- `pdb.yaml` and `hpa.yaml` for production resiliency.

## Chart.yaml Best Practices

### Required Fields

- `apiVersion: v2`
- `name`
- `description`
- `type` (`application` or `library`)
- `version` (chart semantic version)
- `appVersion` (application version)

### Example

```yaml
apiVersion: v2
name: payment-api
description: Helm chart for payment API service
type: application
version: 0.8.4
appVersion: "1.21.0"
keywords:
  - payments
  - api
home: https://example.internal/platform/payment-api
sources:
  - https://github.com/example/payment-api
maintainers:
  - name: platform-sre
```

### Versioning Rules

- bump `version` whenever chart content changes,
- bump `appVersion` when app artifact version changes,
- avoid mixing chart and app semver semantics,
- publish deterministic version history.

### Dependency Management

- declare dependencies explicitly in `Chart.yaml`,
- pin dependency version ranges intentionally,
- avoid broad ranges like `*` in production charts,
- run dependency updates as explicit release events.

### Metadata Quality

- use accurate `description`,
- include `sources` and `maintainers`,
- add `annotations` for ownership and compliance.

## values.yaml Patterns

### Principles

- keep defaults safe and production-friendly,
- separate functional values from environment-specific values,
- avoid deeply nested values without clear template usage,
- document each top-level value section.

### Suggested Top-Level Layout

```yaml
nameOverride: ""
fullnameOverride: ""

image:
  repository: ghcr.io/example/payment-api
  tag: "1.21.0"
  pullPolicy: IfNotPresent

replicaCount: 2

serviceAccount:
  create: true
  name: ""

service:
  type: ClusterIP
  port: 80
  targetPort: 8080

ingress:
  enabled: false
  className: ""
  annotations: {}
  hosts: []
  tls: []

resources:
  requests:
    cpu: 200m
    memory: 256Mi
  limits:
    cpu: 500m
    memory: 512Mi

autoscaling:
  enabled: false
  minReplicas: 2
  maxReplicas: 6
  targetCPUUtilizationPercentage: 75

podDisruptionBudget:
  enabled: true
  minAvailable: 1

env:
  LOG_LEVEL: info

nodeSelector: {}
tolerations: []
affinity: {}
```

### Value Naming Conventions

- use camelCase for key names,
- keep boolean keys explicit (`enabled`, `create`),
- avoid ambiguous keys like `config1`, `misc`, `other`.

### Avoiding Common Pitfalls

- do not hardcode namespaces in values,
- do not duplicate `image.tag` under multiple keys,
- avoid hidden defaults in templates that bypass values.

### values.schema.json Recommendations

- enforce required values for critical paths,
- define type constraints for ports, booleans, and lists,
- provide enum restrictions where practical,
- prevent null/empty values for required identifiers.

## _helpers.tpl Common Templates

`_helpers.tpl` centralizes naming and labels.

### Typical Helpers

```tpl
{{- define "payment-api.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "payment-api.fullname" -}}
{{- if .Values.fullnameOverride -}}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- printf "%s-%s" .Release.Name (include "payment-api.name" .) | trunc 63 | trimSuffix "-" -}}
{{- end -}}
{{- end -}}

{{- define "payment-api.labels" -}}
app.kubernetes.io/name: {{ include "payment-api.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
helm.sh/chart: {{ printf "%s-%s" .Chart.Name .Chart.Version | quote }}
{{- end -}}

{{- define "payment-api.selectorLabels" -}}
app.kubernetes.io/name: {{ include "payment-api.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end -}}
```

### Helper Best Practices

- keep selectors stable across chart versions,
- avoid embedding random suffixes in selector labels,
- centralize all labels in helper templates,
- use helper-generated names for all resources.

## Environment-Specific Values Pattern

Use layered values files by environment.

Recommended file layout:

```text
values/
  common.yaml
  dev.yaml
  staging.yaml
  prod.yaml
```

Pattern:
- `common.yaml` for shared defaults,
- env file for environment overrides only,
- avoid duplicating entire values tree per environment.

Example approach:
1. Keep image repository, service port, labels in `common.yaml`.
2. Keep replica counts, resource sizes, ingress hosts per env file.
3. Keep secrets externalized; reference secret names, not secret literals.

Operational guidance:
- always preview with `helm.diff_release` before mutation,
- use explicit file ordering for deterministic overrides,
- verify effective state with `k8s.get` and `k8s.describe`.

## NOTES.txt Template

`NOTES.txt` should provide concise post-install usage instructions.

Goals:
- show service endpoint strategy,
- show next verification steps,
- avoid stale hardcoded commands.

Example:

```tpl
{{- if .Values.ingress.enabled }}
Application is available via ingress:
{{- range .Values.ingress.hosts }}
  https://{{ .host }}
{{- end }}
{{- else }}
Application is internal. Use service:
  {{ include "payment-api.fullname" . }}:{{ .Values.service.port }}
{{- end }}

Release: {{ .Release.Name }}
Namespace: {{ .Release.Namespace }}
Chart: {{ .Chart.Name }} {{ .Chart.Version }}
```

## Test Template

Use a chart test pod for basic connectivity smoke checks.

Example `templates/tests/test-connection.yaml`:

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: "{{ include \"payment-api.fullname\" . }}-test-connection"
  labels:
    {{- include "payment-api.labels" . | nindent 4 }}
  annotations:
    "helm.sh/hook": test
spec:
  restartPolicy: Never
  containers:
    - name: wget
      image: busybox:1.36
      command: ['sh', '-c']
      args:
        - wget -qO- http://{{ include "payment-api.fullname" . }}:{{ .Values.service.port }}/healthz
```

Test template guidance:
- keep tests deterministic and fast,
- avoid external dependencies in smoke tests,
- test service reachability or health endpoint.

## Validation Checklist

Before publishing chart changes:

1. Structure and metadata
- `Chart.yaml` fields complete and accurate.
- semantic version updated appropriately.

2. Values quality
- `values.yaml` defaults are safe.
- `values.schema.json` enforces critical constraints.

3. Template hygiene
- helpers centralized in `_helpers.tpl`.
- selectors and labels are stable.
- resource names are deterministic.

4. Environment strategy
- `common` and env override files are minimal and clear.
- no secret literals stored in values files.

5. Deployment safety review
- run `helm.diff_release` before install/upgrade.
- verify expected resources and immutable field safety.

6. Post-render and runtime checks
- validate release with `helm.status`.
- verify resources with `k8s.list`, `k8s.get`, `k8s.describe`.
- inspect warnings via `k8s.events` or `k8s.events_timeline`.

7. Recovery readiness
- ensure rollback path can be evaluated with `helm.rollback_advisor`.

## Authoring Anti-Patterns

Avoid these common chart problems:

- mutable selector labels,
- hardcoded names that collide across releases,
- per-environment copy-paste charts,
- hidden template defaults that ignore values,
- undocumented required values,
- templates that assume cluster-admin permissions.

## Review Questions for PRs

- Does this change preserve selector immutability?
- Are names and labels release-scoped and deterministic?
- Are values documented and schema-validated?
- Is the upgrade path safe according to `helm.diff_release`?
- Is rollback target reasoning clear via `helm.rollback_advisor`?
- Are post-deploy checks defined using `helm.status` and `k8s.*` evidence tools?

## Quick Mapping: Chart Concern to RootCause Tool

| Concern | Primary Tool | Supporting Tool |
|---|---|---|
| chart version selection | `helm.get_chart` | `helm.search_charts` |
| repo source validation | `helm.repo_list` | `helm.repo_update` |
| pre-mutation risk preview | `helm.diff_release` | `helm.status` |
| runtime release health | `helm.status` | `k8s.events_timeline` |
| workload failure details | `k8s.describe` | `k8s.events` |
| object-level truth | `k8s.get` | `k8s.list` |
| rollback planning | `helm.rollback_advisor` | `helm.diff_release` |

## Final Notes

Good chart structure reduces release risk.
Strong values discipline and stable selectors prevent most upgrade incidents.
Always pair chart authoring with release-diff review and runtime evidence checks.

# Skill: k8s-policy

Comprehensive Kyverno policy diagnostics for admission failures, policy report analysis,
and enforcement posture validation.

## Trigger Phrases

Use this skill when the user mentions:
- kyverno denied request
- policy violation
- admission webhook block
- require labels policy
- require resource limits policy
- block privileged container
- policy report failed
- audit vs enforce behavior

## RootCause Tools Allowed

Only use these tool names in this skill:
- `k8s.kyverno_detect`
- `k8s.diagnose_kyverno`
- `k8s.describe`
- `k8s.list`
- `k8s.events`

## Kyverno Workflow Overview

Canonical path:
1. detect Kyverno,
2. diagnose policy and controller health,
3. identify failing policy/rule,
4. separate audit findings from enforce blocks,
5. provide remediation with policy intent preserved.

## Policy Types Reference

### Validate policies

Purpose:
- enforce required fields/labels/security constraints.

Examples:
- require `owner` label,
- require memory/cpu requests and limits,
- block privileged containers.

### Mutate policies

Purpose:
- add defaults or normalize fields.

Examples:
- inject standard labels,
- default imagePullPolicy,
- enforce namespace annotations.

### Generate policies

Purpose:
- create dependent resources automatically.

Examples:
- generate network policy baseline,
- generate role bindings for namespaces.

## Audit vs Enforce Modes

Interpretation:
- `Audit`: violations reported, requests allowed.
- `Enforce`: violations block admission.

When incident is active:
- first confirm if block is from `Enforce` policy.
- avoid disabling policy blindly; propose compliant manifest fix first.

## Step-by-Step Investigation

### Step 1: Detect Kyverno installation

Use `k8s.kyverno_detect`.

If not detected:
- stop Kyverno-specific diagnosis,
- report absence clearly.

### Step 2: Run platform diagnosis

Use `k8s.diagnose_kyverno`:
```yaml
namespace: kyverno
limit: 100
```

Collect:
- controller readiness,
- policy readiness,
- report failures,
- warning events.

### Step 3: Inventory policy resources

Use `k8s.list`:
```yaml
resources:
  - kind: ClusterPolicy
  - kind: Policy
  - kind: PolicyReport
  - kind: ClusterPolicyReport
namespace: payments
```

For cluster-scoped policy checks, run without namespace for `ClusterPolicy` resources.

### Step 4: Inspect failing policy and report

Use `k8s.describe` on:
- relevant `ClusterPolicy` or `Policy`,
- referenced `PolicyReport` entry,
- blocked object if needed.

Focus fields:
- `validationFailureAction`
- match/exclude selectors
- rule names and messages
- report result severity and category

### Step 5: Correlate with events

Use `k8s.events` in affected namespace to capture denial entries.

Example:
```yaml
namespace: payments
```

## Common Policy Scenarios

### Require labels

Policy intent:
- all workloads include ownership metadata.

Failure pattern:
- deployment missing required label keys.

### Require limits/requests

Policy intent:
- prevent unbounded pods and scheduling instability.

Failure pattern:
- container missing one of cpu/memory requests or limits.

### Block privileged containers

Policy intent:
- enforce pod security baseline.

Failure pattern:
- `securityContext.privileged: true` or unsafe capabilities.

## YAML Examples

### Validate required labels

```yaml
apiVersion: kyverno.io/v1
kind: ClusterPolicy
metadata:
  name: require-owner-label
spec:
  validationFailureAction: Enforce
  rules:
    - name: check-owner-label
      match:
        any:
          - resources:
              kinds:
                - Deployment
      validate:
        message: "owner label is required"
        pattern:
          metadata:
            labels:
              owner: "?*"
```

### Validate resource limits

```yaml
apiVersion: kyverno.io/v1
kind: ClusterPolicy
metadata:
  name: require-limits
spec:
  validationFailureAction: Audit
  rules:
    - name: require-cpu-memory
      match:
        any:
          - resources:
              kinds:
                - Pod
      validate:
        message: "cpu/memory requests and limits are required"
        pattern:
          spec:
            containers:
              - resources:
                  requests:
                    cpu: "?*"
                    memory: "?*"
                  limits:
                    cpu: "?*"
                    memory: "?*"
```

### Block privileged containers

```yaml
apiVersion: kyverno.io/v1
kind: ClusterPolicy
metadata:
  name: disallow-privileged
spec:
  validationFailureAction: Enforce
  rules:
    - name: no-privileged
      match:
        any:
          - resources:
              kinds:
                - Pod
      validate:
        message: "privileged containers are not allowed"
        pattern:
          spec:
            containers:
              - securityContext:
                  privileged: false
```

## Policy Report Analysis Guidance

When reading PolicyReports:
1. group by policy name,
2. group by rule name,
3. count failed resources,
4. prioritize enforce-mode blockers first.

Key distinction:
- report failures do not always mean blocked admissions,
- enforce events indicate immediate deploy impact.

## Troubleshooting Table

| Symptom | Likely Cause | Confirm With | Fix Direction |
|---|---|---|---|
| deployment blocked at admission | enforce validate rule failed | `k8s.events` + `k8s.describe` policy | patch manifest to satisfy rule |
| expected mutation not applied | mutate rule selectors miss target | `k8s.describe` policy and resource labels | adjust match/exclude selectors |
| many audit violations but no blocks | policy in Audit mode | `k8s.describe` policy action | plan gradual compliance rollout |
| policy not triggering at all | wrong kinds/apiGroups match | `k8s.describe` policy rules | correct match scope |
| kyverno unstable | controller/webhook issues | `k8s.diagnose_kyverno` | restore control-plane health |

## Parameter Guidance

### `k8s.diagnose_kyverno`

- use `namespace` for focused incidents.
- use higher `limit` in noisy clusters.

### `k8s.list`

- include policy and report resources together for context.

### `k8s.describe`

- inspect both policy and failing resource when ambiguity exists.

### `k8s.events`

- admission failures usually surface here earliest.

## Output Contract

Always provide:
1. Kyverno detection status,
2. failing policy and rule names,
3. audit vs enforce impact summary,
4. exact manifest changes needed,
5. validation steps after fix.

Example:
```text
Root cause: ClusterPolicy require-owner-label in Enforce mode blocked Deployment payments-api due missing owner label.
Evidence: k8s.events shows admission deny; k8s.describe ClusterPolicy rule check-owner-label requires metadata.labels.owner.
Fix: add owner label to deployment metadata and pod template labels.
Verify: re-apply workload and confirm no new Kyverno deny events.
```

## Completion Criteria

Policy diagnosis is complete when:
- blocking rule is identified unambiguously,
- audit vs enforce implications are clear,
- compliant remediation is proposed,
- post-fix verification is defined.

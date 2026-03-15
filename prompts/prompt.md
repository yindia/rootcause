# RootCause Prompt Templates (Specific + Copy/Paste Ready)

Use these prompts as-is, then replace values in angle brackets.

## Usage Rules (Recommended)

- Always ask for JSON unless you need markdown (`rootcause.postmortem_export`).
- Keep outputs concise and machine-parseable.
- If a tool returns an error, ask for:
  - `error`
  - probable cause
  - exact next tool call to run

## Global Output Contract (for all prompts)

Append this line to any template for consistent responses:

```text
Return strict JSON only. No prose outside JSON. If data is missing, use null and include a "nextAction" field.
```

## 1) RootCause Workflow

### `rootcause.incident_bundle`
Template:
```text
Objective: Build a compact incident evidence bundle for fast triage.

Run `rootcause.incident_bundle` with:
- namespace: "<namespace>"
- keyword: "<symptom keyword: 5xx|timeout|crashloop|pending|forbidden>"
- eventLimit: <100-300>
- releaseLimit: <10-50>
- includeHelm: true

Return STRICT JSON with this shape:
{
  "sectionCount": number,
  "stepCount": number,
  "errorCount": number,
  "sectionsSummary": [
    {
      "section": string,
      "risk": "high|medium|low",
      "keySignals": [string, ... up to 3]
    }
  ],
  "nextChecks": [string, ... up to 5]
}

Rules:
- Do not include prose outside JSON.
- If a section fails, include it in errorCount and still return partial result.
- Rank sections by operational risk first.
- Do not ask for low-level chain/tool args from the user; build the tool chain automatically from namespace + keyword.
```

Example:
```text
Run `rootcause.incident_bundle` with namespace "payments", keyword "5xx", eventLimit 180, releaseLimit 20, includeHelm true.
Return STRICT JSON only with keys: sectionCount, stepCount, errorCount, sectionsSummary[{section,risk,keySignals<=3}], nextChecks<=5.
No prose outside JSON.
```

Automatic chain example (no low-level tool list from user):
```text
Run `rootcause.incident_bundle` with namespace "payments", keyword "5xx", eventLimit 180, releaseLimit 20, includeHelm true.
MCP should automatically decide and execute internal chained tools needed for this incident.
Return STRICT JSON only with keys: sectionCount, stepCount, errorCount, sectionsSummary, nextChecks.
If any step fails, include failedSteps as [{tool, section, error}].
```

### `rootcause.rca_generate`
Template:
```text
Run `rootcause.rca_generate` with:
- namespace: "<namespace>"
- keyword: "<symptom>"
- incidentSummary: "<1-line impact summary>"

Return JSON with exactly:
- confidence
- rootCauses: [{rank, summary, evidenceHint}]
- recommendations: [max 6]
- missingEvidence: [max 5]
```

### `rootcause.change_timeline`
Template:
```text
Objective: Correlate incident chronology with platform changes.

Prefer single-tool mode via `rootcause.incident_bundle`:
- outputMode: "timeline"

Run `rootcause.incident_bundle` with:
- namespace: "<namespace>"
- keyword: "<symptom keyword>"
- eventLimit: 200
- releaseLimit: 30
- timelineLimit: 200
- outputMode: "timeline"
- includeHelm: true
- includeNormal: false

Return STRICT JSON with:
- timelineCount
- startTime
- endTime
- timeline: [{time, source, severity, summary, resource}]
- errorCount
```

Example:
```text
Run `rootcause.incident_bundle` with namespace "payments", keyword "5xx", eventLimit 200, releaseLimit 30, timelineLimit 200, outputMode "timeline", includeHelm true, includeNormal false. Return strict JSON with timelineCount, startTime, endTime, timeline entries (time, source, severity, summary, resource), and errorCount.
```

Example:
```text
Run `rootcause.rca_generate` with namespace "payments", keyword "5xx", incidentSummary "Payment API returned intermittent 5xx for 12 minutes". Return JSON with confidence, ranked rootCauses (rank, summary, evidenceHint), recommendations (max 6), and missingEvidence (max 5). Return strict JSON only.
```

### `rootcause.remediation_playbook`
Template:
```text
Run `rootcause.remediation_playbook` with:
- namespace: "<namespace>"
- keyword: "<symptom>"
- maxImmediateActions: 3

Return JSON with exactly:
- immediateActions: [{priority, title, owner, toolHints}]
- followUpActions: [{title, owner}]
- validation: [step-by-step checks]
```

Example:
```text
Run `rootcause.remediation_playbook` with namespace "payments", keyword "5xx", maxImmediateActions 3. Return JSON with immediateActions (priority, title, owner, toolHints), followUpActions (title, owner), and validation steps. Return strict JSON only.
```

### `rootcause.postmortem_export`
Template:
```text
Run `rootcause.postmortem_export` with:
- namespace: "<namespace>"
- keyword: "<symptom>"
- incidentSummary: "<short summary>"
- format: "markdown"

Return only markdown content. Include sections:
- Incident Summary
- Timeline
- Root Causes
- Corrective Actions
- Preventive Actions
```

Example:
```text
Run `rootcause.postmortem_export` with namespace "payments", keyword "5xx", incidentSummary "Payment API 5xx spike after chart upgrade", format "markdown". Return only markdown content with Incident Summary, Timeline, Root Causes, Corrective Actions, and Preventive Actions.
```

### End-to-End Chain Prompt (bundle -> rca -> playbook -> postmortem)
```text
Generate in order:
1) `rootcause.incident_bundle` for namespace "payments" keyword "5xx"
2) `rootcause.rca_generate` using that bundle
3) `rootcause.remediation_playbook` using bundle + rca
4) `rootcause.postmortem_export` in markdown

Return a single JSON object with keys: bundleSummary, rcaSummary, playbookSummary, postmortemMarkdown.
```

## 2) Kubernetes Debugging

### `k8s.events_timeline`
Template:
```text
Run `k8s.events_timeline` with:
- namespace: "<namespace>"
- involvedObjectKind: "Pod"
- involvedObjectName: "<optional object name>"
- limit: 200
- includeNormal: false

Return:
1) ordered timeline (oldest->newest)
2) top repeated warning reasons with counts
3) first failure timestamp and latest failure timestamp
```

Example:
```text
Run `k8s.events_timeline` with namespace "payments", involvedObjectKind "Pod", limit 200, includeNormal false. Return ordered timeline, repeated warning reasons with counts, and first/latest failure timestamps.
```

### `k8s.restart_safety_check`
Template:
```text
Run `k8s.restart_safety_check` with:
- namespace: "<namespace>"
- name: "<deployment-name>"
- minReadyReplicas: 2
- maxUnavailableRatio: 0.3

Return:
1) safe (true/false)
2) issues by severity (high/medium/low)
3) PDB/HPA/readiness check outputs
4) clear recommendation: proceed now or delay
```

Example:
```text
Run `k8s.restart_safety_check` with namespace "payments", name "payment-api", minReadyReplicas 2, maxUnavailableRatio 0.3. Return safe flag, issues by severity, PDB/HPA/readiness checks, and proceed-or-delay recommendation.
```

### `k8s.best_practice`
Template:
```text
Run `k8s.best_practice` with:
- kind: "Deployment"  (or DaemonSet/StatefulSet)
- name: "<workload-name>"
- namespace: "<namespace>"

Return STRICT JSON with:
- compliant
- score (0-100)
- summary {high, medium, low, total}
- checks [{id,status,severity,message,recommendation}]
- recommendations

Focus checks on rollout/restart/node-recreate safety:
- topology spread / anti-affinity for replica distribution
- resource requests/limits and probes
- PVC detach/attach risks (termination grace, preStop, shared RWO, StorageClass binding mode, VolumeAttachment errors)
```

Example:
```text
Run `k8s.best_practice` with kind "Deployment", name "payment-api", namespace "payments". Return strict JSON with compliant, score, summary, checks, and recommendations.
```

### `k8s.debug_flow`
Template:
```text
Run `k8s.debug_flow` with:
- namespace: "<namespace>"
- kind: "Service"
- name: "<service-name>"
- scenario: "traffic"

Return:
- likelyRootCauses
- evidence
- recommendedNextChecks
```

Example:
```text
Run `k8s.debug_flow` with namespace "payments", kind "Service", name "payment-api", scenario "traffic". Return likelyRootCauses, evidence, and recommendedNextChecks.
```

## 3) Helm Operations

### `helm.diff_release`
Template:
```text
Run `helm.diff_release` with:
- namespace: "<namespace>"
- release: "<release-name>"
- chart: "<repo/chart>"
- version: "<target-version>"
- includeUnchanged: false

Return:
1) summary {added, removed, changed, unchanged}
2) changed resources with concise current vs desired difference
3) high-risk changes (deletions or major replacements)
4) pre-upgrade checks
```

Example:
```text
Run `helm.diff_release` with namespace "payments", release "payment-api", chart "platform/payment-api", version "1.18.4", includeUnchanged false. Return summary, changed resources, high-risk changes, and pre-upgrade checks.
```

### `helm.rollback_advisor`
Template:
```text
Run `helm.rollback_advisor` with:
- namespace: "<namespace>"
- release: "<release-name>"
- historyLimit: 20

Return:
1) current revision and status
2) top rollback candidates (latest stable first)
3) risk per candidate
4) checks required before rollback
```

Example:
```text
Run `helm.rollback_advisor` with namespace "payments", release "payment-api", historyLimit 20. Return current revision/status, rollback candidates with risk, and required checks before rollback.
```

## 4) Terraform Analysis

### `terraform.debug_plan`
Template:
```text
Run `terraform.debug_plan` with:
- planJSON: "<terraform show -json output>"
- summarizeByProvider: true
- includeNoOp: false

Return:
1) action summary by create/update/delete/replace
2) high-risk resources
3) provider-level breakdown
4) recommended guard checks
```

Example:
```text
Run `terraform.debug_plan` with planJSON "<insert-json>", summarizeByProvider true, includeNoOp false. Return action summary, high-risk resources, provider breakdown, and guard checks.
```

### `terraform.search_modules`
Template:
```text
Run `terraform.search_modules` with query "<query>", provider "<provider>", verified true, limit 10. Return namespace/name/provider, trust signals, and why each result matches.
```

Example:
```text
Run `terraform.search_modules` with query "vpc", provider "aws", verified true, limit 10. Return namespace/name/provider, trust signals, and match rationale.
```

## 5) AWS Triage

### `aws.eks.debug`
Template:
```text
Run `aws.eks.debug` with clusterName "<cluster>", includeSTS true, includeKMS true, includeECR true. Return failures first, probable causes, and specific remediation steps.
```

Example:
```text
Run `aws.eks.debug` with clusterName "prod-eks-01", includeSTS true, includeKMS true, includeECR true. Return failures first, probable causes, and specific remediations.
```

### `aws.iam.list_roles`
Template:
```text
Run `aws.iam.list_roles` with limit 50. Return roles related to EKS/workloads and flag risky trust-policy patterns.
```

Example:
```text
Run `aws.iam.list_roles` with limit 50. Return EKS-related roles and suspicious trust-policy findings.
```

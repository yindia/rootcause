# Skill: rootcause-rca

Full incident RCA pipeline for evidence collection, timeline analysis, root-cause drafting,
remediation planning, and postmortem export.

This skill is for structured incident documentation and decision support,
not for ad-hoc single-command troubleshooting.

## Trigger Phrases

Use this skill when the user asks for:
- root cause analysis
- incident report
- postmortem draft
- what changed before outage
- remediation plan
- corrective and preventive actions
- timeline export

## RootCause Tools Allowed

Only use these tool names in this skill:
- `rootcause.incident_bundle`
- `rootcause.change_timeline`
- `rootcause.rca_generate`
- `rootcause.remediation_playbook`
- `rootcause.postmortem_export`
- `k8s.events_timeline`
- `helm.list`

## RCA Pipeline Overview

Canonical sequence:
1. Bundle
2. Timeline
3. RCA draft
4. Remediation playbook
5. Postmortem export

Shorthand:
`bundle -> timeline -> rca -> remediation -> postmortem`

## Step 1: Incident Bundle

Use `rootcause.incident_bundle` to gather cross-tool evidence.

Recommended minimal parameters:
```yaml
namespace: payments
keyword: "timeout"
includeHelm: true
includeDefaultChain: true
```

Useful optional parameters:
- `eventLimit`: tune for noisy namespaces.
- `maxSteps`: cap diagnostic expansion.
- `outputMode`: `bundle` for full context, `timeline` for condensed view.
- `continueOnError`: keep collecting when one branch fails.

When to run again:
- evidence is too generic,
- namespace was too broad,
- keyword was not specific.

## Step 2: Change Timeline

Use `rootcause.change_timeline` to align cluster events and release changes.

Recommended parameters:
```yaml
namespace: payments
keyword: "timeout"
includeHelm: true
includeNormal: false
timelineLimit: 150
```

Use timeline to answer:
- What changed just before impact?
- Was there a rollout, config update, or restart wave?
- Did warning events precede customer-visible failure?

If Helm activity is expected but absent:
1. run `helm.list` in relevant namespace,
2. confirm release names and history context.

## Step 3: RCA Draft Generation

Use `rootcause.rca_generate` to synthesize likely causes and contributing factors.

Recommended parameters:
```yaml
namespace: payments
keyword: "timeout"
incidentSummary: "Checkout API experienced intermittent 503 and elevated latency for 17 minutes"
```

Guidance:
- include concise incident summary for better narrative quality.
- prefer concrete symptom keywords over generic terms.
- rerun after timeline refinement if initial RCA is vague.

## Step 4: Remediation Playbook

Use `rootcause.remediation_playbook` to produce prioritized actions.

Recommended parameters:
```yaml
namespace: payments
keyword: "timeout"
maxImmediateActions: 5
```

Interpretation model:
- immediate actions: stop or reduce user impact now.
- near-term actions: stabilize and close known reliability gaps.
- long-term actions: systemic prevention and guardrails.

## Step 5: Postmortem Export

Use `rootcause.postmortem_export` when document output is needed.

Markdown export:
```yaml
namespace: payments
keyword: "timeout"
incidentSummary: "Checkout API outage on 2026-03-16"
format: markdown
```

JSON export:
```yaml
namespace: payments
keyword: "timeout"
incidentSummary: "Checkout API outage on 2026-03-16"
format: json
```

Use markdown for human review, JSON for automation pipelines.

## When to Skip Steps

Skip rules:
- Skip `rootcause.incident_bundle` if reliable fresh bundle already exists.
- Skip `rootcause.change_timeline` for non-time-sensitive one-off config mistakes.
- Skip `rootcause.remediation_playbook` only for purely informational retrospectives.
- Do not skip `rootcause.rca_generate` if user asked for root cause statement.

Fast path example:
`rca_generate -> postmortem_export`
only if bundle and timeline were completed recently and context is unchanged.

## Parameter Guidance by Tool

### `rootcause.incident_bundle`

- `namespace`: strongly recommended to reduce noise.
- `keyword`: high-impact symptom token.
- `includeHelm`: true when release correlation matters.
- `includeDefaultChain`: true for standard triage stack.
- `outputMode`: `bundle` for full RCA workflows.

### `rootcause.change_timeline`

- `includeNormal`: false for cleaner incident focus.
- `eventLimit`: raise when high churn expected.
- `timelineLimit`: keep manageable for human analysis.

### `rootcause.rca_generate`

- `incidentSummary`: improves causal narrative quality.
- `keyword`: anchors symptom context.

### `rootcause.remediation_playbook`

- `maxImmediateActions`: keeps priorities executable.

### `rootcause.postmortem_export`

- `format`: choose based on destination system.

### `k8s.events_timeline`

Use to validate sequencing conflicts between generated timeline and raw events.

Example:
```yaml
namespace: payments
includeNormal: false
limit: 200
```

## Output Format Expectations

Generated outputs should include:
- incident metadata (time window, namespace, impacted service)
- probable primary root cause
- contributing factors
- confidence level and evidence references
- immediate and long-term remediation plan
- ownership suggestions and verification checkpoints

## Quality Gates for RCA

An RCA is acceptable only if:
1. root cause references concrete objects/events,
2. evidence links directly to claims,
3. timeline is causally coherent,
4. remediation actions are testable,
5. prevention actions reduce recurrence risk.

## Troubleshooting Pipeline Failures

| Problem | Likely Cause | Mitigation |
|---|---|---|
| Bundle too broad and noisy | namespace/keyword too generic | narrow namespace and keyword, rerun bundle |
| Timeline misses release changes | Helm metadata excluded or release unknown | run `helm.list`, rerun timeline with `includeHelm: true` |
| RCA says "insufficient evidence" | weak incident summary or sparse events | provide richer `incidentSummary`, expand event limit |
| Playbook has vague actions | RCA lacks concrete failure points | improve timeline-to-evidence mapping first |
| Postmortem missing sections | incomplete upstream pipeline context | regenerate bundle + RCA, then export |

## Example End-to-End Call Set

```text
1) rootcause.incident_bundle(namespace=payments, keyword=timeout, includeHelm=true)
2) rootcause.change_timeline(namespace=payments, keyword=timeout, includeHelm=true)
3) rootcause.rca_generate(namespace=payments, keyword=timeout, incidentSummary="Checkout API 503 spike")
4) rootcause.remediation_playbook(namespace=payments, keyword=timeout, maxImmediateActions=5)
5) rootcause.postmortem_export(namespace=payments, keyword=timeout, format=markdown)
```

## Report Contract

Always deliver:
1. one-sentence root cause,
2. 3-7 evidence bullets,
3. concise timeline milestones,
4. immediate mitigation list,
5. prevention plan,
6. export location/format details.

## Example Final Summary Template

```text
Root cause: payments-api Service routed to healthy pods, but ingress backend pointed to stale port after chart upgrade.
Evidence: change timeline shows Helm release at 09:14; events show backend endpoint failures from 09:15; RCA links first customer errors at 09:16.
Immediate actions: rollback backend port config, verify ingress health, monitor error rate.
Long-term actions: add release smoke test for ingress backend port and enforce chart schema validation.
Postmortem: exported in markdown for incident review.
```

## Completion Criteria

RCA workflow is complete only when:
- generated RCA and remediation are evidence-backed,
- timeline consistency has been validated,
- postmortem is exported in requested format,
- action items are prioritized and scoped.

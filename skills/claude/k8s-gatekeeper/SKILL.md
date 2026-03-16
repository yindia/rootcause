# k8s-gatekeeper

Use this skill for Gatekeeper policy diagnostics and constraint health checks.

## Triggers

- "gatekeeper not enforcing"
- "constraint violations"
- "admission denied"
- "constraint template issue"

## Workflow

1. Detect Gatekeeper footprint with `k8s.gatekeeper_detect`.
2. Diagnose health and warnings with `k8s.diagnose_gatekeeper`.
3. Correlate deny events and affected workloads (`k8s.events`, `k8s.describe`).
4. Propose remediation order: template issues, constraint status, then policy rollout.

## Output Contract

- Gatekeeper install/health summary
- Constraint/template findings with evidence
- Safe remediation and verification steps

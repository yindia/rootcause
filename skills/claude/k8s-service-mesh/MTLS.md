# mTLS Operations and Troubleshooting with RootCause Tools

This guide covers mTLS design validation, staged rollout, and failure triage using RootCause service-mesh and Kubernetes tools.

## Scope

- PeerAuthentication modes.
- DestinationRule TLS settings.
- Mesh-wide STRICT migration.
- Namespace and workload exception strategy.
- Handshake failure diagnostics.
- Certificate and identity issue triage.

## RootCause Tools Used

### Istio tools

- `istio.health`
- `istio.config_summary`
- `istio.discover_namespaces`
- `istio.virtualservice_status`
- `istio.destinationrule_status`
- `istio.cr_status`
- `istio.proxy_bootstrap`
- `istio.proxy_clusters`
- `istio.proxy_routes`
- `istio.proxy_endpoints`
- `istio.proxy_config_dump`

### Linkerd tools

- `linkerd.health`
- `linkerd.proxy_status`
- `linkerd.identity_issues`
- `linkerd.policy_debug`
- `linkerd.cr_status`

### Supporting tools

- `k8s.describe`
- `k8s.list`
- `k8s.events`
- `k8s.events_timeline`
- `k8s.config_debug`
- `k8s.network_debug`

## mTLS Concepts That Matter in Incidents

- Control-plane health is a prerequisite for trustworthy policy status.
- Accepted CR status does not guarantee effective proxy behavior.
- Connection-level TLS settings are visible in proxy cluster config.
- Namespace boundaries and sidecar injection affect mTLS outcomes.
- Identity trust roots and SAN expectations can fail independently from routing.

## PeerAuthentication Modes

### PERMISSIVE

- Accepts both mTLS and plaintext.
- Useful during migration and brownfield onboarding.
- Can hide non-meshed callers unless explicitly audited.

### STRICT

- Requires mTLS for inbound workload traffic.
- Best default for production after migration completion.
- Exposes non-meshed callers immediately.

### DISABLE

- Explicitly disables mTLS for selected traffic.
- Should be used only as temporary exception.
- Requires tight documentation and expiration controls.

## DestinationRule TLS Modes

- `ISTIO_MUTUAL`: preferred for in-mesh service-to-service traffic.
- `SIMPLE`/`MUTUAL`: used for external TLS patterns; must align with cert material.
- `DISABLE`: avoid unless justified and tightly scoped.

## Baseline Validation Workflow

1. Run `istio.health` to confirm control plane.
2. Run `istio.discover_namespaces` to identify sidecar coverage.
3. Run `istio.destinationrule_status` and capture policy condition issues.
4. Run `istio.cr_status` for PeerAuthentication resources.
5. Run `istio.proxy_bootstrap` on representative pods.
6. Run `istio.proxy_clusters` to verify transport sockets and TLS context.

## Mesh-Wide STRICT Migration Strategy

### Stage 0: Inventory

1. Run `istio.discover_namespaces`.
2. Run `istio.config_summary` for service density.
3. Run `k8s.list` for workloads lacking sidecars.
4. Run `k8s.events_timeline` for historical auth failures.

### Stage 1: Namespace readiness checks

1. Verify every critical namespace has consistent injection footprint.
2. Verify no critical callers remain outside mesh.
3. Use `istio.proxy_status` for sidecar sync confidence.

### Stage 2: DestinationRule consistency

1. Run `istio.destinationrule_status` for every critical namespace.
2. Confirm DR host/subset correctness.
3. Confirm TLS modes align with mesh policy.

### Stage 3: Controlled STRICT rollout

1. Apply STRICT in least-critical namespace first.
2. Run `k8s.events` and `k8s.events_timeline` for immediate warning spikes.
3. Run `istio.proxy_clusters` to ensure mTLS transport for expected clusters.
4. Expand scope gradually.

### Stage 4: Validation gates

1. No sustained 5xx increase during rollout window.
2. No widespread handshake failures in events.
3. Proxy cluster config shows expected TLS context for service traffic.

## Namespace Exception Strategy

- Prefer shortest-lived exceptions possible.
- Keep exceptions namespace-local or workload-specific.
- Record owner, reason, and expiration.
- Revalidate with `istio.cr_status` and `istio.destinationrule_status` after each exception update.

## Handshake Failure Triage

### Symptom: upstream connect error or disconnect/reset before headers

1. Run `istio.proxy_clusters` on caller pod.
2. Confirm transport socket contains expected TLS context.
3. Run `istio.proxy_endpoints` and ensure endpoint set exists.
4. Run `istio.proxy_bootstrap` to verify trust-domain config.
5. Check warning events with `k8s.events`.

### Symptom: sudden failure after policy update

1. Run `k8s.events_timeline` to identify exact change moment.
2. Run `istio.cr_status` for PeerAuthentication and auth policy conditions.
3. Run `istio.destinationrule_status` for invalid host/subset or TLS mode mismatch.
4. Run `istio.proxy_config_dump` for detailed diff evidence.

### Symptom: failures isolated to one namespace

1. Run `istio.discover_namespaces` and check injection drift.
2. Run `k8s.list` for namespace workloads and labels.
3. Run `istio.proxy_clusters` on representative callers.
4. Run `k8s.config_debug` if cert/config references are workload-mounted.

## Linkerd mTLS and Identity Triage

1. Run `linkerd.health`.
2. Run `linkerd.proxy_status`.
3. Run `linkerd.identity_issues` for cert and issuer failures.
4. Run `linkerd.policy_debug` for policy CRD health.
5. Run `linkerd.cr_status` for impacted policy resources.

## Failure Pattern Catalog

| Pattern | Primary signal | Primary tool | Follow-up |
|---|---|---|---|
| Non-meshed caller blocked after STRICT | namespace injection gap | `istio.discover_namespaces` | `k8s.list` |
| DR TLS mismatch | accepted policy but handshake reset | `istio.destinationrule_status` | `istio.proxy_clusters` |
| Missing endpoint appears as TLS issue | cluster has no healthy endpoints | `istio.proxy_endpoints` | `istio.pods_by_service` |
| Trust-domain inconsistency | cert SAN mismatch | `istio.proxy_bootstrap` | `istio.proxy_config_dump` |
| Linkerd identity outage | cert/issuer health degraded | `linkerd.identity_issues` | `linkerd.health` |

## STRICT Migration Checklist

- Control plane healthy from `istio.health`.
- Namespace injection coverage confirmed.
- DestinationRule TLS modes audited.
- PeerAuthentication hierarchy validated.
- Non-mesh clients identified and remediated.
- Events timeline monitored during each rollout wave.
- Rollback plan documented before each promotion.

## Rollback Triggers

Rollback or pause migration if any condition is true:

- Cross-namespace critical path failure persists beyond retry window.
- Widespread handshake reset signatures across multiple services.
- Identity service instability in Linkerd.
- Gateway ingress path affected in production.

## Verification Commands by Intent

### Validate policy acceptance

- `istio.cr_status`
- `istio.destinationrule_status`

### Validate effective proxy behavior

- `istio.proxy_clusters`
- `istio.proxy_bootstrap`
- `istio.proxy_config_dump`

### Validate incident blast radius

- `k8s.events`
- `k8s.events_timeline`
- `istio.discover_namespaces`

## Reporting Template

When reporting mTLS incidents, include:

1. Mesh type and control-plane status.
2. Policy objects with failing conditions.
3. Effective proxy evidence for TLS transport mismatch.
4. Namespace/workload scope and blast radius.
5. Immediate mitigations and long-term hardening tasks.

## Operational Recommendations

- Keep STRICT as target state for production.
- Use canary namespace waves for policy migration.
- Audit DR host and subset correctness before auth policy changes.
- Tie mTLS rollouts to clear SLO guardrails.
- Use event timelines to separate policy-caused failures from unrelated churn.

## Common Anti-Patterns

- Enabling STRICT mesh-wide before sidecar coverage is complete.
- Assuming accepted CR means effective Envoy config is correct.
- Ignoring destination-level TLS overrides during migration.
- Leaving temporary plaintext exceptions undocumented.
- Diagnosing only one pod and extrapolating to all namespaces.

## Related Guides

- `SKILL.md` for full mesh triage workflows.
- `TRAFFIC-SHIFTING.md` for route and resilience policy debugging.

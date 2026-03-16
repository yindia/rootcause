# Traffic Shifting with Istio RootCause Tools

This guide documents advanced traffic-shifting diagnostics and verification using RootCause Istio tools.

## Scope

- Weight-based routing.
- Header-based routing.
- Cookie-based routing.
- Traffic mirroring.
- Fault injection.
- Timeout and retry policy interactions.

## RootCause Tools Used

- `istio.health`
- `istio.config_summary`
- `istio.service_mesh_hosts`
- `istio.virtualservice_status`
- `istio.destinationrule_status`
- `istio.gateway_status`
- `istio.httproute_status`
- `istio.proxy_listeners`
- `istio.proxy_routes`
- `istio.proxy_clusters`
- `istio.proxy_endpoints`
- `istio.proxy_config_dump`
- `istio.pods_by_service`
- `k8s.describe`
- `k8s.list`
- `k8s.events`
- `k8s.events_timeline`

## Prerequisites

1. Confirm control-plane health with `istio.health`.
2. Confirm relevant namespaces are in mesh scope with `istio.config_summary` and `istio.service_mesh_hosts`.
3. Capture incident start time for timeline correlation using `k8s.events_timeline`.

## Weight-Based Routing

### Goal

Shift traffic gradually between stable and canary versions.

### Diagnostic Sequence

1. Run `istio.virtualservice_status` to confirm route object acceptance.
2. Run `istio.destinationrule_status` to validate subset definitions.
3. Run `istio.proxy_routes` on ingress or caller pod and confirm weight split.
4. Run `istio.proxy_clusters` to verify clusters for both subsets exist.
5. Run `istio.proxy_endpoints` to validate active endpoints per subset.

### Common Failure Modes

- VirtualService accepted but route weights not visible in proxy routes.
- Subset missing in DestinationRule, causing all traffic to default subset.
- Canary subset has no endpoints.
- Weight appears correct, but gateway route host mismatch sends traffic elsewhere.

### Fast Checks

- `istio.virtualservice_status` condition is not False.
- `istio.destinationrule_status` subsets align with workload labels.
- `istio.proxy_routes` shows expected weighted clusters.
- `istio.proxy_endpoints` has healthy addresses for each weighted destination.

## Header-Based Routing

### Goal

Route traffic based on headers such as user segments or debug sessions.

### Diagnostic Sequence

1. Run `istio.virtualservice_status` to verify route resource health.
2. Run `istio.proxy_routes` and inspect match clauses for header keys and values.
3. Run `istio.proxy_listeners` to validate route attachment to correct port and host.
4. Run `istio.proxy_clusters` to ensure target cluster exists.

### Common Failure Modes

- Header match key case mismatch.
- Regex or prefix match not reflected due to route ordering.
- Route exists but attached to different gateway/listener.

### Verification Checklist

- Header match appears before generic catch-all rule in `istio.proxy_routes`.
- Listener SNI/host and port are correct in `istio.proxy_listeners`.
- Upstream destination cluster exists in `istio.proxy_clusters`.

## Cookie-Based Routing

### Goal

Pin sessions or cohorts to canary/stable subsets using cookie matches.

### Diagnostic Sequence

1. Run `istio.virtualservice_status`.
2. Run `istio.proxy_routes` and inspect cookie-based header match conditions.
3. Run `istio.destinationrule_status` to verify subset mapping.
4. Run `istio.proxy_endpoints` to confirm subset endpoints are available.

### Common Failure Modes

- Cookie value format mismatch.
- Incorrect path/host prevents cookie route from matching.
- Subset endpoints unavailable, causing fallback behavior.

### Verification Checklist

- Cookie match rule is present in `istio.proxy_routes`.
- Destination subset exists in `istio.destinationrule_status`.
- Endpoint pool is not empty in `istio.proxy_endpoints`.

## Traffic Mirroring

### Goal

Mirror production traffic to a shadow backend without impacting user responses.

### Diagnostic Sequence

1. Run `istio.virtualservice_status` to verify mirror policy is accepted.
2. Run `istio.proxy_routes` to confirm mirror route appears on active path.
3. Run `istio.proxy_clusters` to check mirror cluster connectivity.
4. Run `istio.proxy_endpoints` for mirror backend endpoint availability.
5. Run `k8s.events` in mirror namespace for overload or error spikes.

### Common Failure Modes

- Mirror destination configured but route not programmed.
- Mirror backend unavailable and silently dropping mirrored requests.
- Mirror policy attached to wrong host or gateway.

### Verification Checklist

- Mirror action visible in `istio.proxy_routes`.
- Mirror cluster present in `istio.proxy_clusters`.
- Mirror endpoints healthy in `istio.proxy_endpoints`.

## Fault Injection

### Goal

Inject delays or aborts for resilience testing.

### Diagnostic Sequence

1. Run `istio.virtualservice_status` and confirm fault policy accepted.
2. Run `istio.proxy_routes` to verify fault filter placement.
3. Run `k8s.events_timeline` to correlate error spikes with rollout time.
4. Run `istio.proxy_config_dump` if route-level details are unclear.

### Common Failure Modes

- Fault policy unintentionally scoped to all traffic.
- Delay/abort rule shadowing business-critical paths.
- Failure to remove old fault policies after test window.

### Guardrails

- Always define explicit match conditions in route config.
- Validate non-test traffic route paths in `istio.proxy_routes`.
- Re-run `istio.virtualservice_status` after rollback.

## Timeout and Retry Patterns

### Goal

Tune request timeouts and retries to improve reliability without creating retry storms.

### Diagnostic Sequence

1. Run `istio.virtualservice_status` for timeout/retry policy readiness.
2. Run `istio.destinationrule_status` for connection pool and outlier policies.
3. Run `istio.proxy_clusters` to inspect effective retry and transport settings.
4. Run `k8s.events_timeline` to map latency incidents to policy changes.

### Failure Patterns

- Timeout lower than backend p95 latency causes unnecessary failures.
- Retries on non-idempotent methods lead to duplicate writes.
- Aggressive retries plus small connection pool amplifies load.

### Verification Checklist

- Retry count and backoff are visible in effective proxy config.
- Connection pool settings support expected concurrency.
- Endpoint health remains stable during retry tests.

## Gateway and HTTPRoute Interactions

1. Use `istio.gateway_status` to validate gateway readiness.
2. Use `istio.httproute_status` for Gateway API route attachment.
3. Use `istio.proxy_listeners` to confirm listener-level host/port wiring.
4. Use `istio.proxy_routes` to ensure request paths map to intended backend.

## Service-Level Mapping Workflow

1. Run `istio.pods_by_service` for source service.
2. Run `istio.pods_by_service` for destination service.
3. Pick representative source and destination pods.
4. Run `istio.proxy_routes` and `istio.proxy_endpoints` on source pod.
5. Confirm destination pod set matches expected subset.

## Incident Playbooks

### Playbook 1: Canary receives 0% when configured to 10%

1. `istio.virtualservice_status`.
2. `istio.destinationrule_status`.
3. `istio.proxy_routes`.
4. `istio.proxy_endpoints`.
5. `k8s.events_timeline`.

Likely causes:

- Route order shadowing weighted route.
- Missing canary subset labels.
- No canary endpoints ready.

### Playbook 2: Header-based route never matches

1. `istio.virtualservice_status`.
2. `istio.proxy_routes`.
3. `istio.proxy_listeners`.
4. `k8s.describe` for ingress object if host mismatch suspected.

Likely causes:

- Header key/value mismatch.
- Listener host mismatch.
- Route precedence issue.

### Playbook 3: Mirror backend overload

1. `istio.proxy_routes` to confirm mirror scope.
2. `istio.proxy_endpoints` for mirror backend capacity.
3. `k8s.events` for OOM/restart warnings.
4. `k8s.events_timeline` for exact onset timing.

Likely causes:

- Mirror backend underprovisioned.
- Mirror not sampled.
- Mirror includes heavy endpoints unintentionally.

## Change Verification Protocol

After any traffic-policy change:

1. Re-run `istio.virtualservice_status`.
2. Re-run `istio.destinationrule_status`.
3. Re-run `istio.proxy_routes` and `istio.proxy_clusters`.
4. Re-run `istio.proxy_endpoints`.
5. Validate no fresh warning events via `k8s.events`.

## Operational Recommendations

- Use small weight increments for canary shifts.
- Couple retries with strict timeout budgeting.
- Keep fault injection rules time-bounded and clearly scoped.
- Validate effective proxy config, not only CR acceptance.
- Keep gateway and route host definitions explicit and minimal.

## Evidence to Include in Reports

- Route and subset status from `istio.virtualservice_status` and `istio.destinationrule_status`.
- Effective route snippets from `istio.proxy_routes`.
- Cluster and endpoint evidence from `istio.proxy_clusters` and `istio.proxy_endpoints`.
- Timeline evidence from `k8s.events_timeline`.
- Clear before/after verification outputs.

## Related Guides

- `SKILL.md` for end-to-end Istio and Linkerd triage.
- `MTLS.md` for mTLS migration and trust policy troubleshooting.

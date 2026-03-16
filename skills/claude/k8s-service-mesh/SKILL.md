# Skill: k8s-service-mesh

Comprehensive service mesh diagnostics and operations guidance for Istio and Linkerd using RootCause MCP tools only.

## Scope

- Mesh control-plane readiness and degradation analysis.
- Data-plane proxy readiness, drift, and configuration inspection.
- Routing diagnostics for VirtualService, DestinationRule, Gateway, and HTTPRoute paths.
- mTLS and identity troubleshooting workflows.
- External dependency checks and blast-radius mapping.
- Namespace mesh footprint discovery and service-to-proxy mapping.

## Tooling Rule

- Use RootCause tool names only.
- Do not use kubectl-style MCP aliases in this skill.
- Prefer mesh-specific tools first, then `k8s.describe`, `k8s.list`, `k8s.events`, and `k8s.events_timeline` for corroborating evidence.

## Triggers

- istio
- linkerd
- service mesh
- envoy
- sidecar
- mesh traffic
- mTLS
- certificate rotation
- destination rule
- virtualservice
- gateway api
- httproute
- canary routing
- mirrored traffic
- service-to-service timeout
- retries causing overload
- identity handshake failures

## Canonical Tool List

### Istio

- `istio.health`
- `istio.proxy_status`
- `istio.config_summary`
- `istio.service_mesh_hosts`
- `istio.discover_namespaces`
- `istio.pods_by_service`
- `istio.external_dependency_check`
- `istio.proxy_clusters`
- `istio.proxy_listeners`
- `istio.proxy_routes`
- `istio.proxy_endpoints`
- `istio.proxy_bootstrap`
- `istio.proxy_config_dump`
- `istio.cr_status`
- `istio.virtualservice_status`
- `istio.destinationrule_status`
- `istio.gateway_status`
- `istio.httproute_status`

### Linkerd

- `linkerd.health`
- `linkerd.proxy_status`
- `linkerd.identity_issues`
- `linkerd.policy_debug`
- `linkerd.cr_status`
- `linkerd.virtualservice_status`
- `linkerd.destinationrule_status`
- `linkerd.gateway_status`
- `linkerd.httproute_status`

### Supporting Kubernetes tools

- `k8s.describe`
- `k8s.list`
- `k8s.events`
- `k8s.events_timeline`
- `k8s.network_debug`
- `k8s.config_debug`

## Priority Rules

| Condition | Action | Why |
|---|---|---|
| Mesh type unknown | Run `istio.health` and `linkerd.health` in parallel | Detect active mesh quickly |
| Control plane unhealthy | Run only health + events first | Avoid noisy deep dives while control plane is down |
| Pod-to-pod 5xx | Start `istio.proxy_status` or `linkerd.proxy_status` | Confirms data-plane readiness |
| Routing drift | Use `istio.virtualservice_status` + `istio.proxy_routes` | Compare intended vs applied routes |
| Backend unavailable | Use `istio.proxy_clusters` + `istio.proxy_endpoints` | Validate cluster and endpoint wiring |
| Gateway traffic drop | Use `istio.gateway_status` + `istio.proxy_listeners` | Validates ingress listener programming |
| mTLS handshake errors | Use `istio.proxy_bootstrap` or `linkerd.identity_issues` | Surfaces trust domain and cert chain issues |
| Cross-namespace call failures | Run `istio.discover_namespaces` + `istio.service_mesh_hosts` | Validate injection and host scoping |
| External service failures | Run `istio.external_dependency_check` | Detect broken ServiceEntry-style dependencies |

## Investigation Workflow

### Phase 1: Determine active mesh and baseline health

1. Run `istio.health`.
2. Run `linkerd.health`.
3. If both are detected, scope incident by namespace and traffic path.
4. Capture control-plane failures and warning events with `k8s.events`.

### Phase 2: Verify sidecar and proxy readiness

1. For Istio, run `istio.proxy_status`.
2. For Linkerd, run `linkerd.proxy_status`.
3. For Istio namespaces, run `istio.discover_namespaces`.
4. Map backing pods with `istio.pods_by_service` for affected services.
5. Correlate workload events with `k8s.events_timeline`.

### Phase 3: Validate routing control-plane resources

1. Run `istio.config_summary` for resource density and missing classes.
2. Run `istio.virtualservice_status` and capture condition failures.
3. Run `istio.destinationrule_status` for subset and TLS policy issues.
4. Run `istio.gateway_status` or `istio.httproute_status` for ingress path issues.
5. For Linkerd policy concerns, run `linkerd.policy_debug` and `linkerd.cr_status`.

### Phase 4: Inspect effective proxy configuration

1. Run `istio.proxy_listeners` on an affected source proxy.
2. Run `istio.proxy_routes` and confirm route match and weighted destinations.
3. Run `istio.proxy_clusters` for upstream cluster config and transport sockets.
4. Run `istio.proxy_endpoints` to verify discovered endpoints and locality.
5. Run `istio.proxy_bootstrap` for trust-domain and xDS server details.
6. If mismatch remains unclear, run `istio.proxy_config_dump`.

### Phase 5: Check external dependencies and boundary conditions

1. Run `istio.external_dependency_check`.
2. Use `istio.service_mesh_hosts` to list referenced internal and external hosts.
3. Use `k8s.network_debug` if the target service appears healthy but unreachable.
4. Use `k8s.config_debug` to verify backend cert/config mounts when TLS fails.

## Istio Troubleshooting Patterns

### Pattern A: 503 UF / no healthy upstream

1. Run `istio.proxy_endpoints` on caller proxy.
2. If empty endpoints, run `istio.pods_by_service` for target service.
3. If pods exist but endpoints missing, inspect route with `istio.proxy_routes`.
4. Confirm destination host/subset in `istio.destinationrule_status`.

### Pattern B: Route exists but wrong backend selected

1. Run `istio.virtualservice_status`.
2. Run `istio.proxy_routes` and inspect match order.
3. Confirm cluster wiring using `istio.proxy_clusters`.
4. Validate subset labels and policy state with `istio.destinationrule_status`.

### Pattern C: Ingress returns 404 or 503

1. Run `istio.gateway_status`.
2. Run `istio.httproute_status` if Gateway API path is used.
3. Run `istio.proxy_listeners` on gateway proxy pod.
4. Run `istio.proxy_routes` to verify host/path matches.

### Pattern D: Intermittent timeout spikes

1. Run `istio.proxy_clusters` and inspect circuit-breaker or outlier settings.
2. Run `istio.proxy_endpoints` for endpoint churn.
3. Check service resource churn with `k8s.events_timeline`.
4. Review retry/timeout policy in `istio.virtualservice_status` and `istio.destinationrule_status`.

## Linkerd Troubleshooting Patterns

### Pattern A: Identity failures

1. Run `linkerd.identity_issues`.
2. Run `linkerd.health` to ensure identity controller readiness.
3. Use `k8s.events` in control-plane namespace for cert rotation warnings.

### Pattern B: Missing sidecar or policy mismatch

1. Run `linkerd.proxy_status`.
2. Run `linkerd.policy_debug` to verify policy CRD presence.
3. Run `linkerd.cr_status` on relevant policy objects.

### Pattern C: Gateway API route not attaching

1. Run `linkerd.gateway_status`.
2. Run `linkerd.httproute_status`.
3. Use `k8s.describe` on failing route object for conditions and refs.

## Decision Tree

1. Is a mesh detected?
2. Is control plane healthy?
3. Are sidecars healthy and injected?
4. Are routing CR conditions accepted?
5. Does effective proxy config match desired routing?
6. Are upstream endpoints available?
7. Is identity/mTLS healthy?
8. Are external dependencies reachable?

## Evidence Collection Contract

Always collect and return:

- Mesh type detected: Istio, Linkerd, both, or none.
- Control-plane health status and failing components.
- Proxy readiness summary by namespace and workload.
- Routing CR status failures with names and conditions.
- Effective proxy mismatch details (listeners/routes/clusters/endpoints).
- Identity or TLS error evidence.
- External dependency findings.
- Ranked remediation actions and verification checks.

## Common Failure Signatures

| Signature | Most likely layer | Primary tool | Next tool |
|---|---|---|---|
| 503 UF | endpoint discovery | `istio.proxy_endpoints` | `istio.pods_by_service` |
| 503 NR | route miss | `istio.proxy_routes` | `istio.virtualservice_status` |
| TLS handshake reset | identity/mTLS | `istio.proxy_bootstrap` | `linkerd.identity_issues` |
| Gateway 404 | host/path mismatch | `istio.gateway_status` | `istio.proxy_routes` |
| High retries and latency | resilience policy mismatch | `istio.proxy_clusters` | `istio.destinationrule_status` |
| Linkerd identity pending | cert/control-plane issue | `linkerd.identity_issues` | `linkerd.health` |

## Troubleshooting Checklists

### Control-plane checklist

- Run `istio.health` and/or `linkerd.health`.
- Capture warning events with `k8s.events`.
- Confirm no widespread control-plane restart loops.

### Data-plane checklist

- Run `istio.proxy_status` or `linkerd.proxy_status`.
- Validate namespace injection footprint with `istio.discover_namespaces`.
- Validate service backing pods via `istio.pods_by_service`.

### Routing checklist

- Check resource summary with `istio.config_summary`.
- Inspect per-kind status with `istio.virtualservice_status`, `istio.destinationrule_status`, `istio.gateway_status`, `istio.httproute_status`.
- Compare to effective route programming via `istio.proxy_routes`.

### mTLS checklist

- Inspect trust-domain and cert providers via `istio.proxy_bootstrap`.
- Inspect connection transport sockets via `istio.proxy_clusters`.
- For Linkerd identity issues, run `linkerd.identity_issues`.
- Correlate failures with event timeline using `k8s.events_timeline`.

## Operational Playbooks

### Playbook: rollout introduced mesh regression

1. Capture timeline with `k8s.events_timeline`.
2. Validate CR statuses after rollout using `istio.virtualservice_status` and `istio.destinationrule_status`.
3. Verify effective routes with `istio.proxy_routes`.
4. Verify endpoint health with `istio.proxy_endpoints`.
5. Verify fix by rerunning status and proxy checks.

### Playbook: external API dependency failing

1. Run `istio.external_dependency_check`.
2. Confirm host references using `istio.service_mesh_hosts`.
3. Inspect caller proxy clusters via `istio.proxy_clusters`.
4. Review network path using `k8s.network_debug`.

### Playbook: multi-mesh migration confusion

1. Detect both meshes with `istio.health` and `linkerd.health`.
2. Segment namespaces by ownership and sidecar readiness.
3. Use mesh-specific status tools only for workloads on that mesh.
4. Avoid mixing route diagnoses across meshes in one conclusion.

## Escalation Guidance

Escalate if any condition below is true:

- Control plane unhealthy for more than one reconciliation interval.
- Proxies are not receiving xDS updates for critical services.
- mTLS identity failures affect multiple namespaces.
- Gateway ingress path fails for production external traffic.
- External dependency failures affect top business paths.

## References

- `TRAFFIC-SHIFTING.md` for weighted routing, headers, cookies, mirroring, retries, and fault injection analysis.
- `MTLS.md` for PeerAuthentication and DestinationRule TLS migration and troubleshooting.

## Related Skills

- `k8s-security` for RBAC, policy, and network isolation checks.
- `k8s-networking` for service and network-policy path analysis outside mesh control.

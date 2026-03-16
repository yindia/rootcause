# Skill: k8s-operations

Safety-first mutation guide for Kubernetes operations with RootCause MCP tool names.

## Mission

Execute mutations deliberately with evidence and guardrails.

Golden rule:
no write operation without preflight.

## Mutation Coverage

This skill handles:
- `k8s.apply`
- `k8s.create`
- `k8s.patch`
- `k8s.delete`
- `k8s.scale`
- `k8s.rollout`
- `k8s.node_management`
- `k8s.cleanup_pods`

Safety and verification companions:
- `k8s.safe_mutation_preflight`
- `k8s.restart_safety_check`
- `k8s.best_practice`
- `k8s.get`
- `k8s.describe`
- `k8s.events_timeline`

## Universal Mutation Pattern

For every write operation:
1. classify operation type
2. run `k8s.safe_mutation_preflight`
3. if restart path, run `k8s.restart_safety_check`
4. execute mutation with `confirm: true`
5. verify object and event state

If preflight reports blockers:
- stop mutation
- report blocker details
- recommend safe alternatives

## Required Confirm Rule

All write calls in this skill require `confirm: true`.

Never run:
- `k8s.apply` without confirm
- `k8s.create` without confirm
- `k8s.patch` without confirm
- `k8s.delete` without confirm
- `k8s.scale` without confirm
- `k8s.rollout` without confirm
- `k8s.node_management` without confirm
- `k8s.cleanup_pods` without confirm

## Preflight Examples for Every Write Operation

Preflight for apply:
```json
{
  "operation": "apply",
  "kind": "Deployment",
  "name": "api",
  "namespace": "payments",
  "manifest": "apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: api\n..."
}
```

Preflight for create:
```json
{
  "operation": "create",
  "kind": "ConfigMap",
  "name": "api-config-v2",
  "namespace": "payments",
  "manifest": "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: api-config-v2\n..."
}
```

Preflight for patch:
```json
{
  "operation": "patch",
  "kind": "Deployment",
  "name": "api",
  "namespace": "payments",
  "patch": "{\"spec\":{\"replicas\":6}}"
}
```

Preflight for delete:
```json
{
  "operation": "delete",
  "kind": "Pod",
  "name": "api-6fd5d4f7fb-9w2rc",
  "namespace": "payments"
}
```

Preflight for scale:
```json
{
  "operation": "scale",
  "kind": "Deployment",
  "name": "api",
  "namespace": "payments",
  "replicas": 6
}
```

Preflight for rollout:
```json
{
  "operation": "rollout",
  "kind": "Deployment",
  "name": "api",
  "namespace": "payments"
}
```

Preflight for node maintenance:
```json
{
  "operation": "node_management",
  "kind": "Node",
  "name": "ip-10-0-32-17.ec2.internal"
}
```

Preflight for cleanup pods:
```json
{
  "operation": "cleanup_pods",
  "namespace": "payments"
}
```

## Direct Mutation Examples

`k8s.apply`:
```json
{
  "confirm": true,
  "namespace": "payments",
  "fieldManager": "rootcause-ops",
  "force": false,
  "manifest": "apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: api\n..."
}
```

`k8s.create`:
```json
{
  "confirm": true,
  "namespace": "payments",
  "manifest": "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: api-config-v2\n..."
}
```

`k8s.patch` merge:
```json
{
  "confirm": true,
  "kind": "Deployment",
  "name": "api",
  "namespace": "payments",
  "patchType": "merge",
  "patch": "{\"spec\":{\"replicas\":6}}"
}
```

`k8s.patch` json:
```json
{
  "confirm": true,
  "kind": "Deployment",
  "name": "api",
  "namespace": "payments",
  "patchType": "json",
  "patch": "[{\"op\":\"replace\",\"path\":\"/spec/replicas\",\"value\":6}]"
}
```

`k8s.patch` strategic:
```json
{
  "confirm": true,
  "kind": "Deployment",
  "name": "api",
  "namespace": "payments",
  "patchType": "strategic",
  "patch": "{\"spec\":{\"template\":{\"spec\":{\"containers\":[{\"name\":\"api\",\"image\":\"ghcr.io/acme/api:v1.13.0\"}]}}}}"
}
```

`k8s.delete` object:
```json
{
  "confirm": true,
  "kind": "Pod",
  "name": "api-6fd5d4f7fb-9w2rc",
  "namespace": "payments"
}
```

`k8s.scale`:
```json
{
  "confirm": true,
  "kind": "Deployment",
  "name": "api",
  "namespace": "payments",
  "replicas": 6
}
```

`k8s.rollout` status:
```json
{
  "confirm": true,
  "action": "status",
  "kind": "Deployment",
  "name": "api",
  "namespace": "payments"
}
```

`k8s.rollout` restart:
```json
{
  "confirm": true,
  "action": "restart",
  "kind": "Deployment",
  "name": "api",
  "namespace": "payments"
}
```

`k8s.restart_safety_check`:
```json
{
  "name": "api",
  "namespace": "payments",
  "minReadyReplicas": 2,
  "maxUnavailableRatio": 0.25
}
```

`k8s.node_management` cordon:
```json
{
  "confirm": true,
  "action": "cordon",
  "nodeName": "ip-10-0-32-17.ec2.internal"
}
```

`k8s.node_management` drain:
```json
{
  "confirm": true,
  "action": "drain",
  "nodeName": "ip-10-0-32-17.ec2.internal",
  "force": false,
  "gracePeriodSeconds": 60
}
```

`k8s.node_management` uncordon:
```json
{
  "confirm": true,
  "action": "uncordon",
  "nodeName": "ip-10-0-32-17.ec2.internal"
}
```

`k8s.cleanup_pods`:
```json
{
  "confirm": true,
  "namespace": "payments",
  "states": ["evicted", "crashloop", "imagepullbackoff"],
  "labelSelector": "app=api"
}
```

`k8s.best_practice` before and after high-risk changes:
```json
{
  "kind": "Deployment",
  "name": "api",
  "namespace": "payments"
}
```

## Patch Types Deep Dive

### merge patch

Best for simple map replacements and scalar changes.

Pros:
- concise
- easy to reason about

Cons:
- list handling can be unintuitive

Typical use:
- update `spec.replicas`
- update service selector map

### json patch

Best for explicit operation sequencing.

Pros:
- precise add/remove/replace control
- explicit paths reduce ambiguity

Cons:
- verbose
- path-sensitive errors

Typical use:
- remove specific list item
- append exact annotations or labels

### strategic patch

Best for built-in resource structs with merge semantics.

Pros:
- list merge by key for containers
- natural for Deployment container image updates

Cons:
- not all custom resources support expected semantics

Typical use:
- update one named container image
- add environment variable in a container spec

## Rollout Management Workflow

### Safe restart flow

1. `k8s.safe_mutation_preflight` with `operation: "rollout"`
2. `k8s.restart_safety_check`
3. `k8s.rollout` action `restart`
4. `k8s.rollout` action `status`
5. `k8s.events_timeline` warning check

### Safe scale flow

1. `k8s.safe_mutation_preflight` with `operation: "scale"`
2. `k8s.scale`
3. `k8s.rollout` action `status`
4. `k8s.describe` deployment for condition review

### Apply flow

1. `k8s.safe_mutation_preflight` with `operation: "apply"`
2. optional `k8s.best_practice`
3. `k8s.apply`
4. `k8s.rollout` action `status` if workload changes
5. `k8s.events_timeline` warning scan

## Node Maintenance Workflow

Recommended sequence:
1. identify node and resident critical workloads
2. run preflight for node operation
3. `k8s.node_management` action `cordon`
4. `k8s.node_management` action `drain`
5. validate rescheduling via `k8s.list` pods and `k8s.events_timeline`
6. perform maintenance
7. `k8s.node_management` action `uncordon`

Verification calls:

List pods impacted by node:
```json
{
  "fieldSelector": "spec.nodeName=ip-10-0-32-17.ec2.internal",
  "resources": [
    { "kind": "Pod" }
  ]
}
```

Node describe:
```json
{
  "kind": "Node",
  "name": "ip-10-0-32-17.ec2.internal"
}
```

## Pod Cleanup Workflow

Use cleanup for noisy bad-state accumulation.

Process:
1. list candidate pods with selectors
2. preflight cleanup operation
3. run `k8s.cleanup_pods` with explicit states
4. verify no healthy pods were touched
5. inspect events for repeated root cause

Candidate list example:
```json
{
  "namespace": "payments",
  "fieldSelector": "status.phase=Failed",
  "resources": [
    { "kind": "Pod" }
  ]
}
```

## Troubleshooting Matrix

| Symptom | Likely Blocker | Primary Tool |
|---|---|---|
| preflight fails for scale | quota or PDB limit | `k8s.safe_mutation_preflight` |
| restart not allowed | insufficient ready replica budget | `k8s.restart_safety_check` |
| patch returns conflict | field ownership collision | `k8s.patch` with adjusted type/fields |
| drain hangs | non-evictable workload constraints | `k8s.node_management` |
| cleanup leaves pods returning | underlying app/image/config issue | `k8s.events_timeline` + `k8s.describe` |

## Post-Mutation Verification Pattern

After every mutation:
1. `k8s.get` target object
2. `k8s.describe` target object
3. `k8s.events_timeline` warning-only
4. `k8s.rollout` status for Deployment-like resources

Verification calls:
```json
{
  "kind": "Deployment",
  "name": "api",
  "namespace": "payments"
}
```

```json
{
  "namespace": "payments",
  "involvedObjectKind": "Deployment",
  "involvedObjectName": "api",
  "includeNormal": false,
  "limit": 60
}
```

## Output Contract

Each operation response must include:
1. preflight result and any blockers
2. exact mutation executed and parameters
3. verification evidence after mutation
4. residual risk and next safe action

## Related Skills

- `skills/claude/k8s-core/SKILL.md`
- `skills/claude/k8s-deploy/SKILL.md`
- `skills/claude/k8s-incident/SKILL.md`
- `skills/claude/k8s-diagnostics/SKILL.md`

End of skill.

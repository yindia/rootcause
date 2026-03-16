# Skill: k8s-core

Comprehensive read-first Kubernetes reference using RootCause MCP tool names.

Use this skill when the task is to inspect, query, explain, and trace core resources.

## Mission

Deliver high-signal cluster state with minimal noise.

Default flow:
1. verify context and connectivity
2. scope namespace and selectors
3. list/get/describe resources
4. correlate logs and events
5. summarize facts and open questions

## Core Resources Covered

- Pods
- Deployments
- Services
- ConfigMaps
- Secrets
- Namespaces
- Nodes

## Core Tool Set

Read and discovery tools:
- `k8s.ping`
- `k8s.overview`
- `k8s.get`
- `k8s.list`
- `k8s.describe`
- `k8s.logs`
- `k8s.events`
- `k8s.events_timeline`
- `k8s.context`
- `k8s.api_resources`
- `k8s.crds`
- `k8s.explain_resource`
- `k8s.exec`
- `k8s.port_forward`

## Priority Rules

| Priority | Rule | Reason |
|---|---|---|
| P0 | Run `k8s.context` then `k8s.ping` before data collection | prevent wrong-cluster conclusions |
| P1 | Start broad with `k8s.overview` for unknown scope | quick health baseline |
| P2 | Prefer `k8s.list` with selectors before `k8s.get` loops | less API chatter, cleaner outputs |
| P3 | Use `k8s.describe` for state transitions and owner chains | richer operational context |
| P4 | Use `k8s.events_timeline` for chronology, `k8s.events` for raw detail | time ordering matters |

## Parameter Examples By Tool

`k8s.context` list contexts:
```json
{
  "action": "list"
}
```

`k8s.context` current context:
```json
{
  "action": "current"
}
```

`k8s.ping`:
```json
{}
```

`k8s.overview` cluster scope:
```json
{}
```

`k8s.overview` namespace scope:
```json
{
  "namespace": "payments"
}
```

`k8s.get` by kind:
```json
{
  "kind": "Pod",
  "name": "api-6fd5d4f7fb-9w2rc",
  "namespace": "payments"
}
```

`k8s.get` by resource + apiVersion:
```json
{
  "apiVersion": "apps/v1",
  "resource": "deployments",
  "name": "api",
  "namespace": "payments"
}
```

`k8s.list` pods with label selector:
```json
{
  "namespace": "payments",
  "labelSelector": "app=api,tier=backend",
  "resources": [
    { "kind": "Pod" }
  ]
}
```

`k8s.list` services + deployments:
```json
{
  "namespace": "payments",
  "resources": [
    { "kind": "Service" },
    { "kind": "Deployment" }
  ]
}
```

`k8s.list` field selector example:
```json
{
  "namespace": "payments",
  "fieldSelector": "status.phase!=Running",
  "resources": [
    { "kind": "Pod" }
  ]
}
```

`k8s.describe` deployment:
```json
{
  "kind": "Deployment",
  "name": "api",
  "namespace": "payments"
}
```

`k8s.logs` current container logs:
```json
{
  "namespace": "payments",
  "pod": "api-6fd5d4f7fb-9w2rc",
  "container": "api",
  "tailLines": 200,
  "sinceSeconds": 900
}
```

`k8s.events` namespace events:
```json
{
  "namespace": "payments"
}
```

`k8s.events` object-focused:
```json
{
  "namespace": "payments",
  "involvedObjectKind": "Pod",
  "involvedObjectName": "api-6fd5d4f7fb-9w2rc"
}
```

`k8s.events_timeline` warning focused:
```json
{
  "namespace": "payments",
  "includeNormal": false,
  "limit": 100
}
```

`k8s.api_resources` query filter:
```json
{
  "query": "deployment",
  "limit": 50
}
```

`k8s.crds` query filter:
```json
{
  "query": "gateway",
  "limit": 50
}
```

`k8s.explain_resource` by kind:
```json
{
  "apiVersion": "apps/v1",
  "kind": "Deployment"
}
```

`k8s.exec` non-shell command:
```json
{
  "namespace": "payments",
  "pod": "api-6fd5d4f7fb-9w2rc",
  "container": "api",
  "command": ["printenv"]
}
```

`k8s.port_forward` service:
```json
{
  "namespace": "payments",
  "service": "api",
  "ports": ["8080:80"],
  "durationSeconds": 120
}
```

## CRUD Reference (Read-Centric)

Note:
This skill is read-first.
When write operations are needed, switch to `k8s-operations`.

### Pods

Common questions:
- which pods are not running
- which node hosts critical pod
- what restarted recently

Examples:

List failing pods:
```json
{
  "namespace": "payments",
  "fieldSelector": "status.phase!=Running",
  "resources": [
    { "kind": "Pod" }
  ]
}
```

Describe pod:
```json
{
  "kind": "Pod",
  "name": "api-6fd5d4f7fb-9w2rc",
  "namespace": "payments"
}
```

Fetch pod logs:
```json
{
  "namespace": "payments",
  "pod": "api-6fd5d4f7fb-9w2rc",
  "tailLines": 150,
  "sinceSeconds": 600
}
```

### Deployments

Common questions:
- desired vs available replicas
- rollout status inferred from conditions
- image currently deployed

Examples:

Get deployment:
```json
{
  "kind": "Deployment",
  "name": "api",
  "namespace": "payments"
}
```

List deployments by label:
```json
{
  "namespace": "payments",
  "labelSelector": "team=checkout",
  "resources": [
    { "kind": "Deployment" }
  ]
}
```

Describe deployment:
```json
{
  "kind": "Deployment",
  "name": "api",
  "namespace": "payments"
}
```

### Services

Common questions:
- selector mismatch
- endpoints missing
- port mapping confusion

Examples:

Get service:
```json
{
  "kind": "Service",
  "name": "api",
  "namespace": "payments"
}
```

Describe service:
```json
{
  "kind": "Service",
  "name": "api",
  "namespace": "payments"
}
```

List services:
```json
{
  "namespace": "payments",
  "resources": [
    { "kind": "Service" }
  ]
}
```

### ConfigMaps

Common questions:
- missing key
- stale config version

Examples:

Get ConfigMap:
```json
{
  "kind": "ConfigMap",
  "name": "api-config",
  "namespace": "payments"
}
```

List ConfigMaps by app label:
```json
{
  "namespace": "payments",
  "labelSelector": "app=api",
  "resources": [
    { "kind": "ConfigMap" }
  ]
}
```

### Secrets

Common questions:
- secret exists but app still failing
- wrong namespace reference

Examples:

Get Secret metadata:
```json
{
  "kind": "Secret",
  "name": "api-secrets",
  "namespace": "payments"
}
```

List secrets by label:
```json
{
  "namespace": "payments",
  "labelSelector": "app=api",
  "resources": [
    { "kind": "Secret" }
  ]
}
```

### Namespaces

Common questions:
- list team namespaces
- inspect quota/event pressure by namespace

Examples:

List namespaces:
```json
{
  "resources": [
    { "kind": "Namespace" }
  ]
}
```

Get one namespace:
```json
{
  "kind": "Namespace",
  "name": "payments"
}
```

Namespace events:
```json
{
  "namespace": "payments"
}
```

### Nodes

Common questions:
- node readiness and pressure
- workload placement

Examples:

List nodes:
```json
{
  "resources": [
    { "kind": "Node" }
  ]
}
```

Get one node:
```json
{
  "kind": "Node",
  "name": "ip-10-0-32-17.ec2.internal"
}
```

Describe node:
```json
{
  "kind": "Node",
  "name": "ip-10-0-32-17.ec2.internal"
}
```

## Filtering Patterns

### labelSelector patterns

Use for ownership and workload grouping:
- `app=api`
- `app=api,tier=backend`
- `team in (payments,checkout)`
- `environment=prod`

### fieldSelector patterns

Use for runtime state filtering:
- `status.phase=Pending`
- `status.phase!=Running`
- `metadata.name=api-6fd5d4f7fb-9w2rc`

## Multi-Context Operations

Multi-context read flow:
1. `k8s.context` with `action: "list"`
2. `k8s.context` with `action: "current"`
3. if needed, switch context using `k8s.context` with `action: "use"`
4. run `k8s.ping`
5. run read queries

Switch context example:
```json
{
  "action": "use"
}
```

After switching, verify with:
```json
{
  "action": "current"
}
```

## Practical Query Playbooks

### Playbook: find why pod is not ready

1. `k8s.list` non-running pods
2. `k8s.describe` target pod
3. `k8s.logs` target container
4. `k8s.events_timeline` for pod

### Playbook: validate service wiring

1. `k8s.get` Service
2. `k8s.list` Pods with matching labels
3. `k8s.describe` Service
4. `k8s.port_forward` temporary probe

### Playbook: verify deployment state

1. `k8s.get` Deployment
2. `k8s.describe` Deployment
3. `k8s.list` Pods by deployment labels
4. `k8s.events_timeline` warning-only

## Output Contract

Every response should include:
1. context and connectivity status
2. exact resource scope used (namespace/selectors)
3. factual findings grouped by resource type
4. event/log evidence when relevant
5. clear next checks when evidence is incomplete

## Related Skills

- `skills/claude/k8s-operations/SKILL.md`
- `skills/claude/k8s-diagnostics/SKILL.md`
- `skills/claude/k8s-incident/SKILL.md`

End of skill.

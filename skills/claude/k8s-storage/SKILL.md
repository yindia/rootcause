# Skill: k8s-storage

Comprehensive Kubernetes persistent storage diagnostics covering PVC lifecycle,
StorageClass behavior, snapshot workflows, mount failures, and attach/detach errors.

## Scope

Use this skill for stateful workload issues where data volume behavior blocks scheduling,
startup, recovery, scaling, or deletion.

## Trigger Phrases

Use this skill when the user mentions:
- pvc pending
- pv not bound
- volume mount failed
- multi attach error
- volumeattachment issue
- storage class misconfiguration
- csi error
- ebs or efs issue
- snapshot restore failed
- pod stuck containercreating
- filesystem readonly
- disk full in statefulset

## RootCause Tools Allowed

Only use these tool names in this skill:
- `k8s.storage_debug`
- `k8s.describe`
- `k8s.list`
- `k8s.events`
- `k8s.events_timeline`

## PVC Lifecycle Reference

### Typical lifecycle states

| PVC Phase | Meaning | Typical Next Step |
|---|---|---|
| `Pending` | Not yet bound to a volume | Inspect StorageClass and provisioning events |
| `Bound` | Successfully attached to PV | Validate pod mount and filesystem |
| `Lost` | Underlying PV unavailable/reclaimed unexpectedly | Trace reclaim events and backend health |

### PV lifecycle states commonly observed

| PV Phase | Meaning | Operator Concern |
|---|---|---|
| `Available` | Unclaimed static volume | Match claim requirements |
| `Bound` | Claimed by PVC | Confirm claimRef is expected |
| `Released` | PVC removed, reclaim pending | Reclaim policy and cleanup process |
| `Failed` | Provisioning/reclaim failed | Storage backend or driver issue |

## StorageClass Configuration Checklist

When diagnosing provisioning behavior, verify:
- `provisioner` value maps to installed CSI driver.
- `volumeBindingMode` is expected (`Immediate` or `WaitForFirstConsumer`).
- `reclaimPolicy` aligns with retention policy (`Delete` or `Retain`).
- `allowVolumeExpansion` matches growth requirements.
- parameters match cloud-specific constraints.

Use `k8s.describe` on StorageClass for all checks.

## End-to-End Workflow

### Step 1: Run storage debug first

Use `k8s.storage_debug` with either a pod or PVC context.

Pod-first example:
```yaml
namespace: orders
pod: orders-db-0
includeEvents: true
```

PVC-first example:
```yaml
namespace: orders
pvc: data-orders-db-0
includeEvents: true
```

This gives the fastest signal on binding, matching, and attachment issues.

### Step 2: Inventory storage objects

Use `k8s.list` to map claims, volumes, and classes.

Example:
```yaml
namespace: orders
resources:
  - kind: PersistentVolumeClaim
  - kind: PersistentVolume
  - kind: StorageClass
```

Focus on:
- many `Pending` claims
- unexpected default StorageClass
- stale PVs in `Released`

### Step 3: Deep inspect failing claim

Use `k8s.describe` on target PVC.

Required fields to inspect:
- `status.phase`
- `spec.storageClassName`
- `spec.accessModes`
- `spec.resources.requests.storage`
- event messages from provisioner

### Step 4: Validate matched PV (if any)

Use `k8s.describe` on bound or candidate PV.

Check:
- `spec.capacity.storage`
- `spec.accessModes`
- `spec.claimRef`
- node affinity requirements
- backend volume identifier

### Step 5: Analyze event chronology

Use `k8s.events` and `k8s.events_timeline` for causality.

PVC timeline example:
```yaml
namespace: orders
involvedObjectKind: PersistentVolumeClaim
involvedObjectName: data-orders-db-0
includeNormal: false
```

Pod mount timeline example:
```yaml
namespace: orders
involvedObjectKind: Pod
involvedObjectName: orders-db-0
includeNormal: false
```

### Step 6: Conclude with exact failure mode

Map issue to one of:
- dynamic provisioning failure
- no static PV match
- access mode mismatch
- attach/detach controller failure
- node-level mount failure
- reclaim or reuse lifecycle problem

## Snapshot and Restore Guidance

Even though snapshot operations are backend-specific, diagnostic flow is consistent:
1. Confirm source PVC health before snapshot.
2. Confirm snapshot/restore CR events (if available in cluster integrations).
3. Validate restored PVC binding and mount.
4. Confirm restored workload sees expected data.

When snapshot restore is suspected broken:
- `k8s.storage_debug` on restored pod/PVC.
- `k8s.events_timeline` for restore-time ordering.
- `k8s.describe` on restored claim and target workload.

## Common Troubleshooting Paths

### PVC stuck Pending

Run in this order:
1. `k8s.storage_debug`
2. `k8s.describe` PVC
3. `k8s.describe` StorageClass
4. `k8s.events_timeline`

Likely causes:
- missing or invalid StorageClass
- CSI provisioner down or permission issue
- quota/limit preventing allocation
- topology constraints unsatisfied

### Mount failures on running node

Run in this order:
1. `k8s.storage_debug` (pod mode)
2. `k8s.describe` Pod
3. `k8s.events` on Pod
4. `k8s.events_timeline` on Pod

Likely causes:
- multi-attach conflict
- filesystem corruption or readonly remount
- node plugin transient errors

### Access mode mismatch

Signs:
- PVC requests `ReadWriteMany` but backend supports only `ReadWriteOnce`.
- StatefulSet template copied from incompatible class.

Confirm with:
- `k8s.describe` PVC
- `k8s.describe` PV

### Released PV not reusable

Signs:
- PV in `Released` long after claim deletion.
- new PVC remains pending despite capacity availability.

Confirm with:
- `k8s.list` PV
- `k8s.describe` PV claimRef/reclaim policy

## YAML Examples

### Example PVC with explicit class and mode

```yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: app-data
  namespace: orders
spec:
  accessModes:
    - ReadWriteOnce
  storageClassName: gp3
  resources:
    requests:
      storage: 20Gi
```

### Example StatefulSet volumeClaimTemplates

```yaml
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: orders-db
  namespace: orders
spec:
  serviceName: orders-db
  replicas: 1
  selector:
    matchLabels:
      app: orders-db
  template:
    metadata:
      labels:
        app: orders-db
    spec:
      containers:
        - name: db
          image: postgres:16
          volumeMounts:
            - name: data
              mountPath: /var/lib/postgresql/data
  volumeClaimTemplates:
    - metadata:
        name: data
      spec:
        accessModes:
          - ReadWriteOnce
        storageClassName: gp3
        resources:
          requests:
            storage: 50Gi
```

## Common Errors Table

| Error Message | Likely Root Cause | Confirm With | Remediation Direction |
|---|---|---|---|
| `no persistent volumes available for this claim` | No PV match and dynamic provisioning unavailable | `k8s.storage_debug`, `k8s.describe` PVC | Fix StorageClass or create matching PV |
| `waiting for first consumer` | `WaitForFirstConsumer` with unscheduled pod | `k8s.describe` PVC and pod events | Schedule pod or resolve scheduling constraints |
| `Multi-Attach error` | Same RWO volume attached to another node | `k8s.events_timeline` Pod | Drain conflicting attachment path |
| `MountVolume.SetUp failed` | Node mount/plugin failure | `k8s.storage_debug`, `k8s.events` | Inspect node plugin and retry mount |
| `timed out waiting for the condition` | Provisioner/backing storage latency or failure | `k8s.events_timeline` PVC | Validate backend capacity and driver health |
| `PVC is being deleted` stuck | Finalizer/protection not cleared | `k8s.describe` PVC | Resolve finalizer dependencies safely |

## Output Contract

Return results with:
1. Failing object and lifecycle state.
2. Evidence from at least two tools.
3. Exact blocking condition message.
4. Recommended fix path (safe first).
5. Verification steps to prove recovery.

Example:
```text
Root cause: orders/data-orders-db-0 PVC remains Pending because StorageClass gp3 does not exist in cluster.
Evidence: k8s.storage_debug reports class not found; k8s.describe PVC events show provisioner lookup failure.
Fix: Update PVC/StatefulSet to existing class gp2 or create gp3 StorageClass.
Verify: Re-run k8s.storage_debug and confirm PVC phase transitions to Bound and pod mounts volume.
```

## Related Skills

- `k8s-autoscaling` when storage pressure interacts with right-sizing and node constraints.
- `k8s-rollouts` when restart/upgrade safety depends on persistent data state.

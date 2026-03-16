# Kubernetes Troubleshooting Decision Trees

Use these ASCII flowcharts to pick the first RootCause MCP tool quickly.
Tool names are intentionally in `k8s.*` format.

---

## 1) Pod Not Running

```text
START: Pod is not healthy
|
+-- Is pod phase Pending?
|   |
|   +-- YES -> run k8s.scheduling_debug
|   |   |
|   |   +-- mentions taints / affinity / selector mismatch?
|   |   |   |
|   |   |   +-- YES -> run k8s.describe (pod) and k8s.events_timeline
|   |   |   |
|   |   |   +-- NO -> check quota/capacity via k8s.resource_usage
|   |   |
|   |   +-- still Pending after constraints fixed?
|   |       |
|   |       +-- run k8s.debug_flow scenario=pending
|   |
|   +-- NO -> continue
|
+-- Is status CrashLoopBackOff / Init:CrashLoopBackOff?
|   |
|   +-- YES -> run k8s.crashloop_debug
|   |   |
|   |   +-- image pull failure reported?
|   |   |   |
|   |   |   +-- YES -> run k8s.describe and k8s.events_timeline
|   |   |   |
|   |   |   +-- NO -> run k8s.logs and inspect startup error
|   |   |
|   |   +-- startup says missing secret/config?
|   |       |
|   |       +-- YES -> run k8s.config_debug
|   |       |
|   |       +-- NO -> continue app-level debugging with k8s.logs
|   |
|   +-- NO -> continue
|
+-- Is state OOMKilled or exit code 137?
|   |
|   +-- YES -> run k8s.logs
|   |   |
|   |   +-- confirm memory pressure in k8s.describe
|   |   |
|   |   +-- analyze saturation via k8s.resource_usage
|   |   |
|   |   +-- right-size recommendation via k8s.vpa_debug
|   |
|   +-- NO -> continue
|
+-- Is state CreateContainerConfigError?
|   |
|   +-- YES -> run k8s.config_debug
|   |   |
|   |   +-- verify missing key/object via k8s.describe
|   |   |
|   |   +-- correlate with rollout timing via k8s.events_timeline
|   |
|   +-- NO -> continue
|
+-- Is state Running but restarts increasing?
|   |
|   +-- YES -> run k8s.logs
|   |   |
|   |   +-- if probe failures appear -> run k8s.network_debug
|   |   |
|   |   +-- if auth failures appear -> run k8s.permission_debug
|   |   |
|   |   +-- if startup loops reappear -> run k8s.crashloop_debug
|   |
|   +-- NO -> continue
|
+-- Unclear or multiple symptoms?
    |
    +-- run k8s.diagnose (keyword from dominant symptom)
    |
    +-- then run k8s.debug_flow with matching scenario
```

---

## 2) Service Not Accessible

```text
START: Client cannot reach service
|
+-- Is DNS/service name resolution failing?
|   |
|   +-- YES -> run k8s.network_debug
|   |   |
|   |   +-- inspect service object via k8s.describe
|   |   |
|   |   +-- check related events via k8s.events_timeline
|   |
|   +-- NO -> continue
|
+-- Is service present but no endpoints?
|   |
|   +-- YES -> run k8s.network_debug
|   |   |
|   |   +-- verify selector/pod labels with k8s.list
|   |   |
|   |   +-- inspect readiness failures via k8s.logs
|   |
|   +-- NO -> continue
|
+-- Are endpoints present but traffic still fails?
|   |
|   +-- YES -> run k8s.graph (ingress/service/pod path)
|   |   |
|   |   +-- if policy denial suspected -> k8s.debug_flow scenario=networkpolicy
|   |   |
|   |   +-- if route chain issue suspected -> k8s.debug_flow scenario=traffic
|   |   |
|   |   +-- verify backend logs via k8s.logs
|   |
|   +-- NO -> continue
|
+-- Is failure intermittent under load?
|   |
|   +-- YES -> run k8s.resource_usage
|   |   |
|   |   +-- inspect HPA behavior with k8s.hpa_debug
|   |   |
|   |   +-- inspect restart/probe flaps with k8s.describe
|   |
|   +-- NO -> continue
|
+-- Unclear source of failure?
    |
    +-- run k8s.diagnose keyword="service unreachable"
    |
    +-- run k8s.debug_flow scenario=traffic
```

---

## 3) Node Issues (Pressure, Evictions, Instability)

```text
START: Node-related symptoms observed
|
+-- Pods show Evicted / Pending spikes?
|   |
|   +-- YES -> run k8s.scheduling_debug
|   |   |
|   |   +-- examine events via k8s.events_timeline
|   |   |
|   |   +-- inspect cluster pressure via k8s.resource_usage (include nodes)
|   |
|   +-- NO -> continue
|
+-- Pods restarted on one node only?
|   |
|   +-- YES -> run k8s.list (pods by node selector if available)
|   |   |
|   |   +-- inspect impacted workloads with k8s.describe
|   |   |
|   |   +-- verify container errors via k8s.logs
|   |
|   +-- NO -> continue
|
+-- Scheduler reports Insufficient CPU/Memory?
|   |
|   +-- YES -> run k8s.resource_usage
|   |   |
|   |   +-- confirm pod requests/limits via k8s.describe
|   |   |
|   |   +-- inspect autoscaling blockers with k8s.hpa_debug
|   |
|   +-- NO -> continue
|
+-- Node pressure impacts only stateful workloads?
|   |
|   +-- YES -> run k8s.storage_debug
|   |   |
|   |   +-- correlate storage attach failures via k8s.events_timeline
|   |
|   +-- NO -> continue
|
+-- Still uncertain?
    |
    +-- run k8s.diagnose keyword="node pressure"
    |
    +-- run k8s.debug_flow scenario=pending
```

---

## 4) Storage Issues

```text
START: Volume/PVC-related errors
|
+-- Pod event contains FailedScheduling due to PVC?
|   |
|   +-- YES -> run k8s.storage_debug
|   |   |
|   |   +-- inspect PVC state via k8s.describe
|   |   |
|   |   +-- inspect timeline via k8s.events_timeline
|   |
|   +-- NO -> continue
|
+-- PVC stuck Pending?
|   |
|   +-- YES -> run k8s.storage_debug
|   |   |
|   |   +-- check class/capacity/access mode in k8s.describe
|   |   |
|   |   +-- inspect similar PVCs via k8s.list
|   |
|   +-- NO -> continue
|
+-- Pod has FailedMount / FailedAttachVolume?
|   |
|   +-- YES -> run k8s.storage_debug
|   |   |
|   |   +-- identify CSI/node attach issue in k8s.events_timeline
|   |   |
|   |   +-- inspect app error impact in k8s.logs
|   |
|   +-- NO -> continue
|
+-- Volume mounted but app still fails?
|   |
|   +-- YES -> run k8s.logs
|   |   |
|   |   +-- inspect mount details/permissions via k8s.describe
|   |   |
|   |   +-- run k8s.exec for file permission checks when needed
|   |
|   +-- NO -> continue
|
+-- Uncertain sequence?
    |
    +-- run k8s.events_timeline for storage objects
    |
    +-- run k8s.diagnose keyword="failed mount"
```

---

## 5) Deployment Not Progressing

```text
START: Deployment rollout does not complete
|
+-- Are new pods Pending?
|   |
|   +-- YES -> run k8s.scheduling_debug
|   |   |
|   |   +-- follow with k8s.events_timeline
|   |   |
|   |   +-- inspect pressure via k8s.resource_usage
|   |
|   +-- NO -> continue
|
+-- Are new pods CrashLoopBackOff?
|   |
|   +-- YES -> run k8s.crashloop_debug
|   |   |
|   |   +-- inspect startup error via k8s.logs
|   |   |
|   |   +-- inspect object/events via k8s.describe
|   |
|   +-- NO -> continue
|
+-- Are pods Running but not Ready?
|   |
|   +-- YES -> run k8s.logs
|   |   |
|   |   +-- inspect probe and endpoint health via k8s.network_debug
|   |   |
|   |   +-- visualize route chain via k8s.graph
|   |
|   +-- NO -> continue
|
+-- Is app blocked by config or permission at startup?
|   |
|   +-- YES -> run k8s.config_debug or k8s.permission_debug
|   |   |
|   |   +-- verify with k8s.describe and k8s.logs
|   |
|   +-- NO -> continue
|
+-- Is scaling behavior preventing readiness capacity?
|   |
|   +-- YES -> run k8s.hpa_debug
|   |   |
|   |   +-- check resource saturation with k8s.resource_usage
|   |   |
|   |   +-- check VPA recommendations with k8s.vpa_debug
|   |
|   +-- NO -> continue
|
+-- Deployment symptom spans multiple layers?
    |
    +-- run k8s.diagnose keyword="deployment stuck"
    |
    +-- run k8s.debug_flow scenario=crashloop or scenario=traffic
```

---

## Operator Notes

- Prefer scenario debuggers before generic object inspection.
- Use `k8s.events_timeline` to identify the first failure, not just latest symptom.
- Use `k8s.graph` whenever there is an accessibility or routing complaint.
- Use `k8s.diagnose` + `k8s.debug_flow` for layered incidents.
- Keep evidence triad: `k8s.logs` + `k8s.describe` + `k8s.events_timeline`.

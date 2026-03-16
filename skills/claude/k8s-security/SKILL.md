# Skill: k8s-security

Security diagnostics and audit guidance for Kubernetes workloads using RootCause MCP tools only.

## Scope

- RBAC posture and dangerous grants.
- ServiceAccount and IRSA trust-chain analysis.
- Secret and ConfigMap reference validation.
- Policy compliance checks with Kyverno and Cilium context.
- Workload security best-practice review.
- Network isolation and policy-path diagnostics.

## Tooling Rule

- Use RootCause tool names only.
- Use `k8s.list` for resource enumeration (including ClusterRoleBinding and RoleBinding queries).
- Use `k8s.describe` for object-level condition/event detail.
- Do not use non-RootCause tool name variants.

## Canonical Tool Set

### Primary security tools

- `k8s.permission_debug`
- `k8s.config_debug`
- `k8s.best_practice`
- `k8s.kyverno_detect`
- `k8s.diagnose_kyverno`
- `k8s.cilium_detect`
- `k8s.diagnose_cilium`
- `k8s.describe`
- `k8s.list`
- `k8s.network_debug`
- `aws.iam.get_role`
- `aws.iam.get_policy`

### Supporting event evidence

- `k8s.events`
- `k8s.events_timeline`

## Triggers

- rbac denied
- forbidden
- who can access
- cluster-admin usage
- wildcard verbs
- secret exfiltration risk
- pod exec abuse
- serviceaccount hardening
- irsa not working
- sts assume role error
- kyverno policy violation
- cilium deny
- network isolation drift
- compliance audit
- pod security posture

## Priority Rules

| Condition | First action | Why |
|---|---|---|
| Unknown policy stack | Run `k8s.kyverno_detect` and `k8s.cilium_detect` | Scope policy engines before deep triage |
| Permission error in app logs | Run `k8s.permission_debug` | Resolves RBAC and IRSA quickly |
| IRSA suspect | Run `k8s.permission_debug` then `aws.iam.get_role` | Validates K8s and IAM chain |
| Config/secret runtime failure | Run `k8s.config_debug` | Detects missing keys and references |
| Admission denied | Run `k8s.diagnose_kyverno` | Surfaces policy and report failures |
| Network deny suspicion | Run `k8s.network_debug` then `k8s.diagnose_cilium` | Covers K8s policy + Cilium datapath |
| Broad security review | Run `k8s.best_practice` | Produces security-focused workload findings |

## Investigation Workflow

### Phase 1: Baseline security context

1. Run `k8s.kyverno_detect`.
2. Run `k8s.cilium_detect`.
3. Run `k8s.events` for namespace warning events.
4. Run `k8s.events_timeline` to align failures with changes.

### Phase 2: Identity and authorization path

1. Run `k8s.permission_debug` with target namespace and pod/service account.
2. Enumerate bindings with `k8s.list` for Role, RoleBinding, ClusterRole, ClusterRoleBinding.
3. Use `k8s.describe` on suspicious bindings for complete rule context.
4. If IRSA in play, run `aws.iam.get_role` for trust and attached policies.
5. Run `aws.iam.get_policy` for high-risk policy statements.

### Phase 3: Configuration and secret integrity

1. Run `k8s.config_debug` with required keys.
2. Use `k8s.describe` on referenced ConfigMap or Secret objects.
3. Correlate missing/misnamed key failures with rollout events.

### Phase 4: Network isolation and policy behavior

1. Run `k8s.network_debug` on affected service path.
2. Run `k8s.list` for NetworkPolicy objects in scope namespace.
3. If Cilium is present, run `k8s.diagnose_cilium` for endpoint and policy health.

### Phase 5: Workload posture and hardening

1. Run `k8s.best_practice` for deployment/statefulset targets.
2. Map findings to actionable fixes and policy controls.
3. Verify no new warnings in `k8s.events` after change.

## RBAC Audit Framework

### Step 1: Inventory identities

1. List ServiceAccounts with `k8s.list`.
2. Identify high-privilege workloads by namespace criticality.
3. Prioritize CI/CD, operators, and externally exposed workloads.

### Step 2: Enumerate grants

1. List Role and ClusterRole resources with `k8s.list`.
2. List RoleBinding and ClusterRoleBinding resources with `k8s.list`.
3. Detect wildcard grants and cluster-admin bindings.

### Step 3: Evaluate dangerous privileges

Check for:

- Verbs: `*`.
- Resources: `*`.
- Secret read permissions cluster-wide.
- Pod exec/attach permissions.
- Impersonate and escalate verbs.
- Node-level write permissions.

### Step 4: Validate binding scope

- Namespace-local roles should bind namespace-local identities.
- Cluster roles should be justified and minimal.
- Shared service accounts across environments should be avoided.

### Step 5: Produce risk-ranked findings

- Critical: direct cluster takeover paths.
- High: broad secret access and privileged exec controls.
- Medium: oversized namespace admin grants.
- Low: hygiene and observability gaps.

## IRSA Analysis Framework

### Signal chain

1. ServiceAccount annotation present and correct.
2. Role trust policy principal and audience correct.
3. Role policies match required AWS API actions.
4. Pod uses expected ServiceAccount.

### Workflow

1. Run `k8s.permission_debug` for pod/service account.
2. Run `aws.iam.get_role` for trust policy and attachments.
3. Run `aws.iam.get_policy` for each attached custom policy.
4. Confirm least privilege for required AWS operations.

### Frequent IRSA failures

- Wrong role ARN annotation.
- Trust policy mismatch with cluster OIDC provider.
- Missing `sts:AssumeRoleWithWebIdentity` path.
- Policy denies required calls despite role attachment.

## Kyverno Compliance Flow

1. Run `k8s.kyverno_detect`.
2. Run `k8s.diagnose_kyverno`.
3. Use `k8s.describe` on failing policy resources.
4. Use `k8s.events_timeline` to identify rollout windows causing denial bursts.
5. Build fix plan: policy correction or workload manifest correction.

## Cilium Security Flow

1. Run `k8s.cilium_detect`.
2. Run `k8s.diagnose_cilium`.
3. Run `k8s.network_debug` for failing service paths.
4. Enumerate network policy resources with `k8s.list`.
5. Verify post-change path connectivity using the same debug route.

## Security Incident Patterns

| Symptom | Primary tool | Follow-up | Likely root cause |
|---|---|---|---|
| `forbidden` API errors | `k8s.permission_debug` | `k8s.list` | Missing RoleBinding or wrong SA |
| Pod cannot read secret key | `k8s.config_debug` | `k8s.describe` | Missing key or wrong secret name |
| Pod cannot call AWS API | `k8s.permission_debug` | `aws.iam.get_role` | IRSA trust/policy misconfig |
| Admission denied after deploy | `k8s.diagnose_kyverno` | `k8s.describe` | Policy violation |
| Service path denied | `k8s.network_debug` | `k8s.diagnose_cilium` | Policy mismatch |
| Overly privileged CI account | `k8s.list` | `k8s.describe` | Broad cluster role binding |

## Security Checklists

### RBAC minimum checklist

- No unnecessary `cluster-admin` bindings.
- No wildcard resource/verb grants without strong exception.
- Secret access strictly scoped to required namespace.
- `pods/exec` only for audited break-glass identities.
- No stale RoleBindings to deleted SAs.

### Workload hardening checklist

- Run `k8s.best_practice` for each critical deployment.
- Prioritize findings around privilege escalation and runtime hardening.
- Verify container/image constraints align with org policy.

### Policy engine checklist

- `k8s.kyverno_detect` and `k8s.diagnose_kyverno` clean for target namespaces.
- `k8s.cilium_detect` and `k8s.diagnose_cilium` stable.
- Event timelines show no recurring denial loops.

## Remediation Patterns

### Pattern 1: Overprivileged service account

1. Identify binding path with `k8s.permission_debug`.
2. Enumerate exact role grants with `k8s.describe`.
3. Replace broad role with least-privilege role.
4. Re-run `k8s.permission_debug` and verify reduced surface.

### Pattern 2: Wildcard RBAC in CI/CD namespace

1. List all CI/CD bindings with `k8s.list`.
2. Flag wildcard verbs/resources.
3. Scope by namespace and explicit resources.
4. Re-test pipeline and verify no security regressions.

### Pattern 3: Secret sprawl exposure

1. Identify secret read bindings via `k8s.list`.
2. Restrict binding subjects and namespaces.
3. Validate workloads still resolve required keys with `k8s.config_debug`.

### Pattern 4: IRSA access denied

1. Validate SA binding with `k8s.permission_debug`.
2. Validate role trust with `aws.iam.get_role`.
3. Validate action-level permission with `aws.iam.get_policy`.
4. Retry and verify no deny events.

## Output Contract

Always return:

- Security engines detected and their health posture.
- Identity path findings (ServiceAccount, RBAC, IRSA where relevant).
- Specific risky grants and why they are risky.
- Policy and network isolation findings.
- Severity-ranked remediation with verification steps.

## Report Severity Model

- Critical: immediate privilege escalation or broad cluster takeover path.
- High: broad secrets/exec access or cross-namespace exposure.
- Medium: policy misconfig with moderate blast radius.
- Low: hygiene gaps and non-exploitable drift.

## Reference Document

Use `RBAC-PATTERNS.md` for detailed dangerous patterns, secure templates, and audit checklists.

## Related Skills

- `k8s-service-mesh` for identity/mTLS interaction in service-to-service security.
- `k8s-policy` for policy-focused deep dive.

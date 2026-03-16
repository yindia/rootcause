# RBAC Patterns for Kubernetes Security with RootCause Tools

This reference defines dangerous RBAC patterns, secure role templates, and audit methods using RootCause tools.

## RootCause Tools Used

- `k8s.list`
- `k8s.describe`
- `k8s.permission_debug`
- `k8s.best_practice`
- `k8s.events`
- `k8s.events_timeline`
- `k8s.config_debug`
- `k8s.network_debug`
- `aws.iam.get_role`
- `aws.iam.get_policy`

## Goals

- Minimize privilege surface.
- Separate duties by namespace and function.
- Protect secret data and runtime control paths.
- Detect privilege escalation pathways quickly.
- Keep CI/CD access narrow and auditable.

## Dangerous Pattern 1: `cluster-admin` Binding

### Why risky

- Full control over cluster-scoped and namespaced resources.
- Enables credential theft, policy bypass, and workload takeover.

### Detection method

1. Use `k8s.list` to enumerate ClusterRoleBindings.
2. Filter for roleRef to `cluster-admin`.
3. Use `k8s.describe` on each binding to inspect subjects.

### Severity

- Critical if bound to broad groups or CI/CD accounts.

### Safer alternative

- Use namespace-scoped RoleBindings where possible.
- Use dedicated operational break-glass identities with approval controls.

## Dangerous Pattern 2: Wildcard Verbs (`*`)

### Why risky

- Includes mutate/delete/escalate actions not explicitly intended.

### Detection method

1. Use `k8s.list` for Role and ClusterRole resources.
2. Inspect rule blocks for wildcard verbs.
3. Use `k8s.describe` to verify real scope and bindings.

### Severity

- High to Critical depending on resource scope.

### Safer alternative

- Enumerate only required verbs.
- Keep write verbs limited to deployment automation identities.

## Dangerous Pattern 3: Wildcard Resources (`*`)

### Why risky

- Grants implicit access to future resources and CRDs.

### Detection method

1. List roles with `k8s.list`.
2. Identify `resources: ["*"]`.
3. Correlate with bindings to external or multi-tenant identities.

### Severity

- High when coupled with write verbs.

### Safer alternative

- Explicitly enumerate resource kinds required by workload.

## Dangerous Pattern 4: Cluster-Wide Secret Read

### Why risky

- Secret access can reveal tokens, keys, and database credentials.

### Detection method

1. Use `k8s.list` for ClusterRole and Role resources.
2. Search rules containing `secrets` with `get`, `list`, or `watch`.
3. Use `k8s.describe` for binding subjects and namespaces.

### Severity

- Critical if non-admin workload identities can read secrets across namespaces.

### Safer alternative

- Limit secret read to namespace-local service accounts.
- Prefer key-specific secret design and controlled secret projection.

## Dangerous Pattern 5: Pod Exec / Attach Access

### Why risky

- Enables shell access into running workloads.
- Can bypass application-layer controls.

### Detection method

1. Use `k8s.list` for roles including `pods/exec` or `pods/attach`.
2. Use `k8s.describe` to inspect bound subjects.
3. Correlate with incident events via `k8s.events_timeline`.

### Severity

- High for production namespaces.

### Safer alternative

- Restrict exec access to audited SRE break-glass identities.

## Dangerous Pattern 6: Escalate / Bind / Impersonate Verbs

### Why risky

- Direct privilege-escalation capabilities.

### Detection method

1. Use `k8s.list` for ClusterRole and Role resources.
2. Identify verbs `escalate`, `bind`, `impersonate`.
3. Use `k8s.describe` for subject and scope impact.

### Severity

- Critical by default.

### Safer alternative

- Remove these verbs from routine identities.
- Keep only in controlled platform-admin workflows.

## Dangerous Pattern 7: Shared ServiceAccount Across Environments

### Why risky

- Cross-environment blast radius.
- Weak audit attribution.

### Detection method

1. Use `k8s.list` to inventory ServiceAccounts and bindings.
2. Map same SA names across dev/stage/prod namespaces.
3. Use `k8s.permission_debug` on high-risk workloads.

### Safer alternative

- One service account per environment and app boundary.

## Dangerous Pattern 8: CI/CD ServiceAccount with Broad Cluster Writes

### Why risky

- Pipeline compromise becomes cluster compromise.

### Detection method

1. Use `k8s.list` to find CI/CD subject bindings.
2. Use `k8s.describe` for role rule detail.
3. Verify exposed write verbs and cluster-scoped resources.

### Safer alternative

- Namespace-targeted deploy roles with explicit resource lists.

## Dangerous Pattern 9: Unused High-Privilege Bindings

### Why risky

- Dormant privileges become latent attack paths.

### Detection method

1. Use `k8s.list` to inventory all bindings.
2. Use `k8s.events_timeline` for activity signals.
3. Flag stale identities not tied to active workloads.

### Safer alternative

- Remove stale bindings on scheduled cadence.

## Secure Pattern 1: Read-Only Viewer Role

### Characteristics

- Namespaced scope.
- `get`, `list`, `watch` only.
- No secret read by default.

### Validation

1. Use `k8s.describe` on Role.
2. Verify no wildcard write verbs.
3. Confirm binding subjects are intended read-only users/groups.

## Secure Pattern 2: Developer Role

### Characteristics

- Write access to app resources in one namespace.
- No RBAC mutation rights.
- No node, secret, or cluster-scoped mutation.

### Validation

1. Use `k8s.describe` on Role and RoleBinding.
2. Verify resource list excludes privileged endpoints.
3. Confirm no `pods/exec` unless approved.

## Secure Pattern 3: CI/CD Deployment Role

### Characteristics

- Scoped to deployment-related resources.
- Namespace-bound RoleBinding.
- No broad secret read and no cluster-role edits.

### Validation

1. Use `k8s.list` to enumerate CI/CD bindings.
2. Use `k8s.describe` to verify tight resource/verb scopes.
3. Use `k8s.permission_debug` to confirm effective permissions.

## Secure Pattern 4: Namespace Admin Role

### Characteristics

- Full namespace administration.
- No cluster-scoped control.
- Break-glass process for elevated operations.

### Validation

1. Use `k8s.describe` on namespace-admin role.
2. Ensure no cluster-scoped resources are granted.

## Secure Pattern 5: IRSA-Backed ServiceAccount with Least-Privilege IAM

### Characteristics

- ServiceAccount mapped to one IAM role.
- IAM trust policy restricted to correct OIDC subject.
- IAM permissions limited to workload needs.

### Validation

1. Run `k8s.permission_debug`.
2. Run `aws.iam.get_role`.
3. Run `aws.iam.get_policy` for attached policies.

## Audit Checklist

### Identity inventory

- List all ServiceAccounts via `k8s.list`.
- Identify external identities and CI/CD principals.

### RBAC inventory

- List all Role/RoleBinding/ClusterRole/ClusterRoleBinding via `k8s.list`.
- Flag cluster-admin and wildcard usage.

### Permission verification

- Run `k8s.permission_debug` for critical workloads.
- Confirm SA, roles, and bindings match intent.

### Secret and config exposure

- Run `k8s.config_debug` on critical workloads.
- Verify only required keys are referenced.

### Network and policy context

- Run `k8s.network_debug` for sensitive services.
- Run `k8s.events` and `k8s.events_timeline` for denial and drift patterns.

### Workload hardening

- Run `k8s.best_practice` on high-risk workloads.
- Track unresolved high/critical findings.

## Detection Queries by Intent

### Find cluster-admin usage

1. `k8s.list` for `ClusterRoleBinding`.
2. `k8s.describe` for each candidate binding.

### Find wildcard grants

1. `k8s.list` for `Role` and `ClusterRole`.
2. `k8s.describe` and inspect rule arrays.

### Find secret-read risk

1. `k8s.list` roles and cluster roles.
2. `k8s.describe` for resources including `secrets`.

### Find pod exec risk

1. `k8s.list` roles and cluster roles.
2. `k8s.describe` for `pods/exec` and `pods/attach`.

### Verify IRSA chain

1. `k8s.permission_debug`.
2. `aws.iam.get_role`.
3. `aws.iam.get_policy`.

## Remediation Sequence

1. Remove or narrow critical bindings first.
2. Replace wildcard rules with explicit resources and verbs.
3. Segment ServiceAccounts by namespace and workload.
4. Restrict secret read paths.
5. Harden CI/CD identities.
6. Re-run detection checks.

## Reporting Format

For each finding, include:

- Identity subject.
- Role and binding path.
- Risk reason.
- Blast radius.
- Recommended least-privilege replacement.
- Verification steps and expected result.

## Common False Assumptions

- "Namespaced binding cannot be dangerous".
- "Read-only secret access is safe".
- "CI/CD needs cluster-wide write by default".
- "Temporary cluster-admin bindings are harmless".
- "IRSA role attachment alone guarantees secure access".

## Ongoing Governance Recommendations

- Run RBAC audits on a fixed schedule.
- Enforce peer review for role and binding changes.
- Maintain exception registry with expiration.
- Tie high-risk RBAC changes to change-management approvals.
- Track repeat offenders and convert to policy controls.

## Related Skill

- `SKILL.md` in `k8s-security` for full investigation workflow and output contract.

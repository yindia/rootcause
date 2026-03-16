# Skill: k8s-helm

Helm lifecycle operations for Kubernetes using RootCause MCP tool names only.

This skill is evidence-first and safety-first.
`helm.diff_release` is mandatory before any mutating action.

## Purpose

Use this skill for:
- repository setup,
- chart discovery and version selection,
- release install and upgrade,
- rollback planning and recovery,
- uninstall and cleanup validation,
- template apply/uninstall workflows.

## Strict Tooling Contract

Use only these Helm tool names:
- `helm.repo_add`
- `helm.repo_list`
- `helm.repo_update`
- `helm.list_charts`
- `helm.get_chart`
- `helm.search_charts`
- `helm.list`
- `helm.status`
- `helm.diff_release`
- `helm.rollback_advisor`
- `helm.install`
- `helm.upgrade`
- `helm.uninstall`
- `helm.template_apply`
- `helm.template_uninstall`

Use these Kubernetes inspection tools when validating releases:
- `k8s.describe`
- `k8s.list`
- `k8s.events`
- `k8s.events_timeline`
- `k8s.get`

Never emit kubectl-alias tool names.
Never emit non-RootCause Helm naming variants.

## Triggers

Enable when user intent includes:
- install chart,
- upgrade release,
- rollback failed deployment,
- resolve Helm release errors,
- inspect chart versions,
- remove a release,
- debug values not applying.

Common terms:
- helm, chart, release, values, repository,
- pending-install, pending-upgrade, failed,
- cannot re-use a name,
- immutable selector,
- another operation in progress.

## Priority Rules

1. State first, mutation later.
- Start with `helm.list` or `helm.status`.

2. Diff before every mutating operation.
- Run `helm.diff_release` before:
  - `helm.install`
  - `helm.upgrade`
  - `helm.uninstall`
  - `helm.template_apply`
  - `helm.template_uninstall`

3. Recovery planning before rollback.
- Run `helm.rollback_advisor` for safer target selection.

4. Confirmation requirements are mandatory.
- Use `confirm=true` for tools that require it.

5. Post-action verification is mandatory.
- Validate with `helm.status` and `k8s.*` evidence tools.

## Quick Reference: 15 Helm Tools

| Tool | Purpose | Typical Use |
|---|---|---|
| `helm.repo_add` | Add or update a Helm repository | Repo bootstrap |
| `helm.repo_list` | List configured repositories | Repo preflight |
| `helm.repo_update` | Refresh repo indexes | Resolve stale cache |
| `helm.list_charts` | List charts from configured repos | Catalog browsing |
| `helm.get_chart` | Retrieve chart details and versions | Version selection |
| `helm.search_charts` | Search charts across repos | Find chart quickly |
| `helm.list` | List Helm releases | Inventory current state |
| `helm.status` | Show release status and notes | Health and failure triage |
| `helm.diff_release` | Diff live release against target render | Mandatory risk review |
| `helm.rollback_advisor` | Recommend safer rollback targets | Recovery planning |
| `helm.install` | Install chart release | New deployment |
| `helm.upgrade` | Upgrade existing release | Change rollout |
| `helm.uninstall` | Remove release-managed resources | Decommission |
| `helm.template_apply` | Render and apply manifests | Controlled apply path |
| `helm.template_uninstall` | Render and delete manifests | Controlled delete path |

## Workflow 1: Repository Setup

1. Check configured repos with `helm.repo_list`.
2. Add missing source with `helm.repo_add`.
3. Refresh index with `helm.repo_update`.
4. Verify chart visibility with `helm.search_charts`.
5. Validate chart metadata and versions with `helm.get_chart`.

## Workflow 2: Chart Discovery

1. Use `helm.search_charts` to locate chart candidates.
2. Use `helm.list_charts` to inspect repo catalog when needed.
3. Use `helm.get_chart` to compare versions.
4. Select explicit version for production stability.
5. Document namespace and release naming convention.

## Workflow 3: Install Release

1. Preflight current namespace state:
- `helm.list`
- `k8s.list`

2. Validate chart/version:
- `helm.get_chart`

3. Diff required:
- `helm.diff_release`

4. Execute install:
- `helm.install` with `confirm=true`

5. Verify deployment:
- `helm.status`
- `k8s.events`
- `k8s.describe` for any unhealthy workload

## Workflow 4: Upgrade Release

1. Capture baseline:
- `helm.status`
- `k8s.list`

2. Validate target chart:
- `helm.get_chart`

3. Mandatory change preview:
- `helm.diff_release`

4. Execute change:
- `helm.upgrade` with `confirm=true`

5. Validate rollout:
- `helm.status`
- `k8s.events_timeline`
- `k8s.describe` on failed objects

6. If degraded:
- `helm.rollback_advisor`
- perform recovery using controlled `helm.upgrade`

## Workflow 5: Rollback and Recovery

1. Gather evidence:
- `helm.status`
- `k8s.events`
- `k8s.describe`

2. Identify lower-risk rollback target:
- `helm.rollback_advisor`

3. Compare target state:
- `helm.diff_release`

4. Apply recovery:
- typically `helm.upgrade` with known-safe version/values and `confirm=true`

5. Confirm stability:
- `helm.status`
- `k8s.events_timeline`
- `k8s.get` for critical resources

## Workflow 6: Uninstall Release

1. Confirm target release:
- `helm.status`

2. Inspect impacted resources:
- `k8s.list`

3. Preview changes:
- `helm.diff_release`

4. Execute removal:
- `helm.uninstall` with `confirm=true`

5. Validate cleanup:
- `helm.list`
- `k8s.events`
- `k8s.describe` for stuck finalizers or hooks

## Workflow 7: Template Apply and Template Uninstall

Apply path:
1. Validate chart with `helm.get_chart`.
2. Run `helm.diff_release`.
3. Execute `helm.template_apply` with `confirm=true`.
4. Verify via `helm.status` and `k8s.list`.

Uninstall path:
1. Preview with `helm.diff_release`.
2. Execute `helm.template_uninstall` with `confirm=true`.
3. Verify with `helm.list` and `k8s.events_timeline`.

## Common Failure Patterns

- `cannot re-use a name`
  - Existing release conflict. Check `helm.list` and namespace.

- timeout during install or upgrade
  - Check `helm.status`, then `k8s.events` and `k8s.describe`.

- resource already exists
  - Ownership collision. Inspect with `k8s.get` and `k8s.describe`.

- another operation in progress
  - Release lock state. Validate with `helm.status`; avoid concurrent mutation.

- immutable selector change
  - Usually discovered in `helm.diff_release`; redesign migration strategy.

## Troubleshooting and Authoring References

Use these two docs with this skill:
- `skills/claude/k8s-helm/TROUBLESHOOTING.md`
- `skills/claude/k8s-helm/references/CHART-STRUCTURE.md`

## Output Contract

Every execution summary should include:
1. RootCause tool sequence used.
2. `helm.diff_release` result before any mutation.
3. Mutation action and confirmation context.
4. Final `helm.status` result.
5. Kubernetes evidence summary from `k8s.events` or `k8s.events_timeline`.
6. Next safe action if release is still degraded.

## Safety Checklist

- Repo readiness confirmed (`helm.repo_list`, `helm.repo_add`, `helm.repo_update`)
- Chart/version validated (`helm.get_chart`)
- Diff reviewed (`helm.diff_release`)
- Mutation executed with explicit confirmation requirement
- Post-action verification complete (`helm.status`, `k8s.*` evidence)
- Recovery strategy available (`helm.rollback_advisor`)

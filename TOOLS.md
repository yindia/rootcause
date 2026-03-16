# RootCause Tools

This document lists all tools exposed by RootCause. For setup and usage, see `README.md`.

## Quick Tool Picker

- Traffic errors (5xx/timeout): `k8s.debug_flow` scenario `traffic`, then `k8s.network_debug`, `istio.*` or `linkerd.*` as needed.
- Pending pods: `k8s.debug_flow` scenario `pending`, or `k8s.scheduling_debug` + `k8s.storage_debug`.
- CrashLoopBackOff: `k8s.debug_flow` scenario `crashloop`, or `k8s.crashloop_debug` + `k8s.config_debug`.
- Autoscaling issues: `k8s.hpa_debug` / `k8s.vpa_debug` + `k8s.resource_usage`.
- Volume/PVC failures: `k8s.storage_debug`.
- Missing config keys: `k8s.config_debug`.
- Permission errors (Forbidden/AccessDenied): `k8s.permission_debug` or `k8s.debug_flow` scenario `permission`.
- EKS auth/KMS/ECR issues: `aws.eks.debug` (include STS/KMS/ECR checks as needed).

---

## Toolchains and Tools

### Core Kubernetes (`k8s.*` + kubectl-style aliases)
- CRUD + discovery: `k8s.get`, `k8s.list`, `k8s.describe`, `k8s.create`, `k8s.apply`, `k8s.patch`, `k8s.delete`, `k8s.api_resources`, `k8s.crds`
- Ops + observability: `k8s.logs`, `k8s.events`, `k8s.context`, `k8s.explain_resource`, `k8s.ping`
- Timeline: `k8s.events_timeline`
- Workload operations: `k8s.scale`, `k8s.rollout`
- Restart preflight: `k8s.restart_safety_check`
- Workload standards: `k8s.best_practice` (rollout safety, restart resilience, node-recreate spread, PVC attach/detach risk)
- Ecosystem detection: `k8s.argocd_detect`, `k8s.flux_detect`, `k8s.cert_manager_detect`, `k8s.kyverno_detect`, `k8s.cilium_detect`
- Ecosystem diagnostics: `k8s.diagnose_argocd`, `k8s.diagnose_flux`, `k8s.diagnose_cert_manager`, `k8s.diagnose_kyverno`, `k8s.diagnose_cilium`
- Mutation guardrails: `k8s.safe_mutation_preflight` (used automatically before `k8s.create`, `k8s.apply`, `k8s.patch`, `k8s.delete`, `k8s.scale`, `k8s.rollout`, `k8s.cleanup_pods`, `k8s.node_management`; also available for explicit dry-run checks)
- Execution and access: `k8s.exec`, `k8s.exec_readonly` (allowlisted), `k8s.port_forward`
- Debugging: `k8s.overview`, `k8s.crashloop_debug`, `k8s.scheduling_debug`, `k8s.hpa_debug`, `k8s.vpa_debug`, `k8s.storage_debug`, `k8s.config_debug`, `k8s.permission_debug`, `k8s.network_debug`, `k8s.private_link_debug`, `k8s.debug_flow`
- Maintenance: `k8s.cleanup_pods`, `k8s.node_management`
- Graph and topology: `k8s.graph` (Ingress/Service/Endpoints/Workloads + mesh + NetworkPolicy)
- Metrics: `k8s.resource_usage` (metrics-server)

### Graph-first debugging

Use `k8s.debug_flow` to run a guided flow that builds `k8s.graph` and walks the dependency chain node-by-node.

### Linkerd (`linkerd.*`)
- `linkerd.health`, `linkerd.proxy_status`, `linkerd.identity_issues`, `linkerd.policy_debug`, `linkerd.cr_status`
- `linkerd.virtualservice_status`, `linkerd.destinationrule_status`, `linkerd.gateway_status`, `linkerd.httproute_status`

### Istio (`istio.*`)
- `istio.health`, `istio.proxy_status`, `istio.config_summary`
- `istio.service_mesh_hosts`, `istio.discover_namespaces`, `istio.pods_by_service`, `istio.external_dependency_check`
- `istio.proxy_clusters`, `istio.proxy_listeners`, `istio.proxy_routes`, `istio.proxy_endpoints`, `istio.proxy_bootstrap`, `istio.proxy_config_dump`
- `istio.cr_status`, `istio.virtualservice_status`, `istio.destinationrule_status`, `istio.gateway_status`, `istio.httproute_status`

### Karpenter (`karpenter.*`)
- `karpenter.status`, `karpenter.node_provisioning_debug`
- `karpenter.nodepool_debug`, `karpenter.nodeclass_debug`, `karpenter.interruption_debug`

### Helm (`helm.*`)
- `helm.repo_add`, `helm.repo_list`, `helm.repo_update`
- `helm.list_charts`, `helm.get_chart`, `helm.search_charts`
- `helm.list`, `helm.status`, `helm.diff_release`
- `helm.rollback_advisor`
- `helm.install`, `helm.upgrade`, `helm.uninstall`
- `helm.template_apply`, `helm.template_uninstall`

`helm.list_charts`, `helm.get_chart`, and `helm.search_charts` use Artifact Hub by default (`https://artifacthub.io`). You can override with `artifactHubURL` when needed.

### AWS IAM (`aws.iam.*`)
- `aws.iam.list_roles`, `aws.iam.get_role`, `aws.iam.get_instance_profile`
- `aws.iam.update_role`, `aws.iam.delete_role` (confirm required)
- `aws.iam.list_policies`, `aws.iam.get_policy`
- `aws.iam.update_policy`, `aws.iam.delete_policy` (confirm required)

### AWS VPC (`aws.vpc.*`)
- `aws.vpc.list_vpcs`, `aws.vpc.get_vpc`
- `aws.vpc.list_subnets`, `aws.vpc.get_subnet`
- `aws.vpc.list_route_tables`, `aws.vpc.get_route_table`
- `aws.vpc.list_nat_gateways`, `aws.vpc.get_nat_gateway`
- `aws.vpc.list_security_groups`, `aws.vpc.get_security_group`
- `aws.vpc.list_network_acls`, `aws.vpc.get_network_acl`
- `aws.vpc.list_internet_gateways`, `aws.vpc.get_internet_gateway`
- `aws.vpc.list_vpc_endpoints`, `aws.vpc.get_vpc_endpoint`
- `aws.vpc.list_network_interfaces`, `aws.vpc.get_network_interface`
- `aws.vpc.list_resolver_endpoints`, `aws.vpc.get_resolver_endpoint`
- `aws.vpc.list_resolver_rules`, `aws.vpc.get_resolver_rule`

### AWS EC2 (`aws.ec2.*`)
- `aws.ec2.list_instances`, `aws.ec2.get_instance`
- `aws.ec2.list_auto_scaling_groups`, `aws.ec2.get_auto_scaling_group`
- `aws.ec2.list_load_balancers`, `aws.ec2.get_load_balancer`
- `aws.ec2.list_target_groups`, `aws.ec2.get_target_group`
- `aws.ec2.list_listeners`, `aws.ec2.get_listener`
- `aws.ec2.get_target_health`
- `aws.ec2.list_listener_rules`, `aws.ec2.get_listener_rule`
- `aws.ec2.list_auto_scaling_policies`, `aws.ec2.get_auto_scaling_policy`
- `aws.ec2.list_scaling_activities`, `aws.ec2.get_scaling_activity`
- `aws.ec2.list_launch_templates`, `aws.ec2.get_launch_template`
- `aws.ec2.list_launch_configurations`, `aws.ec2.get_launch_configuration`
- `aws.ec2.get_instance_iam`
- `aws.ec2.get_security_group_rules`
- `aws.ec2.list_spot_instance_requests`, `aws.ec2.get_spot_instance_request`
- `aws.ec2.list_capacity_reservations`, `aws.ec2.get_capacity_reservation`
- `aws.ec2.list_volumes`, `aws.ec2.get_volume`
- `aws.ec2.list_snapshots`, `aws.ec2.get_snapshot`
- `aws.ec2.list_volume_attachments`
- `aws.ec2.list_placement_groups`, `aws.ec2.get_placement_group`
- `aws.ec2.list_instance_status`, `aws.ec2.get_instance_status`

### AWS EKS (`aws.eks.*`)
- `aws.eks.list_clusters`, `aws.eks.get_cluster`
- `aws.eks.list_nodegroups`, `aws.eks.get_nodegroup`
- `aws.eks.list_addons`, `aws.eks.get_addon`
- `aws.eks.list_fargate_profiles`, `aws.eks.get_fargate_profile`
- `aws.eks.list_identity_provider_configs`, `aws.eks.get_identity_provider_config`
- `aws.eks.list_updates`, `aws.eks.get_update`
- `aws.eks.list_nodes`
- `aws.eks.debug` (optional STS/KMS/ECR diagnostics)

### AWS ECR (`aws.ecr.*`)
- `aws.ecr.list_repositories`, `aws.ecr.describe_repository`
- `aws.ecr.list_images`, `aws.ecr.describe_images`
- `aws.ecr.describe_registry`
- `aws.ecr.get_authorization_token` (confirm required)

### AWS STS (`aws.sts.*`)
- `aws.sts.get_caller_identity`
- `aws.sts.assume_role` (confirm required)

### AWS KMS (`aws.kms.*`)
- `aws.kms.list_keys`, `aws.kms.list_aliases`
- `aws.kms.describe_key`, `aws.kms.get_key_policy`

### Terraform (`terraform.*`)
- `terraform.debug_plan`
- `terraform.list_modules`, `terraform.get_module`, `terraform.list_module_versions`, `terraform.search_modules`
- `terraform.list_providers`, `terraform.get_provider`, `terraform.list_provider_versions`, `terraform.get_provider_package`, `terraform.search_providers`
- `terraform.list_resources`, `terraform.get_resource`, `terraform.search_resources`
- `terraform.list_data_sources`, `terraform.get_data_source`, `terraform.search_data_sources`

### RootCause (`rootcause.*`)
- `rootcause.incident_bundle`
- `rootcause.change_timeline`
- `rootcause.rca_generate`
- `rootcause.remediation_playbook`
- `rootcause.postmortem_export`

`rootcause.incident_bundle` supports chain orchestration via `toolChain` (array of `{tool, section, args}`), with `includeDefaultChain`, `continueOnError`, and `maxSteps` controls.
Use `outputMode="timeline"` on `rootcause.incident_bundle` to get unified timeline output; `rootcause.change_timeline` is kept for compatibility.

---

## Safety Allowlist (allow_destructive_tools)

When `disable_destructive = true`, RootCause removes destructive/risky tools from discovery. Use `allow_destructive_tools` to re-enable specific tools.

Example:
```toml
[safety]
allow_destructive_tools = [
  "k8s.apply",
  "k8s.patch",
  "k8s.delete",
  "helm.install",
  "helm.upgrade",
  "helm.uninstall"
]
```

### Mutating Tools (Explicit List)

Use this section as the source of truth for tools that can mutate state.

Default policy:
- If the user does not explicitly ask for mutation, use read-only tools only.
- Do not implicitly call mutating tools during investigation.

- `write` (mutating, but not filtered by `disable_destructive`; filtered by `read_only`):
  - `k8s.create`, `k8s.scale`, `k8s.rollout`
  - `kubectl_create`, `kubectl_scale`, `kubectl_rollout`
  - `helm.repo_add`, `helm.repo_update`

- `risky_write` (mutating and filtered when `disable_destructive=true`; allowlist with `allow_destructive_tools`):
  - `k8s.apply`, `k8s.patch`, `k8s.exec`, `k8s.exec_readonly`
  - `kubectl_apply`, `kubectl_patch`
  - `helm.install`, `helm.upgrade`, `helm.template_apply`
  - `aws.iam.update_role`, `aws.iam.update_policy`
  - `aws.sts.assume_role`, `aws.ecr.get_authorization_token`

- `destructive` (mutating and filtered when `disable_destructive=true`; allowlist with `allow_destructive_tools`):
  - `k8s.delete`, `k8s.cleanup_pods`, `k8s.node_management`
  - `kubectl_delete`
  - `helm.uninstall`, `helm.template_uninstall`
  - `aws.iam.delete_role`, `aws.iam.delete_policy`

If you want users to explicitly opt in, set `disable_destructive=true` and include only the approved `risky_write` and `destructive` tool names in `allow_destructive_tools`.

---

## Kubectl-style aliases

The `k8s.*` tools also expose aliases like `kubectl_get`, `kubectl_list`, `kubectl_describe`, `kubectl_create`, `kubectl_apply`, `kubectl_delete`, `kubectl_logs`, `kubectl_patch`, `kubectl_scale`, `kubectl_rollout`, `kubectl_context`, `kubectl_generic`, `kubectl_top`, `explain_resource`, `list_api_resources`, and `ping`.

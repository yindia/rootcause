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

---

## Toolchains and Tools

### Core Kubernetes (`k8s.*` + kubectl-style aliases)
- CRUD + discovery: `k8s.get`, `k8s.list`, `k8s.describe`, `k8s.create`, `k8s.apply`, `k8s.patch`, `k8s.delete`, `k8s.api_resources`, `k8s.crds`
- Ops + observability: `k8s.logs`, `k8s.events`, `k8s.context`, `k8s.explain_resource`, `k8s.ping`
- Workload operations: `k8s.scale`, `k8s.rollout`
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
- `helm.list`, `helm.status`
- `helm.install`, `helm.upgrade`, `helm.uninstall`
- `helm.template_apply`, `helm.template_uninstall`

### AWS IAM (`aws.iam.*`)
- `aws.iam.list_roles`, `aws.iam.get_role`
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

---

## Kubectl-style aliases

The `k8s.*` tools also expose aliases like `kubectl_get`, `kubectl_list`, `kubectl_describe`, `kubectl_create`, `kubectl_apply`, `kubectl_delete`, `kubectl_logs`, `kubectl_patch`, `kubectl_scale`, `kubectl_rollout`, `kubectl_context`, `kubectl_generic`, `kubectl_top`, `explain_resource`, `list_api_resources`, and `ping`.

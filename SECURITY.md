# Security Policy

## Reporting a Vulnerability

If you believe you have found a security issue, please open a private issue or contact the maintainers directly.

## Data Handling

- Secret values are never returned. Secret `data` and `stringData` are redacted.
- Token-like strings are redacted from outputs and logs.
- Audit logs are structured JSON and include tool name, user ID, namespaces/resources, and outcome.
- ConfigMap values are returned as-is unless they match token-like patterns.

## Access Control

RootCause uses your kubeconfig identity in this version; local API-key auth is not enabled.

## Safety Modes

- `--read-only` removes all write and exec tools from discovery.
- `--disable-destructive` removes delete and risky write tools unless allowlisted.

## Least Privilege Guidance

- Namespace-scoped users can access only their allowed namespaces; cluster-scoped resources are blocked.
- Some diagnostics (node metrics, StorageClass/PV/VolumeAttachment, NodeClaims) require cluster role permissions.
- `k8s.exec` and write tools require explicit confirmation and should be restricted in shared environments.

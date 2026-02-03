# Security Policy

## Reporting a Vulnerability

If you believe you have found a security issue, please open a private issue or contact the maintainers directly.

## Data Handling

- Secret values are never returned. Secret `data` and `stringData` are redacted.
- Token-like strings are redacted from outputs and logs.
- Audit logs are structured JSON and include tool name, user ID, namespaces/resources, and outcome.

## Access Control

RootCause uses your kubeconfig identity in this version; local API-key auth is not enabled.

## Safety Modes

- `--read-only` removes all write and exec tools from discovery.
- `--disable-destructive` removes delete and risky write tools unless allowlisted.

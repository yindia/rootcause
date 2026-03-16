# Skill: k8s-certs

Comprehensive cert-manager diagnostics for certificate lifecycle, issuer health, renewal failures,
and ingress TLS integration.

## Trigger Phrases

Use this skill when the user mentions:
- cert-manager broken
- certificate not ready
- tls secret missing
- x509 expired or unknown authority
- renewal failed
- letsencrypt challenge failed
- clusterissuer not ready
- ingress tls handshake error
- certificate request stuck

## RootCause Tools Allowed

Only use these tool names in this skill:
- `k8s.cert_manager_detect`
- `k8s.diagnose_cert_manager`
- `k8s.describe`
- `k8s.list`
- `k8s.events`
- `k8s.events_timeline`

## Cert-Manager Workflow Overview

Canonical path:
1. detect installation
2. diagnose control-plane and resource status
3. inspect failing certificate and issuer
4. validate event timeline for renewal attempt
5. map to ingress/secret consumption impact

## Certificate Lifecycle Reference

Lifecycle checkpoints:
1. `Certificate` resource created.
2. `CertificateRequest` generated.
3. Issuer signs request.
4. Secret populated/updated.
5. Workloads consume updated secret.
6. Renewal happens before expiry window.

Common lifecycle states:
- Ready true
- Ready false (temporary)
- Ready false (terminal misconfig)

## Issuer Types

### Let's Encrypt (ACME)

Typical dependencies:
- valid account registration
- challenge solver (HTTP-01 or DNS-01)
- ingress/dns routing correctness

Failure hotspots:
- challenge path not reachable
- DNS mismatch for hostname
- rate limits from ACME provider

### Self-signed

Use cases:
- internal service encryption
- development/test clusters

Failure hotspots:
- trust chain not distributed to clients
- accidental production usage without trust setup

### CA-backed internal issuer

Use cases:
- private PKI integration
- enterprise trust chains

Failure hotspots:
- missing or invalid CA secret
- expired intermediate CA

## Detection and Baseline

### Step 1: Confirm cert-manager footprint

Use `k8s.cert_manager_detect` first.

If not detected:
- stop diagnosis,
- report "cert-manager not installed or not discoverable".

### Step 2: Run cert-manager diagnosis

Use `k8s.diagnose_cert_manager`:
```yaml
namespace: cert-manager
limit: 100
```

Extract:
- controller readiness
- certificate health summary
- issuer readiness summary
- warning events

## Detailed Troubleshooting Workflow

### Path A: Certificate Not Ready

1. `k8s.list` Certificates in target namespace.
2. `k8s.describe` failing Certificate.
3. `k8s.list` CertificateRequests in same namespace.
4. `k8s.describe` related CertificateRequest.
5. `k8s.events_timeline` for ordered failure trace.

`k8s.list` example:
```yaml
namespace: payments
resources:
  - kind: Certificate
  - kind: CertificateRequest
```

### Path B: Issuer Not Ready

1. `k8s.list` Issuer and ClusterIssuer objects.
2. `k8s.describe` failing issuer.
3. `k8s.events` scoped to issuer object.
4. rerun `k8s.diagnose_cert_manager` to validate cluster-wide health.

### Path C: Renewal Failure

1. Identify cert near expiry from Certificate conditions.
2. `k8s.events_timeline` for that certificate and request chain.
3. validate challenge and order resources via `k8s.list`/`k8s.describe` if present.
4. confirm whether secret update happened.

### Path D: Ingress TLS Breakage

1. `k8s.describe` Ingress (outside this skill if needed) for TLS secret reference.
2. `k8s.describe` referenced Certificate and Secret owner references.
3. check cert-manager events around secret updates.
4. identify if ingress points to wrong secret name/namespace.

## Parameter Guidance

### `k8s.diagnose_cert_manager`

- `namespace`: usually `cert-manager` or control-plane namespace.
- `limit`: increase when many warnings are expected.

### `k8s.events_timeline`

Use object filters for high signal:
```yaml
namespace: payments
involvedObjectKind: Certificate
involvedObjectName: payments-tls
includeNormal: false
limit: 200
```

### `k8s.events`

Use for short-window warning scan:
```yaml
namespace: cert-manager
```

## YAML Examples

### Certificate with ClusterIssuer

```yaml
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: payments-tls
  namespace: payments
spec:
  secretName: payments-tls-secret
  dnsNames:
    - payments.example.com
  issuerRef:
    kind: ClusterIssuer
    name: letsencrypt-prod
```

### ACME ClusterIssuer (HTTP-01)

```yaml
apiVersion: cert-manager.io/v1
kind: ClusterIssuer
metadata:
  name: letsencrypt-prod
spec:
  acme:
    email: sre@example.com
    server: https://acme-v02.api.letsencrypt.org/directory
    privateKeySecretRef:
      name: letsencrypt-prod-account-key
    solvers:
      - http01:
          ingress:
            class: nginx
```

### Self-signed issuer

```yaml
apiVersion: cert-manager.io/v1
kind: Issuer
metadata:
  name: selfsigned-issuer
  namespace: payments
spec:
  selfSigned: {}
```

## Common Errors Table

| Symptom/Error | Likely Cause | Confirm With | Fix Direction |
|---|---|---|---|
| `Certificate not ready` | issuer unavailable or challenge failed | `k8s.describe` Certificate + events | fix issuer/challenge configuration |
| `Issuer not ready` | missing secret or invalid config | `k8s.describe` Issuer/ClusterIssuer | repair secret/config reference |
| `Order failed` | ACME challenge unreachable | `k8s.events_timeline` + related resources | fix DNS/ingress solver path |
| `x509 has expired` | renewal failed before deadline | timeline + cert conditions | force reissue after correcting root cause |
| TLS secret missing | secret creation/update failed | `k8s.describe` Certificate + events | recover issuance and ensure secret name alignment |
| renewed cert not used by app | consumer not reloading secret | workload behavior + restart path | trigger safe rollout/reload |

## Renewal Failure Deep Checks

When renewal repeatedly fails:
1. confirm issuer readiness state,
2. validate DNS and ingress route for challenge hostname,
3. inspect event ordering for retried attempts,
4. check for rate limit evidence,
5. verify target secret update timestamps.

## Output Contract

Always return:
1. detection result (`cert-manager present` or not),
2. failing certificates with reason,
3. issuer health summary,
4. renewal timeline highlights,
5. remediation and verification steps.

Example:
```text
Root cause: ClusterIssuer letsencrypt-prod is NotReady due invalid account key secret reference.
Evidence: k8s.describe ClusterIssuer shows Ready=False and secret lookup failure; k8s.events_timeline shows repeated issuance retries.
Fix: restore account key secret and trigger certificate re-issuance.
Verify: certificate Ready=True, secret updated, ingress serves valid certificate chain.
```

## Completion Criteria

Certificate investigation is complete when:
- failing cert and issuer state are both explained,
- timeline evidence confirms sequence,
- ingress/consumer impact is identified,
- fix path and validation checks are explicit.

# Skill: k8s-browser

Browser automation skill for Kubernetes-facing UIs, ingress validation, dashboard evidence capture,
interactive troubleshooting, and incident artifact export.

## Prerequisites

Before using this skill, ensure:
- browser tools are enabled in RootCause runtime.
- target URL is reachable from browser execution environment.
- required credentials are available for login flows.
- if SSO is used, allow extra waiting and redirect checks.

If browser tools are disabled, report it immediately and continue with non-browser alternatives only when requested.

## Trigger Phrases

Use this skill when the user mentions:
- test ingress URL
- take screenshot
- capture grafana dashboard
- check kubernetes dashboard UI
- validate argocd UI
- reproduce ui bug
- automate login form
- export page as pdf
- verify web health check
- gather visual evidence

## Browser Tool Catalog (All 26)

### Session and navigation

| Tool | Purpose | Typical Parameters |
|---|---|---|
| `browser_open` | Open URL in session | `url`, optional `session` |
| `browser_new_tab` | Create new tab | optional `url`, `session` |
| `browser_switch_tab` | Switch active tab | `tab`, `session` |
| `browser_close_tab` | Close selected tab | optional `tab`, `session` |
| `browser_close` | Close browser session | optional `session` |

### Validation and waiting

| Tool | Purpose | Typical Parameters |
|---|---|---|
| `browser_health_check` | URL check with optional text assertion | `url`, optional `contains` |
| `browser_test_ingress` | Ingress URL validation with evidence | `url` |
| `browser_wait_for` | Wait for selector to appear | `selector` |
| `browser_wait_for_url` | Wait for exact/expected URL | `url` |

### Interaction

| Tool | Purpose | Typical Parameters |
|---|---|---|
| `browser_click` | Click element | `selector` |
| `browser_fill` | Fill input directly | `selector`, `text` |
| `browser_type` | Type text events into input | `selector`, `text` |
| `browser_press` | Keyboard key action | `key` |
| `browser_select` | Select option in dropdown | `selector`, `value` |
| `browser_hover` | Hover element | `selector` |
| `browser_check` | Check checkbox/radio | `selector` |
| `browser_uncheck` | Uncheck checkbox | `selector` |
| `browser_upload` | Upload file | `selector`, `path` |
| `browser_drag` | Drag and drop | `source`, `target` |

### Extraction and analysis

| Tool | Purpose | Typical Parameters |
|---|---|---|
| `browser_snapshot` | Accessibility tree / page snapshot | optional `session` |
| `browser_get_text` | Extract text from selector | `selector` |
| `browser_get_html` | Extract HTML from selector or full page | optional `selector` |
| `browser_evaluate` | Execute JavaScript expression | `expression` |

### Evidence export

| Tool | Purpose | Typical Parameters |
|---|---|---|
| `browser_screenshot` | Capture screenshot | optional `path` |
| `browser_screenshot_grafana` | Open Grafana URL and capture screenshot | `url`, optional `path` |
| `browser_pdf` | Export current page to PDF | optional `path` |

## Priority Rules

| Scenario | Best First Tool | Why |
|---|---|---|
| Quick ingress smoke test | `browser_test_ingress` | Faster than full interaction flow |
| Need assertion that page contains expected text | `browser_health_check` | Built-in contains validation |
| Need Grafana evidence | `browser_screenshot_grafana` | Purpose-built capture path |
| Unknown UI structure | `browser_snapshot` | Safe element discovery without guessing selectors |
| Form interaction/login | `browser_fill` + `browser_click` + waits | deterministic workflow |
| Incident artifact package | `browser_screenshot` + `browser_pdf` | visual + printable evidence |

## Core Workflows

### Workflow 1: Ingress Testing

1. Run `browser_test_ingress` with ingress URL.
2. If unclear result, run `browser_health_check` with expected text.
3. Capture screenshot with `browser_screenshot`.
4. If failing, extract visible error with `browser_get_text`.

Parameter example:
```yaml
url: https://payments.example.com/health
```

Assertion example:
```yaml
url: https://payments.example.com/
contains: "Payments API"
```

### Workflow 2: Grafana Dashboard Capture

1. Use `browser_screenshot_grafana` for direct dashboard capture.
2. If login redirects, switch to interactive flow with `browser_open`.
3. Use `browser_wait_for` on graph panel selector.
4. Export both screenshot and PDF.

Example:
```yaml
url: https://grafana.example.com/d/abc123/payments-overview
path: artifacts/grafana-payments.png
```

### Workflow 3: Interactive UI Investigation

1. `browser_open` target URL.
2. `browser_snapshot` to discover selectors.
3. Interact using `browser_click`, `browser_fill`, `browser_select`.
4. Validate navigation with `browser_wait_for_url`.
5. Gather text/HTML evidence.

### Workflow 4: Login and Form Flow

Recommended order:
1. `browser_open` login page.
2. `browser_fill` username and password.
3. `browser_click` submit.
4. `browser_wait_for_url` to post-login route.
5. Optional MFA with `browser_press` and additional click steps.
6. `browser_screenshot` after successful auth.

### Workflow 5: Evidence Export Pack

Use when preparing incident artifacts:
1. `browser_screenshot` key pages.
2. `browser_get_text` for user-facing errors.
3. `browser_get_html` on failing panel/container.
4. `browser_pdf` for a durable report copy.

## Kubernetes Dashboard Workflow

Use this sequence:
1. `browser_open` dashboard URL.
2. `browser_wait_for` on namespace selector/control.
3. `browser_select` target namespace.
4. `browser_click` workload detail rows.
5. `browser_get_text` status fields.
6. `browser_screenshot` of failing resource page.

Common selectors to look for:
- namespace dropdown
- pod status badge
- events tab
- logs button

## ArgoCD UI Workflow

1. `browser_open` ArgoCD URL.
2. Login using `browser_fill` + `browser_click`.
3. `browser_wait_for` app grid/list.
4. `browser_click` target app.
5. `browser_get_text` sync status and health status.
6. `browser_screenshot` app tree.
7. `browser_pdf` for change board sharing.

Useful extraction targets:
- app sync badge
- health badge
- recent operation result
- diff or history panel text

## Advanced Interaction Patterns

### Multi-tab comparative debugging

1. `browser_new_tab` for second environment URL.
2. `browser_switch_tab` between prod and staging.
3. Capture parallel screenshots.
4. `browser_close_tab` once comparison complete.

### Dynamic UI waits

Prefer explicit waits:
- `browser_wait_for` after mutation actions.
- `browser_wait_for_url` after navigation/login.

Avoid brittle fixed-delay assumptions.

### JavaScript-assisted debugging

Use `browser_evaluate` for computed state checks:
```yaml
expression: "document.title"
```

```yaml
expression: "window.location.href"
```

```yaml
expression: "Array.from(document.querySelectorAll('[role=\"alert\"]')).map(el => el.textContent)"
```

## Selector Strategy

Selector preference order:
1. stable `data-testid`
2. aria labels and role selectors
3. unique ids
4. robust class combinations

Avoid fragile nth-child chains unless unavoidable.

## Common Failures and Fixes

| Failure | Likely Cause | First Response | Follow-up |
|---|---|---|---|
| `browser_wait_for` timeout | wrong selector or delayed render | run `browser_snapshot` | refine selector strategy |
| login loops back to same page | invalid credentials or CSRF/session issue | `browser_get_text` on alerts | inspect cookies/state with `browser_evaluate` |
| blank Grafana panel | datasource or time-range mismatch | `browser_get_text` panel errors | capture full screenshot and investigate backend metrics |
| ingress health check fails | backend unavailable or auth redirect | `browser_test_ingress` then `browser_health_check` | correlate with k8s networking diagnostics |
| file upload not applied | hidden file input or validation block | `browser_upload` then `browser_get_text` | inspect form errors and retry |

## Minimal Parameter Guidance

### For URL tools

- Always pass full `https://...` URL.
- Use session reuse when performing multi-step workflows.

### For selectors

- Validate selector existence first with `browser_snapshot`.
- Prefer single, deterministic selectors.

### For artifacts

- Provide `path` when deterministic filenames are needed.
- Capture both PNG and PDF for incident records.

## Security and Safety Notes

- Never store or print raw credentials in final output.
- Avoid uploading sensitive files unless explicitly requested.
- Close browser sessions after evidence capture with `browser_close`.

## Output Contract

Always provide:
1. URL(s) tested.
2. Result status (pass/fail and evidence).
3. Captured artifact paths (screenshots/PDF).
4. Key extracted UI text tied to failure.
5. Next diagnostic step if failure persists.

Example output:
```text
URL tested: https://argocd.example.com/applications/payments
Result: Health=Degraded, Sync=OutOfSync.
Evidence: browser_get_text extracted "ComparisonError: service not found" from app details panel.
Artifacts: artifacts/argocd-payments.png, artifacts/argocd-payments.pdf
Next: run service wiring diagnostics in cluster for payments namespace.
```

## Completion Criteria

Browser investigation is complete when:
- reproduction or validation is deterministic,
- at least one visual artifact is captured,
- failure evidence includes textual extraction,
- next action is explicit and scoped.

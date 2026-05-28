# Authoring custom prompts and skills

This guide teaches you how to add team-specific investigations to RootCause
using a realistic example: **debugging an AWS PrivateLink connectivity failure
from a Kubernetes workload.**

You'll build:

- One **skill** — the always-on rules your team applies to every PrivateLink
  investigation.
- One **prompt** — the workflow your operators and AI agents run when an
  incident hits.

Together they turn an ad-hoc 30-minute troubleshooting session into a single
slash command.

---

## Mental model in 30 seconds

| Concept | What it is | When it fires | Who triggers |
|---|---|---|---|
| **Tool** | A leaf API call (`aws.vpc.list_vpc_endpoints`) | When the AI decides to call it | AI |
| **Prompt** | A workflow template the AI reads first | Once, at the start of a session | User (slash command) |
| **Skill** | Always-on rules attached to tool responses | Every matching tool call | Automatic (tag match) |

Rule of thumb:
- **Workflow steps** → prompt
- **Always-true rules** → skill
- **Cloud APIs** → tool

You write the first two as files. RootCause already provides the tools.

---

## The scenario

The payments team's `checkout-api` deployment intermittently fails to talk to
**AWS Secrets Manager**. Connectivity runs over a VPC PrivateLink endpoint.
The problem could be:

- The endpoint isn't provisioned in the workload's VPC.
- Security groups block the egress.
- Route tables don't direct traffic to the endpoint's ENI.
- Resolver rules for the private hosted zone are missing.
- The endpoint is in `pendingAcceptance` (cross-account sharing not finalised).

Each company hits this slightly differently. Your team knows the playbook;
RootCause doesn't. Let's teach it once and never re-derive again.

---

## Step 1 — Write the skill (team rules)

Skills live one-per-folder under `~/.rootcause/skills/`. Create the folder and
the file:

```bash
mkdir -p ~/.rootcause/skills/privatelink-rules
```

`~/.rootcause/skills/privatelink-rules/SKILL.md`:

```markdown
---
category: Networking
description: PrivateLink debugging rules for the payments stack
tags: [aws, privatelink, network, vpc, payments]
---

# PrivateLink Debugging Rules (Payments)

Always:
- Verify the change calendar in #network-change before suggesting any SG or
  route-table mutation. PrivateLink endpoint changes have caused two SEV1s
  in the last quarter.
- When the endpoint state is `pendingAcceptance`, this means the provider
  account has not yet accepted our VPC. Escalate to #cloud-platform — do not
  touch the endpoint.
- Cross-reference the workload pod's subnet against the route tables before
  blaming SGs. Most "DNS fails" reports turn out to be missing routes.
- For Secrets Manager specifically, check resolver rules for
  `*.secretsmanager.us-east-1.amazonaws.com`. The private hosted zone must
  resolve to the endpoint ENI's IPs.

Never:
- Suggest deleting an endpoint without explicit on-call ACK.
- Restart workload pods to "fix" connectivity. That hides the underlying
  network issue and resets diagnostic state.
- Apply IAM changes during US business hours (9am–6pm PT) — covered by
  the standing change freeze.

Output expectations:
- Always cite the endpoint ID, subnet IDs, and SG IDs you inspected.
- If a check returned no data, state "no <X> found" explicitly — don't
  silently omit it.
```

**What the tags do.** The `tags:` line decides which tool calls this skill
attaches to. Each tag matches:

- A toolset name (`aws`) — fires on every `aws.*` call.
- A tool name token (`vpc`, `network`, `privatelink`) — fires when those
  appear in the tool name or in caller-passed `skillTags`.
- A team identifier (`payments`) — fires only when the caller passes
  `skillTags: ["payments"]` (we'll wire this into the prompt below).

Verify the skill loads:

```bash
rootcause sync --list-skills | grep privatelink
```

You should see `privatelink-rules` in the output.

---

## Step 2 — Write the prompt (the workflow)

Prompts live one-per-file under `~/.rootcause/prompts/`.

```bash
mkdir -p ~/.rootcause/prompts
```

`~/.rootcause/prompts/privatelink-debug.md`:

```markdown
---
name: privatelink_debug
description: Diagnose PrivateLink connectivity failures from a k8s workload.
arguments:
  - name: namespace
    description: Kubernetes namespace of the failing workload
    required: true
  - name: workload
    description: Workload (Deployment / StatefulSet) name
    required: true
  - name: endpoint_service
    description: AWS PrivateLink service name (e.g. com.amazonaws.us-east-1.secretsmanager)
    required: true
  - name: vpc_id
    description: VPC ID (optional — auto-detected if your cluster is single-VPC)
    required: false
---

# PrivateLink Debug: {{namespace}}/{{workload}} → {{endpoint_service}}

Pass `skillTags: ["payments", "privatelink"]` on every tool call so team-specific
guidance attaches.

## Investigation Flow

1. **Workload state.** Call `k8s.list` with:
   - namespace: {{namespace}}
   - kind: Pod
   - labelSelector: app={{workload}}

   Note the pod's subnet IDs (from the node) — you'll need them later.

2. **Workload logs.** Call `k8s.logs` for the first pod with `tail: 200`.
   Look for `timeout`, `i/o timeout`, `connection refused`, `no such host`.

3. **Endpoint inventory.** Call `aws.vpc.list_vpc_endpoints` with:
   - service: {{endpoint_service}}
   - vpc_id: {{vpc_id|<auto from cluster>}}

   If `count == 0` the endpoint is not provisioned — STOP here, that's the
   root cause. Report and escalate.

4. **Endpoint detail.** Call `aws.vpc.get_vpc_endpoint` for the first match.
   Capture `state`, `subnet_ids`, `security_group_ids`, `network_interface_ids`.
   If `state` is anything other than `available`, the team-rules skill will
   tell you what to do.

5. **Routing check.** Call `aws.vpc.list_route_tables` for the VPC. Confirm
   the workload pod's subnet route table has a route to the endpoint's ENI
   (target should be the VPC endpoint, not an Internet Gateway).

6. **SG check.** Call `aws.vpc.list_security_groups` with the endpoint's
   `security_group_ids`. The endpoint SG must allow inbound on the service's
   port (443 for Secrets Manager) from the workload pod's CIDR or SG.

7. **DNS / resolver.** Call `aws.vpc.list_resolver_rules` for the VPC. For
   Secrets Manager, confirm a rule exists for
   `*.secretsmanager.<region>.amazonaws.com` resolving to the endpoint ENI.

## Output Contract

- Time-aligned summary of all checks (✓ / ✗ / not-applicable).
- The single most likely root cause, with evidence.
- Cite endpoint IDs, subnet IDs, and SG IDs you inspected (per team rules).
- One concrete remediation. If a mutation is needed, list the change calendar
  check first (per team rules).
- Whether to involve #network-oncall or #cloud-platform.
```

Two things to notice:

1. **The prompt names specific tools.** When the AI reads "call
   `aws.vpc.get_vpc_endpoint`," it does. RootCause's built-in AWS toolset
   provides each of these.
2. **The prompt instructs the AI to pass `skillTags`.** This is how the
   `privatelink-rules` skill attaches to every step — the prompt threads the
   team tag through each tool call.

---

## Step 3 — Sync to your client

The prompt is server-side via MCP, but for clean `/privatelink-debug` slash
command UX, generate the client-native file:

```bash
rootcause sync --agent claude
```

One command writes both surfaces. Default targets:

```
~/.claude/commands/privatelink-debug.md      # the slash command (your prompt)
~/.claude/skills/privatelink-rules/SKILL.md  # the skill (also attached via MCP)
```

Use `--prompts-only` or `--skills-only` if you want to update just one
surface. Existing files are NOT overwritten unless you pass `--overwrite`.

Restart Claude Code (or start a fresh chat).

---

## Step 4 — Run it

In Claude Code, type:

```
/privatelink-debug payments checkout-api com.amazonaws.us-east-1.secretsmanager
```

What happens, step-by-step:

### 1. The prompt fires

The slash command expands to the rendered prompt text, becoming the user's
first message. The AI sees the full workflow with namespace, workload, and
endpoint_service substituted in.

### 2. The AI calls `k8s.list`

It passes `skillTags: ["payments", "privatelink"]` as instructed. Before the
response returns to the AI, RootCause matches those tags against every loaded
skill. `privatelink-rules` matches both `privatelink` and `payments`. The
skill's content is attached to the tool response as `customSkillGuidance`.

### 3. The AI reads pod data + the skill rules

Now the AI knows: "if I find restarting pods, the rules say not to suggest a
restart as the fix." It proceeds to step 2 of the prompt with that constraint
in mind.

### 4. Repeats for every step

Each subsequent tool call (`aws.vpc.list_vpc_endpoints`, `aws.vpc.get_vpc_endpoint`,
etc.) gets the same skill attachment because the prompt told the AI to keep
passing `skillTags`.

### 5. The AI synthesises

The final summary cites endpoint IDs, subnet IDs, SG IDs (per team rules),
proposes one concrete fix, and flags the change-calendar check before any
mutation (per team rules) — even though none of that was in the prompt itself.
The skill drove that behaviour silently.

---

## A concrete sample run

User:

```
/privatelink-debug payments checkout-api com.amazonaws.us-east-1.secretsmanager
```

AI (after running all 7 steps):

> ## PrivateLink Debug: payments/checkout-api → com.amazonaws.us-east-1.secretsmanager
>
> **Checks**
> - ✓ Workload pods running (3/3) in subnets `subnet-0a1b…` and `subnet-0c2d…`
> - ✗ Logs show `i/o timeout` connecting to `secretsmanager.us-east-1.amazonaws.com:443` (37 occurrences in last 10m)
> - ✓ Endpoint `vpce-0f3a…` present in vpc `vpc-0b1c…`, state `available`, in subnets `subnet-0a1b…`, `subnet-0e4f…`
> - ✗ Endpoint NOT present in `subnet-0c2d…` (one of the workload subnets)
> - ✓ SG `sg-0aab…` on endpoint allows :443 from `sg-pods-checkout…`
> - ✓ Resolver rule for `*.secretsmanager.us-east-1.amazonaws.com` present
>
> **Root cause**
> The PrivateLink endpoint `vpce-0f3a…` is not provisioned in subnet
> `subnet-0c2d…`. Pods scheduled to nodes in that subnet have no path to
> Secrets Manager. The 33% failure rate matches the subnet distribution.
>
> **Remediation**
> Add `subnet-0c2d…` to the endpoint's subnet list. **Before applying:**
> verify #network-change calendar; PrivateLink endpoint changes are in the
> high-impact set. Coordinate with #cloud-platform.
>
> No restart needed — restarting would hide the routing gap without fixing it.

Notice the bolded constraints came from the **skill**, not the prompt. The
prompt didn't say "check the change calendar"; the skill did. The AI applied
the rule automatically.

---

## Iterating

You'll evolve both files over time. The workflow:

```bash
# 1. Edit the file
$EDITOR ~/.rootcause/prompts/privatelink-debug.md
# or
$EDITOR ~/.rootcause/skills/privatelink-rules/SKILL.md

# 2. Re-sync
rootcause sync --agent claude

# 3. New chat in Claude Code (or restart)
```

Two commands. Same loop for every change.

**Tip:** keep both files in a git repo (e.g. `team-runbooks/`) and symlink into
`~/.rootcause/`:

```bash
ln -s ~/work/team-runbooks/prompts ~/.rootcause/prompts
ln -s ~/work/team-runbooks/skills  ~/.rootcause/skills
```

Now your team's runbooks are versioned, code-reviewable, and shared.

---

## When to use which

| Want to encode… | File type | Example |
|---|---|---|
| The order of steps for a specific incident | Prompt | "When PrivateLink fails, check endpoints → routes → SGs → DNS" |
| A rule that applies regardless of context | Skill | "Never restart pods to fix connectivity" |
| Both | Both | This whole tutorial |
| A new API call | Go code (tool) | "Add `aws.networkmanager.describe_global_network`" |

Prompts and skills are for **logic you already know**. Tools are for **APIs that
need code**. If you're encoding company-specific judgment, you almost never
need a new tool.

---

## File layout reference

```
~/.rootcause/
├── prompts/                              # one file per prompt
│   ├── privatelink-debug.md
│   ├── payments-p1-drill.md
│   └── verify-deploy.md
├── skills/                               # one folder per skill
│   ├── privatelink-rules/
│   │   └── SKILL.md
│   ├── payments-rules/
│   │   └── SKILL.md
│   └── no-friday-deploys/
│       └── SKILL.md
└── prompts.yaml                          # (optional, legacy single-file mode)
```

### Resolution order (first existing wins for the dir; legacy file merges on top)

**Prompts directory:**
1. `ROOTCAUSE_PROMPTS_DIR` env var
2. `[prompts].dir` in `config.yaml`
3. `~/.rootcause/prompts/`
4. `~/.config/rootcause/prompts/`
5. `./rootcause-prompts.d/`

**Skills directories** (from `[skills].custom_dirs`, all scanned):
- Default: `~/.rootcause/skills`
- Add more in `config.yaml` for org-wide / project-local skills.

---

## CLI cheatsheet

```bash
# List everything available (prompts + skills, built-in + custom)
rootcause sync --list

# Sync everything to one client (project-local)
rootcause sync --agent claude --project-dir .

# Sync user-globally (writes ~/.claude/... instead of ./.claude/...)
rootcause sync --agent claude --user

# Sync to every supported client
rootcause sync --all-agents

# Subset
rootcause sync --agent claude --prompt privatelink_debug
rootcause sync --agent claude --skill  privatelink-rules

# Only one surface
rootcause sync --agent claude --prompts-only
rootcause sync --agent claude --skills-only

# Existing files are never clobbered unless you say so
rootcause sync --agent claude --overwrite

# Inspect without writing
rootcause sync --agent claude --dry-run

# Use a specific config file (loads [prompts].dir / [skills].custom_dirs)
rootcause sync --agent claude --config /etc/rootcause/config.yaml
```

---

## Front-matter reference

### Prompt (`*.md` in `~/.rootcause/prompts/`)

```yaml
---
name: <snake_case_id>            # required; underscores become dashes in slash commands
description: <one line>          # required for menus
title: <pretty name>             # optional
arguments:                       # optional; declare every {{var}} the body uses
  - name: <var_name>
    description: <one line>
    required: true|false
---

<freeform markdown body with {{var}} and {{var|default}} substitutions>
```

### Skill (`SKILL.md` inside `~/.rootcause/skills/<skill-name>/`)

```yaml
---
category: <freeform string>      # optional; groups in catalog
description: <one line>          # recommended
tags: [tag1, tag2, ...]          # required for the skill to attach
---

<freeform markdown body — the rules the AI will read>
```

### Tag-matching cheatsheet

A skill attaches to a tool call when ANY of its tags match:
- The tool's toolset name (`aws`, `gcp`, `k8s`, `rootcause`, …).
- The exact tool name (`aws.vpc.list_vpc_endpoints`).
- Any token in the tool name (`aws`, `vpc`, `list`, `vpc_endpoints`).
- Anything in the caller's `skillTags: [...]` array (passed in the tool args).

Use **broad tags** (`aws`) for guidance that should fire on every cloud call.
Use **specific tags** (`payments-privatelink`) plus prompt-passed `skillTags`
for team-scoped guidance that shouldn't bleed into other teams' investigations.

---

## Troubleshooting

| Symptom | Likely cause | Fix |
|---|---|---|
| Prompt doesn't appear in `--list-prompts` | File has a TOML/YAML syntax error or wrong directory | Check `~/.rootcause/prompts/` path; run `rootcause sync --list-prompts` |
| `/<name>` not in Claude Code's slash menu | Forgot to run `sync` or didn't restart the client | Re-sync; new chat session |
| Slash command exists but AI doesn't follow the workflow | Template body doesn't actually instruct the AI clearly | Make the steps imperative ("Call X with Y"), not narrative |
| Skill never attaches to tool calls | `tags:` line missing, or tags don't match what the AI calls | Add a broader tag (the toolset name) or instruct the prompt to pass `skillTags` |
| Skill attaches but AI ignores the rules | Rules are too long or buried | Lead with `Always:` / `Never:` bullet lists; keep the file under ~50 lines |
| Custom skill conflicts with built-in | Same `name:` as a built-in | Rename, or set `allow_custom_overrides = true` in `[skills]` if intentional |

---

## Going further

- **Compose multiple skills** by giving them overlapping tag scopes. A
  `payments-rules` skill + a `secrets-handling` skill + a `change-freeze` skill
  can all attach to a single PrivateLink debug session.
- **Project-local runbooks.** Drop a `.rootcause-prompts.d/` directory in your
  service's repo and commit team-specific prompts there. Set
  `[prompts].dir = "./.rootcause-prompts.d"` in the project's `config.yaml`.
- **Override a built-in.** Author a prompt named `troubleshoot_workload` in
  your dir and it replaces the built-in. Same for skills with
  `allow_custom_overrides = true`.

Once your team has 5-10 of these, your incident response is reproducible across
people, sessions, and assistants — not because the AI got smarter, but because
your runbook is now executable.

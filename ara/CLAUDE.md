# Agentic Runtime Authority (ARA) — Expert Reference

This file is the canonical, project-local reference for Akeyless **Agentic Runtime Authority (ARA)**. When the user asks about ARA, treat this as primary context. The "Source docs" section lists upstream files to re-read when verifying current behavior, since ARA is still early-access and changing.

## Status

**Early access.** Features, behavior, and availability can change between releases. When citing specifics (flags, tool names, supported targets), verify against the source docs below before making promises.

## One-line definition

ARA lets an AI agent run authorized **actions** (DB queries, cloud/SaaS calls) against protected resources through the Akeyless Gateway **without ever holding the credential**. The Gateway authenticates the agent, evaluates input/output rules attached to a Dynamic/Rotated/Static secret, executes the action with credentials it generates internally, and returns a (possibly redacted) response. Every call is logged with an `agent-id`.

## Mental model

```
Agent ──[payload + agent-id]──> MCP / CLI ──> Akeyless Gateway
                                                  │
                                                  ├─ AuthN: agent identity via CLI profile / auth method
                                                  ├─ AuthZ: role-rule "Agentic Runtime Authority" on secret path
                                                  ├─ Input rules: validate payload, deny disallowed ops
                                                  ├─ Generate/use credential internally (never exposed)
                                                  ├─ Execute payload against target
                                                  ├─ Output rules: filter/redact response
                                                  └─ Audit log with agent-id  ──> ara-reports-access RBAC
                                                  │
                                           [filtered result]
                                                  ↓
                                                Agent
```

Prompt injection can corrupt *what the agent asks for*, but cannot exfiltrate a credential the agent never holds, and cannot bypass rules enforced at the Gateway.

## Supported targets

| Category | Targets |
|---|---|
| **Database** | MySQL, PostgreSQL, MSSQL, Oracle, Snowflake, HanaDB, Redshift, MongoDB, Redis, Cassandra |
| **Service** | AWS, GCP, Azure, GitHub |

Operates on these secret types:
- **Dynamic secrets** — temporary, rotated credentials (primary use case)
- **Rotated secrets** — regularly rotated credentials
- **Static secrets** — typically OAuth 2.1 MCP workflows and connection-string integrations

## Two control axes (the core of ARA)

### Input rules
Constrain what the agent can **send** (queries, prompts, commands). Blocked requests are denied before reaching the target.

- Attached to a Dynamic Secret via `--input-rule` (repeatable) in `name=...,rule=...` form
- The Console can prepopulate sensible defaults per producer type (e.g., SQL producers get read-only + no-multi-statement by default)
- Default-prepopulating producers: MySQL, PostgreSQL, Redshift, MSSQL, Oracle, Snowflake, HanaDB, Cassandra, Redis, MongoDB

Example:
```
name=read-only-sql,rule=Only allow read-only SQL statements: SELECT, SHOW, DESCRIBE, DESC, EXPLAIN, WITH. Reject any DML or DDL statements such as INSERT, UPDATE, DELETE, DROP, ALTER, CREATE, TRUNCATE, GRANT, REVOKE.
```
```
name=denied-commands,rule=Deny the following Redis commands: KEYS, FLUSHALL, FLUSHDB, DEBUG, SHUTDOWN, BGSAVE, BGREWRITEAOF, SLAVEOF, REPLICAOF, CLUSTER, MIGRATE, MONITOR, SUBSCRIBE, PSUBSCRIBE, EVAL, EVALSHA, EVALRO, EVALSHA_RO, SCRIPT. Also deny CONFIG subcommands SET, REWRITE, and RESETSTAT.
```

### Output rules
Constrain what data can be **returned** to the agent. Blocked response content is filtered or redacted.

- Attached via `--output-rule` (repeatable), same `name=...,rule=...` format
- Example: `name=mask-email,rule=Mask email addresses in the returned results.`

Parser requires BOTH `name` and `rule` keys for every repeated flag.

## RBAC — two separate switches

| Concern | Mechanism | Values |
|---|---|---|
| **Dashboard / reports visibility** | Administrative role rule `--ara-reports-access` | `none` / `scoped` / `all` |
| **Runtime execution** on a path | Role-rule type **Agentic Runtime Authority** with **Allow Access** capability, scoped to secret path | per-path |

Both are independent. Execution still also requires the underlying secret permissions. A user can have reports access without execution rights and vice versa.

### Role CLI

```shell
# new role with scoped report visibility
akeyless create-role --name <role-name> --ara-reports-access scoped

# update existing role
akeyless update-role --name <role-name> --ara-reports-access <none|scoped|all>
```

Adding the execution role-rule is done in the Console (no documented CLI flag yet): role editor → add rule → type **Agentic Runtime Authority** → set path → **Allow Access** → save.

## Surfaces (where ARA is exposed today)

1. **Console**
   - "Agentic Runtime Authority" step/tab on supported Dynamic Secrets (enable + Input Rules + Output Rules tables)
   - Role editor: administrative rule `Agentic Runtime Authority` (reports scope) + role-rule type `Agentic Runtime Authority` (execution)
2. **CLI commands**
   - `akeyless runtime-authority` — direct Gateway runtime query
   - `akeyless mcp-runtime-authority` — starts MCP server for ARA tools
   - `--input-rule` / `--output-rule` flags on dynamic-secret create/update
   - `--ara-reports-access` flag on create-role / update-role
3. **MCP tools** exposed by `mcp-runtime-authority`:
   - `list-secrets` — list ARA-enabled secrets the current profile can access
   - `query-db` — run a DB query
   - `service-execute` — run a cloud/SaaS action

## CLI reference

### `akeyless runtime-authority` (direct Gateway query)

```shell
akeyless runtime-authority \
  --name /demo/apps/analytics/postgres-ro \
  --payload 'SELECT current_user, current_database();' \
  --agent-id ai-assistant-01 \
  -u https://<gateway-url>:8000 \
  --profile <profile-name>
```

Flags:
- `-n, --name` — **required**, full path of the Akeyless secret (dynamic or rotated)
- `--payload` — **required**, query/action to run (SQL, `aws s3 ls`, etc.)
- `--agent-id` — **required**, identifier for the audit trail
- `-u, --gateway-url` — **required**
- `--profile` — use an existing CLI profile (or use `--access-type` / `--access-id` / `--access-key`)

### `akeyless mcp-runtime-authority` (MCP server)

```shell
akeyless mcp-runtime-authority \
  --gateway-url https://<gateway-url>:8000 \
  --secret-name /demo/apps/analytics/postgres-ro \
  --profile <profile-name>
```

Flags:
- `--gateway-url` — **required**
- `--secret-name` — optional default secret path for `query-db` (does NOT replace RBAC scoping; use role rules and secret permissions to restrict access)
- `--profile` — existing CLI profile
- `--access-type` — `access_key | password | saml | ldap | k8s | azure_ad | oidc | aws_iam | universal_identity | jwt | gcp | cert | oci | kerberos`
- `--access-id`, `--access-key` — for non-profile auth

Note: unlike some other CLI commands, `akeyless mcp` does NOT pick up `gateway_url` from a CLI profile — `--gateway-url` must be passed explicitly. The same likely applies to `mcp-runtime-authority`.

### MCP tool semantics

| Tool | Required args | Notes |
|---|---|---|
| `list-secrets` | — | Lists ARA-supported secrets visible to the profile |
| `query-db` | `payload`, `agent-id` | `secret-name` required per-request only if no default was set at server startup |
| `service-execute` | `secret-name`, `payload`, `agent-id` | OAuth-backed service flows: on follow-up call, also pass `auth-code` and `state` after the server returns an authorization URL |

## MCP client configuration

### Claude Desktop

File: `~/Library/Application Support/Claude/claude_desktop_config.json`

### Cursor

File: `~/.cursor/mcp.json`

### Template (works for Claude and Cursor)

```json
{
  "mcpServers": {
    "akeyless-connector": {
      "command": "akeyless",
      "args": [
        "mcp-runtime-authority",
        "--gateway-url",
        "https://<Your-Akeyless-GW-URL>:8000",
        "--profile",
        "profile_name"
      ]
    }
  }
}
```

Other supported MCP clients: GitHub Copilot CLI (`~/.copilot/mcp-config.json`), JetBrains IDEs (plugin settings).

## Prerequisites

- Akeyless Gateway **≥ 4.51.0**
- **AI Insights enabled on the Gateway** — hard dependency for ARA functionality
- A Dynamic Secret configured with ARA enabled
- A role with the Agentic Runtime Authority role-rule on the relevant secret path (+ `ara-reports-access` if reports are needed)
- An auth method bound to that role
- Akeyless CLI **≥ 1.144.0** for CLI-based flows (not needed for MCP-only or direct API)
- A supported MCP client if going the MCP route

## Auditability

- Each session and query is logged with full context
- `--agent-id` is **required** specifically to give each invocation an attributable identity in the audit log
- Visibility into ARA reports is gated by the `ara-reports-access` administrative role rule (`none`/`scoped`/`all`)
- Use the dashboard for monitoring and investigation workflows

## How ARA fits into Akeyless's AI Security stack

| Offering | Role |
|---|---|
| **AI Insights** | Natural-language LLM layer over Akeyless; **gateway dependency for ARA** |
| **ARA** (this) | Runtime execution governance for agents |
| **Identity & Secrets Intelligence (ISI)** | Dashboards, inventory, scanners, policies; gated by `isi-access` |
| **Prompt-Injection Protection** | Design guidance — secretless + JIT + scoped + audit; ARA is the concrete enforcement surface |
| **MCP Server** | Transport that exposes ARA (and general Akeyless tools) to AI clients |

## Common Q&A patterns to expect

- **"How do I let an agent query Postgres safely?"** → Create a dynamic secret for Postgres, enable ARA, attach read-only input rule + mask-PII output rule, add ARA role-rule on the secret path to the role the agent's auth method uses, configure MCP client with `mcp-runtime-authority`.
- **"My agent can't see ARA reports."** → They need `--ara-reports-access scoped` (or `all`) on a role bound to their auth method. This is separate from execution rights.
- **"What's the difference between `ara-reports-access` and the ARA role-rule?"** → `ara-reports-access` is **read-only dashboard visibility**. The ARA role-rule with **Allow Access** capability is **execution authority** on a specific path. You can have either without the other.
- **"Does the agent ever see the credential?"** → No. The Gateway uses the credential internally to execute the payload and returns only the (possibly redacted) result.
- **"Can ARA stop prompt injection?"** → It reduces credential-theft and credential-misuse risk to near-zero, but does NOT prevent the agent from following bad instructions within its legitimate scope. Layered defense is still required.
- **"What targets are supported?"** → DBs: MySQL, PostgreSQL, MSSQL, Oracle, Snowflake, HanaDB, Redshift, MongoDB, Redis, Cassandra. Services: AWS, GCP, Azure, GitHub.

## Source docs (re-read to verify current behavior)

Primary upstream docs live in `~/code/akeyless/docs-akeyless/`:

- **Main ARA doc:** `docs/AI Security/agentic-runtime-authority.md`
- **AI Security overview:** `docs/AI Security/ai-security.md`
- **AI Insights (dependency):** `docs/AI Security/akeyless-ai-insight.md`
- **Prompt-injection guidance:** `docs/AI Security/prompt-injection-protection-for-ai-agents.md`
- **Identity & Secrets Intelligence:** `docs/AI Security/identity-and-secrets-intelligence.md`
- **MCP server overview:** `docs/AI Security/MCP/index.md`
- **CLI: `runtime-authority` & `mcp-runtime-authority`:** `docs/Integrations & Plugins/cli-reference/index.md` (lines ~292, ~362)
- **CLI: `--ara-reports-access` flag:** `docs/Integrations & Plugins/cli-reference/cli-reference-access-roles.md` (line ~61 for create-role, ~286 for update-role)

When the user asks anything ARA-specific that needs current accuracy (exact flag spelling, exact supported target list, exact role-rule names), grep these files before answering.

## Notable absences / things to flag if asked

- No documented CLI flag yet for adding the **execution** ARA role-rule on a path — only the Console workflow is documented. If asked for CLI parity, say so.
- No documented `--input-rule` / `--output-rule` examples beyond SQL and Redis — extrapolate carefully.
- ARA is **early access**. Any answer should be hedged accordingly when the user is making production decisions.

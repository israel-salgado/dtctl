# dtctl

[![Release](https://img.shields.io/github/v/release/dynatrace-oss/dtctl?style=flat-square)](https://github.com/dynatrace-oss/dtctl/releases/latest)
[![Build Status](https://img.shields.io/github/actions/workflow/status/dynatrace-oss/dtctl/build.yml?branch=main&style=flat-square)](https://github.com/dynatrace-oss/dtctl/actions)
[![Go Report Card](https://goreportcard.com/badge/github.com/dynatrace-oss/dtctl?style=flat-square)](https://goreportcard.com/report/github.com/dynatrace-oss/dtctl)
[![License](https://img.shields.io/github/license/dynatrace-oss/dtctl?style=flat-square)](LICENSE)
[![Go Version](https://img.shields.io/github/go-mod/go-version/dynatrace-oss/dtctl?style=flat-square)](go.mod)

**Your Dynatrace platform, one command away.**

`dtctl` is a CLI for the Dynatrace platform. Manage workflows, dashboards, queries, and more from your terminal or let AI agents do it for you. Its predictable verb-noun syntax (inspired by `kubectl`) makes it easy for both humans and AI agents to operate.

```bash
dtctl get workflows                           # List all workflows
dtctl query "fetch logs | limit 10"           # Run DQL queries
dtctl apply -f workflow.yaml --set env=prod   # Declarative configuration
dtctl get dashboards -o json                  # Structured output for automation
dtctl exec copilot nl2dql "error logs from last hour"
```

![dtctl dashboard workflow demo](docs/assets/dtctl-1.gif)

> **Early Development**: This project is in active development. If you encounter any bugs or issues, please [file a GitHub issue](https://github.com/dynatrace-oss/dtctl/issues/new). Contributions and feedback are welcome!

**[Documentation](https://dynatrace-oss.github.io/dtctl/)** · **[Installation](https://dynatrace-oss.github.io/dtctl/docs/installation/)** · **[Quick Start](https://dynatrace-oss.github.io/dtctl/docs/quick-start/)** · **[Command Reference](https://dynatrace-oss.github.io/dtctl/docs/command-reference/)**

---

## Install

```bash
# Homebrew (macOS/Linux)
brew install dynatrace-oss/tap/dtctl
```

```bash
# Shell script (macOS/Linux)
curl -fsSL https://raw.githubusercontent.com/dynatrace-oss/dtctl/main/install.sh | sh
```

```powershell
# PowerShell (Windows)
irm https://raw.githubusercontent.com/dynatrace-oss/dtctl/main/install.ps1 | iex
```

Verify the install on any platform:

```bash
dtctl version
```

Binary downloads, building from source, shell completion setup, and more in the **[Installation Guide](https://dynatrace-oss.github.io/dtctl/docs/installation/)**.

## Authenticate

You'll need your **tenant ID** — the subdomain of your Dynatrace environment URL. Open Dynatrace in your browser; the part before the first `.` in the address bar is your tenant ID.

![Dynatrace browser URL with the tenant ID portion highlighted](docs/assets/tenantid.png)

Production tenants end in `.apps.dynatrace.com`; internal Dynatrace lab tenants end in `.sprint.apps.dynatracelabs.com`.

```bash
# OAuth login (recommended, no token management needed)
dtctl auth login --context my-env --environment "https://<your-tenant-id>.apps.dynatrace.com"

# Verify everything works
dtctl doctor
```

Token-based authentication and multi-environment configuration are covered in the **[Quick Start](https://dynatrace-oss.github.io/dtctl/docs/quick-start/)**.

### Pick a safety level

Every context is created with a **safety level** that controls what `dtctl` (and any AI driving it) is allowed to do in that tenant. Pass `--safety <level>` to `dtctl auth login`, or change it later with `dtctl config set-safety <level>`.

| Level | What it allows |
|---|---|
| `readonly` | Query only — cannot create, update, or delete anything. |
| `readwrite-mine` *(recommended default)* | Read everything; create / update / delete only resources **you own** (your workflows, dashboards, notebooks). |
| `readwrite-all` | Read everything; create / update / delete **any** resource in the tenant. |
| `dangerously-unrestricted` | Everything in `readwrite-all` plus permanent deletes (e.g. emptying trash). Use only when you really mean it. |

Full token-scope mapping per safety level lives in **[docs/TOKEN_SCOPES.md](docs/TOKEN_SCOPES.md)**. You can always check the current context's level with `dtctl config describe-context $(dtctl config current-context) --plain`.

### Your first five commands

A copy-paste sanity sequence to confirm dtctl is installed, authenticated, and pointing at the right tenant:

```bash
dtctl version                       # CLI is installed and on PATH
dtctl config current-context        # which tenant am I pointed at?
dtctl auth whoami --plain           # am I authenticated?
dtctl doctor                        # full health check (CLI + auth + connectivity)
dtctl get problems --mine           # first real query against the tenant
```

If `whoami` returns `User session is no longer active` or `invalid_grant`, run `dtctl auth login` to refresh the session, then re-run the commands above. The active context is **persistent on disk** — it carries over between shells, IDEs, and reboots, so you only need to switch when you actually want a different tenant.

## Why dtctl?

- **Familiar CLI conventions**: `get`, `describe`, `edit`, `apply`, `delete`. If you (or your AI) know `kubectl`, you already know dtctl.
- **Built for AI agents**: Structured output (`--agent`), machine-readable command catalog (`dtctl commands`), and a bundled [Agent Skill](https://agentskills.io) that teaches AI assistants how to operate Dynatrace
- **Multi-environment**: Switch between dev/staging/prod with a single command; safety levels prevent accidental changes
- **Watch mode**: Real-time monitoring with `--watch` for all resources
- **DQL passthrough**: Execute queries directly, with template variables and file-based input
- **[NO_COLOR](https://no-color.org/) support**: Respects `NO_COLOR`, `FORCE_COLOR=1`, and auto-detects TTY

## How dtctl relates to the Dynatrace MCP server

Dynatrace exposes two independent ways for an AI assistant (or a human) to reach a tenant: this CLI and the Dynatrace MCP server. They are complementary, not alternatives.

| Path | How the AI invokes it | Where it's configured | Works without an AI client? |
|---|---|---|---|
| **`dtctl`** (this repo) | Through the agent's terminal/shell tool — the same way a human would. Output is plain text or, with `--agent`, a structured JSON envelope. | `~/.dtctl/config` (per machine, per user). | Yes — humans, scripts, and CI/CD all use it directly. |
| **Dynatrace MCP server** | Through the agent's MCP tool registry. Each Dynatrace operation appears as a typed tool that the AI calls directly, with no shell round-trip. | An MCP client config file (e.g. `.vscode/mcp.json`), or the hosted remote server (no local config). | No — requires an MCP-aware client. |

Both paths reach the same Dynatrace APIs, including DQL, Davis CoPilot chat, Davis Analyzers, workflows, dashboards, and notebooks. The difference is *how the AI talks to Dynatrace*, not *what it can ask Dynatrace to do*. Pick whichever fits your workflow — many users use both.

### Capability matrix: which path do I need?

If you're trying to decide whether you need MCP, dtctl, or both, this table covers the practical day-to-day differences. Most things either path can do; a handful are only on one side.

| | **`dtctl`** | **MCP** |
|---|---|---|
| **What it is** | A CLI the AI runs in your terminal | A direct API bridge the AI calls in-process |
| **Visible to you** | Yes — commands appear in the integrated terminal | No — runs silently in the background |
| **Run DQL queries** | ✅ | ✅ |
| **Read entities (services, hosts, problems, vulnerabilities)** | ✅ | ✅ |
| **Create / edit notebooks, dashboards, workflows, settings** | ✅ | ✅ |
| **Davis CoPilot chat** | ✅ (`dtctl exec copilot`) | ✅ |
| **Davis Analyzers (forecasting, anomaly detection)** | ✅ (`dtctl exec analyzer`) | ✅ |
| **Declarative `apply` / `diff` / `history` / `restore`** | ✅ | — |
| **Document sharing (`share` / `unshare`)** | ✅ | — |
| **Persistent multi-context config + safety levels** | ✅ | — (one tenant per server entry) |
| **Multiple output formats (json / yaml / csv / toon / table / wide)** | ✅ | — (always JSON) |
| **AI skills installer (`dtctl skills install`)** | ✅ | — |
| **Send ad-hoc Slack message from chat** | — | ✅ |
| **Send ad-hoc email from chat** | — | ✅ |
| **Ingest a custom event** | — | ✅ |
| **Reset Grail query budget** | — | ✅ |
| **Natural-language → DQL helpers (one-shot)** | — | ✅ |

**Bottom line:** Both are first-class. Configure whichever you'll actually use; configure both if you want every capability available at all times.

**Where to find the MCP server:**

- **[Dynatrace Hub listing](https://www.dynatrace.com/hub/detail/dynatrace-mcp-server/)** — recommended starting point. It links to the new **Remote Dynatrace MCP Server**, which requires no local install and is the path Dynatrace is actively investing in. Use this for the most current setup instructions and supported clients.
- **[dynatrace-oss/dynatrace-mcp](https://github.com/dynatrace-oss/dynatrace-mcp)** — the original open-source local MCP server (Node.js, runs as a subprocess of your AI client). It is currently in maintenance mode while the remote server takes over, but it remains the place to read the source, file issues, or run the server locally in air-gapped or self-hosted scenarios.

## Supported Resources

| Resource | Operations |
|----------|------------|
| Workflows | get, describe, create, edit, delete, apply, execute, logs, history, restore, diff, watch |
| Dashboards & Notebooks | get, describe, create, edit, delete, apply, share, history, restore, diff, watch |
| Documents & Trash | get, describe, create, edit, delete, share, history, restore |
| DQL Queries | execute, verify, template variables, live mode, filter segments, wait conditions |
| SLOs | get, describe, create, edit, delete, apply, evaluate, watch |
| Settings | get schemas, get/create/update/delete objects |
| Buckets | get, describe, create, delete, apply, watch |
| Segments | get, describe, create, edit, delete, apply, watch |
| Lookup Tables | get, describe, create, delete, apply (CSV auto-detection) |
| Anomaly Detectors | get, describe, create, edit, delete, apply |
| Extensions 2.0 | get, describe, apply monitoring configs |
| Hub Extensions | get, describe, list releases, filter by keyword |
| App Functions & Intents | get, describe, execute, find, open (deep linking) |
| Davis AI | analyzers, CoPilot chat, NL-to-DQL, document search |
| Cloud Integrations | Azure & GCP connections and monitoring (get, describe, create, delete, apply, update) |
| EdgeConnect | get, describe, create, delete |
| Notifications | get, describe, delete, watch |
| Users & Groups | get, describe |
| Live Debugger | breakpoints, workspace filters, snapshot decoding |

See the **[Command Reference](https://dynatrace-oss.github.io/dtctl/docs/command-reference/)** for the full list of verbs, flags, resource types, and aliases.

## AI Agent Skills

dtctl ships with an [Agent Skill](https://agentskills.io) that teaches AI coding assistants how to use dtctl. Agents can also bootstrap at runtime with `dtctl commands --brief -o json`.

```bash
# Install via skills.sh
npx skills add dynatrace-oss/dtctl

# Or install with dtctl itself
dtctl skills install              # Auto-detects your AI agent
dtctl skills install --for claude # Or specify explicitly
dtctl skills install --global     # User-wide installation

# Or copy manually
cp -r skills/dtctl ~/.agents/skills/   # Cross-client (any agent)
```

Compatible with GitHub Copilot, Claude Code, Cursor, Kiro, Junie, OpenCode, OpenClaw, and other [Agent Skills](https://agentskills.io)-compatible tools. See the **[AI Agent Mode docs](https://dynatrace-oss.github.io/dtctl/docs/ai-agent-mode/)** for details on the structured JSON envelope and agent auto-detection.

### Dynatrace domain skills

For deeper Dynatrace domain knowledge (DQL syntax, observability patterns, dashboards, logs, Kubernetes, and more) install the skills from **[Dynatrace/dynatrace-for-ai](https://github.com/Dynatrace/dynatrace-for-ai)**:

```bash
npx skills add dynatrace/dynatrace-for-ai
```

These skills provide the domain context (e.g., how to write DQL queries, which metrics to use for service health, how to navigate distributed traces) while dtctl provides the operational tool to act on it. Together they give AI agents everything they need to work with Dynatrace effectively.

## Observability

dtctl supports W3C Trace Context propagation and OTLP span export via the OpenTelemetry SDK. See [docs/OBSERVABILITY.md](docs/OBSERVABILITY.md) for full details on distributed tracing, environment variables, and CI/CD pipeline integration.

## Documentation

Full documentation is available at **[dynatrace-oss.github.io/dtctl](https://dynatrace-oss.github.io/dtctl/)**:

- [Installation](https://dynatrace-oss.github.io/dtctl/docs/installation/): Homebrew, shell script, binary download, build from source, shell completion
- [Quick Start](https://dynatrace-oss.github.io/dtctl/docs/quick-start/): Authentication, first commands, common patterns
- [Configuration](https://dynatrace-oss.github.io/dtctl/docs/configuration/): Contexts, credentials, safety levels, aliases
- [Command Reference](https://dynatrace-oss.github.io/dtctl/docs/command-reference/): All verbs, flags, resource types, and examples
- [Output Formats](https://dynatrace-oss.github.io/dtctl/docs/output-formats/): Table, JSON, YAML, CSV, charts
- [AI Agent Mode](https://dynatrace-oss.github.io/dtctl/docs/ai-agent-mode/): Structured envelope, auto-detection, agent skill
- [Token Scopes](https://dynatrace-oss.github.io/dtctl/docs/token-scopes/): Required API token scopes per safety level

Resource-specific guides: [DQL Queries](https://dynatrace-oss.github.io/dtctl/docs/dql-queries/) · [Workflows](https://dynatrace-oss.github.io/dtctl/docs/workflows/) · [Dashboards](https://dynatrace-oss.github.io/dtctl/docs/dashboards/) · [SLOs](https://dynatrace-oss.github.io/dtctl/docs/slos/) · [Settings](https://dynatrace-oss.github.io/dtctl/docs/settings/) · [Extensions](https://dynatrace-oss.github.io/dtctl/docs/extensions/) · [Davis AI](https://dynatrace-oss.github.io/dtctl/docs/davis-ai/) · [and more...](https://dynatrace-oss.github.io/dtctl/docs/quick-start/)

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

## License

Apache License 2.0. See [LICENSE](LICENSE).

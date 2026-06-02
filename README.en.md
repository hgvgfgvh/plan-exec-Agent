# AgentTest

> **A local multi-agent runtime (Go / Windows)** built around a `PlanAgent → BehaviorAgent` execution chain. It supports MCP (stdio/http), external Skill Packs, a Web UI (HTTP+SSE), and a sidecar RunView (turn logs → HTML).

![status](https://img.shields.io/badge/status-experimental-orange) ![language](https://img.shields.io/badge/Go-1.25-00ADD8) ![platform](https://img.shields.io/badge/platform-Windows-blue)

> ⚠️ **Experimental**: the system can run “single complex tasks” end-to-end, but the default Web UI password is a hardcoded constant and tools can operate on your machine (PowerShell / files / email / browser automation). Do not run it in an untrusted environment.

---

## What’s implemented (based on the code)

- **Main orchestration chain**: user input → `PlanAgent` generates a TodoList → `BehaviorAgent` executes steps → final delivery
- **Exec-Simple fast path (optional)**: Plan uses MemoryHook experience match + confidence to attempt a compressed “single episode” execution; failures automatically fall back to step-by-step execution
- **MCP integration**: `capabilities.mcp` supports stdio and HTTP; public tool name is ``{server}__{tool}``
- **Capability catalog (runtime injected)**: executors receive a layer-1 catalog in the system prompt; use `get_capability_details` for schemas/docs
- **External Skill Packs**: scans `WorkSpace/skill_packs/*/SKILL.md`; catalog can hot-reload (pack-level `mcp.yaml` still requires restart)
- **Web UI**: HTTP + SSE, login/chat, file upload (saved to `WorkSpace/inbox/{turn_id}/`), RunView access
- **RunView (sidecar)**: watches `WorkSpace/logs/turns/*.json` and generates `WorkSpace/run_views/*.html`

---

## Quick start

### Requirements

- Windows 10/11
- Go 1.25+
- (Optional) Python 3.x: only needed if you enable the `python-sandbox` MCP and require a local `python` executable

> Note: the example configuration uses bundled MCPs under `WorkSpace/mcp_bundled/`. Node-based MCPs typically use the bundled Node runtime in `WorkSpace/mcp_bundled/_runtime/`, so you usually don’t need to install Node separately.

### Configuration

Copy the example config and fill in your own API keys (**do not commit them to GitHub**):

```powershell
cp config/app.example.yaml config/app.yaml
```

Minimum fields to check:

```yaml
agents:
  default_model: "deepSeek-onnx"

integrations:
  dashscope:
    api_key: "sk-..."        # Qwen: web search / TTS / vision / embeddings
  deepseek_legacy:
    api_key: "sk-..."        # DeepSeek: main chat + plan orchestration

web:
  enabled: true
  listen: ":8765"
```

> See `config/app.yaml` and `config/app.example.yaml` for a fully annotated, field-by-field reference.

### Run

```powershell
go run .
# or
go build -o AgentTest.exe .
.\AgentTest.exe
```

Use another config file:

```powershell
$env:AGENTTEST_CONFIG="config/app.custom.yaml"
.\AgentTest.exe
```

### Web UI

Open `http://localhost:8765` in your browser:

- **Username**: `admin`
- **Default password**: `ZAQ!2wsx` (hardcoded constant in `webui/server.go`)

---

## System overview (short)

### Key components

- **`interaction.Router`**: unified inbound normalization, turn context injection (`turn_id`), and reply binding
- **`portal.ProcessTurn`**: sends a turn into the main chain (Plan first; fallback to Behavior if Plan is unavailable)
- **`PlanAgent`**: planning/adjustment/archiving/synthesis (does not directly call MCP tools)
- **`BehaviorAgent`**: executor (tool loop + builtin skill tree + MCP tools + external SKILL)
- **`capabilities`**: MCP connections & tool registration, capability catalogs, audit/observability knobs
- **`runview`**: sidecar watcher that generates HTML views from turn logs

### Registered agents (current)

Initialized by `agent/agentManager.go`:

- `planAgent` (main entry, orchestration)
- `behaviorAgent` (step execution)
- `execSimpleAgent` (optional; required for Exec-Simple routing)
- `interactiveAgent` (dialog/affective; currently not the default portal entry)
- `routerAgent` (router/thalamus; implemented but not used as the default portal main chain)
- `baseAgent` (generic/reference)

---

## MCP and capability catalogs

- **Public tool names**: `{server}__{tool}` (e.g. `sqlite__read_query`)
- **Progressive disclosure**: layer-1 catalog is injected into executors; use `get_capability_details` for schema/doc
- **Enable/disable**: `capabilities.mcp.enabled` in `config/app.yaml`

---

## RunView (turn view HTML)

- **Input**: `WorkSpace/logs/turns/*.json` (written by `turnjournal`)
- **Output**: `WorkSpace/run_views/*.html`
- **Switch**: `run_view.enabled` in `config/app.yaml`

See `config/run_view.example.yaml` for a standalone RunView LLM configuration (decoupled from the main chain model).

---

## Security notes (important)

- **Do not expose the Web UI to the public internet**: the password is a constant and there is no reset flow
- **Do not commit secrets**:
  - `config/app.yaml` (real keys)
  - `WorkSpace/` (logs, caches, artifacts, uploads)
  - `.env*`, IDE configs (e.g. `.idea/`)
- The default `.gitignore` ignores `WorkSpace/*` and `.env*`; for open-source, commit only `config/app.example.yaml`

---

## License

TBD (before publishing, choose a clear license such as MIT or Apache-2.0).

# AgentTest

> **A local multi-agent runtime (Go / Windows)** built around a `PlanAgent → BehaviorAgent` execution chain. It supports MCP (stdio/http), external Skill Packs, a Web UI (HTTP+SSE), and a sidecar RunView (turn logs → HTML).

![status](https://img.shields.io/badge/status-experimental-orange) ![language](https://img.shields.io/badge/Go-1.25-00ADD8) ![platform](https://img.shields.io/badge/platform-Windows-blue)

> ⚠️ **Experimental**: the system can run “single complex tasks” end-to-end, but the default Web UI password is a hardcoded constant and tools can operate on your machine (PowerShell / files / email / browser automation). Do not run it in an untrusted environment.

---

## What’s implemented (based on the code)

- **Main orchestration chain**: user input → `PlanAgent` generates a TodoList → `BehaviorAgent` executes steps → final delivery
- **Exec-Simple fast path (optional)**: Plan uses MemoryHook experience match + confidence to attempt a compressed “single episode” execution; failures automatically fall back to step-by-step execution
- **MCP integration**: `capabilities.mcp` supports stdio and HTTP; public tool name is ``{server}__{tool}``
- **Capability catalog (runtime injected)**: executors receive a layer-1 catalog in the system prompt; use `get_capability_details` for schemas/docs
- **External Skill Packs**: scans `WorkSpace/skill_packs/*/SKILL.md`; catalog can hot-reload (pack-level `mcp.yaml` still requires restart)
- **Web UI**: HTTP + SSE, login/chat, file upload (saved to `WorkSpace/inbox/{turn_id}/`), RunView access
- **RunView (sidecar)**: watches `WorkSpace/logs/turns/*.json` and generates `WorkSpace/run_views/*.html`

---

## Quick start

### Requirements

- Windows 10/11
- Go 1.25+
- (Optional) Python 3.x: only needed if you enable the `python-sandbox` MCP and require a local `python` executable

> Note: the example configuration uses bundled MCPs under `WorkSpace/mcp_bundled/`. Node-based MCPs typically use the bundled Node runtime in `WorkSpace/mcp_bundled/_runtime/`, so you usually don’t need to install Node separately.

### Configuration

Copy the example config and fill in your own API keys (**do not commit them to GitHub**):

```powershell
cp config/app.example.yaml config/app.yaml
```

Minimum fields to check:

```yaml
agents:
  default_model: "deepSeek-onnx"

integrations:
  dashscope:
    api_key: "sk-..."        # Qwen: web search / TTS / vision / embeddings
  deepseek_legacy:
    api_key: "sk-..."        # DeepSeek: main chat + plan orchestration

web:
  enabled: true
  listen: ":8765"
```

> See `config/app.yaml` and `config/app.example.yaml` for a fully annotated, field-by-field reference.

### Run

```powershell
go run .
# or
go build -o AgentTest.exe .
.\AgentTest.exe
```

Use another config file:

```powershell
$env:AGENTTEST_CONFIG="config/app.custom.yaml"
.\AgentTest.exe
```

### Web UI

Open `http://localhost:8765` in your browser:

- **Username**: `admin`
- **Default password**: `ZAQ!2wsx` (hardcoded constant in `webui/server.go`)

---

## System overview (short)

### Key components

- **`interaction.Router`**: unified inbound normalization, turn context injection (`turn_id`), and reply binding
- **`portal.ProcessTurn`**: sends a turn into the main chain (Plan first; fallback to Behavior if Plan is unavailable)
- **`PlanAgent`**: planning/adjustment/archiving/synthesis (does not directly call MCP tools)
- **`BehaviorAgent`**: executor (tool loop + builtin skill tree + MCP tools + external SKILL)
- **`capabilities`**: MCP connections & tool registration, capability catalogs, audit/observability knobs
- **`runview`**: sidecar watcher that generates HTML views from turn logs

### Registered agents (current)

Initialized by `agent/agentManager.go`:

- `planAgent` (main entry, orchestration)
- `behaviorAgent` (step execution)
- `execSimpleAgent` (optional; required for Exec-Simple routing)
- `interactiveAgent` (dialog/affective; currently not the default portal entry)
- `routerAgent` (router/thalamus; implemented but not used as the default portal main chain)
- `baseAgent` (generic/reference)

---

## MCP and capability catalogs

- **Public tool names**: `{server}__{tool}` (e.g. `sqlite__read_query`)
- **Progressive disclosure**: layer-1 catalog is injected into executors; use `get_capability_details` for schema/doc
- **Enable/disable**: `capabilities.mcp.enabled` in `config/app.yaml`

---

## RunView (turn view HTML)

- **Input**: `WorkSpace/logs/turns/*.json` (written by `turnjournal`)
- **Output**: `WorkSpace/run_views/*.html`
- **Switch**: `run_view.enabled` in `config/app.yaml`

See `config/run_view.example.yaml` for a standalone RunView LLM configuration (decoupled from the main chain model).

---

## Security notes (important)

- **Do not expose the Web UI to the public internet**: the password is a constant and there is no reset flow
- **Do not commit secrets**:
  - `config/app.yaml` (real keys)
  - `WorkSpace/` (logs, caches, artifacts, uploads)
  - `.env*`, IDE configs (e.g. `.idea/`)
- The default `.gitignore` ignores `WorkSpace/*` and `.env*`; for open-source, commit only `config/app.example.yaml`

---

## License

TBD (before publishing, choose a clear license such as MIT or Apache-2.0).


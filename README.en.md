# AgentTest

> **A local multi-agent runtime (Go / Windows)** built around a `PlanAgent → BehaviorAgent` execution chain. It supports MCP (stdio/http), external Skill Packs, a Web UI (HTTP+SSE), and a sidecar RunView (turn logs → HTML).

![status](https://img.shields.io/badge/status-experimental-orange) ![language](https://img.shields.io/badge/Go-1.25-00ADD8) ![platform](https://img.shields.io/badge/platform-Windows-blue)

> ⚠️ **Experimental**: tools can operate on your machine. Do not expose the Web UI publicly. **Never commit** `config/app.yaml` with real API keys.

---

## 🐱 Desktop Cat — one-click entry (recommended)

The **WPF desktop pet** (`desktop-cat/`) is the fastest path:

1. Wizard: **DeepSeek API key** (required), **DashScope** (optional), optional Web UI password  
2. Auto-starts `AgentTest.exe`, opens the browser (logged-in Web UI), shows the pet  

```powershell
git clone https://github.com/hgvgfgvh/plan-exec-Agent.git
cd plan-exec-Agent
copy config\app.example.yaml config\app.yaml
.\scripts\start-desktop-cat.ps1
```

Requires **.NET 8** and **Go 1.25+**. See [desktop-cat/README.md](desktop-cat/README.md).

---

## What’s implemented (based on the code)

- **Main orchestration chain**: user input → `PlanAgent` → `BehaviorAgent` → delivery
- **Exec-Simple fast path (optional)**
- **MCP integration**: public tool name ``{server}__{tool}``
- **Capability catalog** + `get_capability_details`
- **External Skill Packs** under `WorkSpace/skill_packs/`
- **Web UI** and **RunView** sidecar
- **Desktop cat**: see above

---

## Developer quick start

```powershell
copy config\app.example.yaml config\app.yaml
go build -o AgentTest.exe .
.\AgentTest.exe
```

Web UI: `http://127.0.0.1:8765` — user/password from `web.*` in `config/app.yaml` (example uses `change-me`).

---

## Security (before you push)

| OK to commit | Do not commit |
|--------------|---------------|
| `config/app.example.yaml` | `config/app.yaml` |
| source, `desktop-cat/` | `WorkSpace/*`, `.env*`, `*.exe` |

---

## License

MIT (see `LICENSE` in the repo root).

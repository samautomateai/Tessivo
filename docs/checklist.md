# Delivery checklist

Status: ☐ not started · ◐ in progress · ☑ done. Story IDs refer to the product spec.

## M0/M1 foundations

| Item | Stories | Status |
| --- | --- | --- |
| Repo, Taskfile, quality gates (lint, gosec, lengths, coverage ≥ 80%, govulncheck, CodeQL, Dependabot) | NFR Quality | ☑ |
| Layered settings with source tracking | SET-5 | ☑ |
| Secrets from environment (keychain next) | SET-1 | ☑ |
| OpenAI-compatible client: models, chat, streaming, structured output, reasoning effort | SET-2, SET-3 | ☑ |
| Test a model: `tess models test <alias|provider/model>`, `POST /v1/models/test`, desktop Test on each alias; one tiny request per model, no retries or fallback, redacted errors | SET-3 | ☑ |
| Agent-mode settings in the desktop app: max steps, tool calls (new limit, default 200), tokens, minutes, approval expiry, and the app-wide rule for each tool (Settings → Agent mode; `tess config set agent.max_tool_calls N`) | SET-7 | ☑ |
| Starter prompt library (10 prompts across the work types: change risk, triage, security review, summarize, action items, meeting notes, documentation, how-to, status and incident reports): `tess prompt library`, `GET /v1/prompt-library`, desktop prompt menu → From the library | PR-3 | ☑ |
| The CLI and the desktop app can change the same notebook at the same time: job and research notebook manifests change under a lock and are re-read before each change (a stale save gets `409`), and every file is replaced through its own temporary file, so nothing is lost or half-written | CLI-4 | ☑ |
| Aliases and fallback chains with retry/backoff | SET-4 | ☑ |
| Built-in work types with output schemas + validation and retry | WT-1, WT-2, WT-3 | ☑ |
| Extractors: text, Markdown, CSV, JSON, code base | SRC-1, SRC-3 | ☑ |
| Notebook folder store, run records, outputs | NB-1, RUN-2, REP-1 | ☑ |
| Pipeline executor | RUN-1 | ☑ |
| `tess` CLI: init, notebook, context, run, models | CLI-1, CLI-2, CLI-3 | ☑ |
| Tamper-evident audit log of runs, model attempts and outputs; `tess audit verify/show` | NFR Security | ☑ |
| Formatting gate (gofmt -s, gofumpt, goimports), `task audit`, fuzz and bench tasks | NFR Quality | ☑ |
| Portable text files: `.gitattributes` (LF everywhere), `task text` / `text:fix` gate (UTF-8, no BOM, LF, final newline) | NFR Quality | ☑ |
| `task ci`: every CI gate locally; portable fuzz runner (`tools/fuzzall`); desktop LCOV coverage gate in `covercheck`; doctor checks the race detector | NFR Quality | ☑ |
| OS-independent tests (`internal/testutil`: permissions, unwritable paths, helper process instead of `sh`); CI tests on Windows and macOS too; OS-specific work needs approval (CLAUDE.md) | NFR Quality | ☑ |
| Dev setup: `task install` (tools + hooks), `task doctor` (diagnose and repair, including symlink support for Flutter plugins — Windows Developer Mode — and desktop build caches that name a removed compiler), `task update` (latest Go toolchain and tools) | NFR Quality | ☑ |
| Desktop gates on par with Go: stricter analyzer rules (leaked streams/sinks/futures are errors), osv-scanner on `pubspec.lock`, `--enforce-lockfile`, Dart in `task check` and the pre-commit hook, Flutter CI on Windows, macOS and Linux | NFR Quality | ☑ |
| Desktop runners for Windows, macOS and Linux committed (`desktop/windows`, `macos`, `linux`; org `dev.tessivo`); `desktop:init` removed; fuller `.gitignore` (IDE, OS, logs, local secrets, Flutter ephemeral output); `task text` also checks Xcode project files, `.metadata` and `pubspec.lock` | NFR Quality | ☑ |
| Downloadable builds: `tools/release` (Go) cross-compiles `tess` for Linux, macOS and Windows (amd64, arm64), packages each desktop bundle with `tess` beside the app, writes `SHA256SUMS`; every archive carries `LICENSE` and `THIRD_PARTY.md`; `release` workflow keeps artifacts for `main` and publishes a GitHub Release for `v*` tags | NFR Quality | ☑ |
| Installers: Debian package (pure Go) and AppImage for Linux, Inno Setup `Setup.exe` (per-user, no admin) for Windows, `.dmg` for macOS (`tools/release installer`, ADR 0008) | NFR Quality | ☑ |
| Signing: Windows Authenticode for the app and installer, macOS Developer ID with hardened runtime, notarization and stapling; runs only when the secrets are set (ADR 0008) | NFR Security | ☑ |
| User guide (install, models, notebooks, sources, agent mode, schedules, chat, research, privacy, troubleshooting) and README refreshed; `tess` usage lists every source and output command | NFR Docs | ☑ |
| MIT license (`LICENSE`) and third-party notices (`THIRD_PARTY.md`: Zero for `internal/mcp`; Flutter `NOTICES` for the desktop) | NFR Legal | ☑ |
| Hardened HTTP: TLS 1.2+, no redirects, bounded timeouts, idle-stream timeout; plaintext to remote hosts refused | NFR Security | ☑ |
| Parallel code-base extraction (bounded pool); benchmarks; fuzz targets for schema, CSV/JSON, Office, audit | NFR Performance | ☑ |
| Desktop ↔ core contract: local JSON API + SSE (ADR 0004, docs/api.md) | — | ☑ |
| Flutter desktop shell: theme, sidebar, workspace, new notebook, library, settings (first version; replaced by the design system below) | — | ☑ |
| Shared service layer; `tess serve` local API (token, origin/host checks, limits, SSE runs) | — | ☑ |
| Flutter client for the local API (Unix socket/loopback, token, SSE, endpoint discovery) | — | ☑ |
| Desktop reuses a healthy core or starts `tess serve --exit-with-parent` (bundled binary or PATH), waits for health, stops it on exit; core exits if the app dies | — | ☑ |
| Desktop design system (ADR 0009): IBM Carbon tokens and components in a VS Code workbench (activity bar, side bar, panel, status bar) under a Carbon header; IBM Plex type, Carbon icons, VS Code corners; system, light, dark and high-contrast themes; zoom 80–200 % and the OS text size; every page rebuilt on it (notebook tabs and thread panel, chat with tool calls and approvals in place, settings sections, research tabs, approvals page, schedules table, template and output galleries); a rules test keeps raw colours and font sizes out | NFR Usability | ☑ |
| No demo data in the app: the fake core is test-only; connection screens (starting, can't reach the core with Retry and Copy details, reconnecting with back-off); `tess serve` starts on the default config on first run | NFR Usability | ☑ |
| Extractors: DOCX (paragraphs, tables), XLSX (all sheets, computed stats), PPTX (slides + notes), PDF (pdfium in a WebAssembly sandbox, pages marked); zip-bomb, entry, size and XML-token limits; fuzzed | SRC-1 | ☑ |
| OS keychain secrets, built in-house: `tess secret set/delete/check`; env then keychain lookup | SET-1 | ☑ |
| Rename, archive (schedules paused) and delete notebooks; delete refused while a run is active, paused or interrupted (`tess notebook`, `PATCH|DELETE /v1/notebooks/{id}`, desktop notebook menu and Archived group) | NB-1 | ☑ |
| Saved prompts per notebook with their own model settings: list, save, delete (refused while a schedule runs one), run by name (`tess prompt`, `/v1/notebooks/{id}/prompts`, desktop Prompts menu in the run bar) | PR-1, PR-2 | ☑ |
| Token limits: every model call counts toward the day (`usage.json`, 90 days kept); calls stop at `limits.max_tokens_per_day` (429) with `usage.warning` at 80% and `usage.limit` events; runs stop before their next call at `max_tokens_per_run` or the schedule's `maxTokens`; `tess usage`, `tess search`, `tess diagnostics`, `GET /v1/usage`. A single call can go past a limit; a CLI and the core counting at once can lose a few tokens | SET-6 | ☑ |
| `tess config get|set <key> [value]` (dotted keys such as `defaults.model` or `providers.0.base_url`; values decoded strictly and validated), `config export [file]` and `config import <file> [--force]` (the replaced config is kept as `.bak`) | SET-9 | ☑ |
| Desktop settings: edit run defaults and token limits, add providers, set API keys (to the OS keychain through the core; never shown), edit alias fallback chains; a setup banner until a provider, a default model and its key are set (`PUT /v1/settings`, `/v1/secrets/{ref}`) | SET-1, SET-3, SET-4 | ☑ |
| Web pages (fetched on every run) and pasted text (saved as Markdown) as notebook sources (`tess context add <nb> https://…` or `--text`, the context API, desktop Sources menu) | SRC-2 | ☑ |
| Git changes as sources (the diff of a repository against a base branch, tag or commit, read with hardened git on every run: `tess context add <nb> --git-diff <repo> --base main`, the context API, desktop Sources menu); `.diff` and `.patch` files read as text | SRC-4 | ☑ |
| Export an output as Markdown, Word (.docx) or PDF, built with the standard library and fuzzed; PDFs use the built-in fonts, so text outside Windows-1252 prints as "?" (`tess output export`, `…/outputs/{id}/export`, desktop Export menu) | REP-2 | ☑ |
| Folder format versions: `formatVersion` in `notebook.json` and `research.json`, step-by-step upgrades (`internal/format`), newer folders refused instead of overwritten | NFR Reliability | ☑ |
| Core log (rotated, redacted) and a diagnostics bundle for bug reports (`tess diagnostics`, `GET /v1/diagnostics`, desktop Settings › Save diagnostics) | NFR Operations | ☑ |
| Native Anthropic and Gemini providers (`kind` in the config; caching, thinking, tools, structured output) and provider presets in the desktop (LiteLLM, OpenAI, Anthropic, Gemini, Ollama, LM Studio, vLLM, OpenRouter, Groq, DeepSeek, Mistral, xAI, Together) | SET-1, SET-2 | ☑ |
| Rate limits and provider failures: 2 retries by default, `Retry-After` honoured, jitter, long waits fall back at once, context overflow recognised for every provider, `max_concurrent` per provider, plain hints for 401/403/404/429 | SET-4 | ☑ |
| Accessibility: every main screen passes Flutter's labeled-tap-target and text-contrast guidelines (tested); the sidebar and work-type cards announce selection and the work type; diff lines, tool approvals and the search box carry labels | NFR Look and feel | ☑ |
| Keyboard: command palette (Ctrl/Cmd+K: sections, new notebook or chat, open any notebook, search), Ctrl/Cmd+N new notebook, Ctrl/Cmd+, settings; each shortcut binds both keys, so no OS checks | NFR Look and feel | ☑ |
| Compare a version with the one it revised, line by line (`tess output diff`, `…/outputs/{id}/diff`, desktop Compare) | REP-3 | ☑ |
| Edit an output's structured result in a form (text, one-item-per-line lists, JSON for nested fields) and save it as a new version without re-running (`tess output data`/`edit`, `…/outputs/{id}/edits`, desktop Edit) | WT-4 | ☑ |
| Run an output again, optionally on another model (`tess output rerun --model`, `GET …/outputs/{id}/run` then `POST …/runs`, desktop Run again) | RUN-3 | ☑ |
| Search across notebooks, outputs, prompts, research notebooks and notes (`tess search`, `GET /v1/search`, desktop sidebar search); scans the folders with bounded reads. A SQLite index can come later for large workspaces | NB-3 | ☑ |
| Agent mode phase 1 (ADR 0005): our own loop with LLM tool calling, rooted read-only code tools, allow/ask/block rules, durable pause and resume across restarts, runs as jobs, `tess run` approve, deny, resume, cancel and pending, approval endpoints and events, desktop approvals | WT/E11 | ☑ |
| Agent mode phase 2: `run_command` runs programs without a shell, in a context folder, with secret-looking variables removed, 5 min timeout (whole process tree stopped), 256 KiB of output; calls matching the notebook allow-list run without asking, others ask (`tess notebook commands`, `PUT …/agent/commands`, desktop Agent commands card) | WT/E11 | ☑ |
| MCP client: stdio and Streamable HTTP servers in settings (`tess mcp`, `/v1/mcp/servers`, desktop Settings), test lists tools, resources and prompts | MCP-1 | ☑ |
| MCP servers attached to notebooks; their tools in agent runs as `mcp__<server>__<tool>` (ask by default) | MCP-2 | ☑ |
| MCP resources as notebook sources, read on every run | MCP-3 | ☑ |
| MCP stdio servers stop with every process they started (`npx`, `uvx`, scripts): process group on Unix, Job Object on Windows, also when Tessivo crashes on Windows (ADR 0006) | MCP-4 | ☑ |
| Research notebooks, separate from job notebooks: files, folders and MCP resources as sources (`tess research`, `/v1/research`, desktop RESEARCH group and Job/Research choice) | RES-1 | ☑ |
| Per-source summaries in the background (≤ 2 at a time, fingerprinted, events) and context levels full, summary or off, with a context bar | RES-2 | ☑ |
| Cited conversations (`[n]` / `[n:path]` resolved to sources and notes, References that open the source) and notes, human or saved from answers, in or out of the context | RES-3 | ☑ |
| Code changes via ACP delegation to bundled nca in a worktree or snapshot; change sets and apply (phase 5) | WT/E11 | ☐ |
| Notebook archives: export a notebook or research notebook as one `.tessivo.zip` (prompts, memory, copied sources, schedules, tags; outputs on request; never runs, audit log, threads, chats or secrets) and import it as a new notebook with schedules off and sources from the sender's disk to relink; hardened and fuzzed (ADR 0010; `tess notebook export|import`, `tess context relink`, `/archive` and `/import` routes, desktop Export…, Import… and the relink notice) | NB-4 | ☑ |
| Tags and pins for notebooks and research notebooks: pinned first, tag filter, search by tag (`tess notebook tag|pin|unpin|list --tag`, `PATCH` `tags`/`pinned`, desktop menu, side bar PINNED group and tag filter) | NB-5 | ☑ |
| Team packs from a folder, a .zip or a git repo: templates, library prompts and MCP servers with required and recommended marks, refreshed at start, bad items reported (ADR 0010; `tess pack add|list|refresh|remove|init|export`, `/v1/packs`, Settings → Packs) | PR-5, TPL-7 | ☑ |
| Pack MCP servers off until trusted one by one after seeing their command; trust tied to the entry's hash; a required untrusted server shows a banner (`tess mcp trust|untrust`, trust routes, Settings → MCP servers → Trust…) | MCP-5 | ☑ |

## M2: scheduling

| Item | Stories | Status |
| --- | --- | --- |
| Cron parser (names, steps, macros, CRON_TZ, DST-safe), fuzzed | SCH-1 | ☑ |
| Schedules per notebook; scheduler inside `tess serve` with no overlap and bounded concurrency | SCH-1 | ☑ |
| Missed-run policy (skip / run_once) after downtime | SCH-2 | ☑ |
| Events stream for notifications (`/v1/events`) | SCH-3 | ☑ |
| Desktop schedules screen and in-app notice banner from `/v1/events` | SCH-3, SCH-4 | ☑ |
| OS notifications for finished, failed and skipped scheduled runs (`tess serve --notify`, on from the desktop app; `tess notify test`) | SCH-3 | ☑ |
| Tray mode: with Settings → Desktop → "Keep running in the tray", closing the window hides it and runs and schedules go on; the tray menu opens or quits (`desktop.close_to_tray`, ADR 0007) | SCH-5 | ☑ |
| Schedule history (newest 50 per schedule) via API and `tess schedule list` | SCH-4 | ☑ |
| Template library: 12 built-ins + user templates; create notebook from template (context slots, disabled schedule) | TPL-1, TPL-2 | ☑ |
| Prompt library via template prompts; save notebook as template; extend with parent and version; parent-update notice with changed prompts | TPL-3–TPL-6 | ☑ |
| Pinned notebook memory included in every run (`tess memory`, API) | MEM-1 | ☑ |
| Desktop template gallery: filter by work type, use, save notebook as template, remove user templates | TPL-1, TPL-4 | ☑ |
| Desktop: live settings and models, add context by path, memory, open outputs | SET-1, NB-1, REP-1 | ☑ |
| Chat core: sessions (new, rename, pin, search, archive, delete), streaming replies with stop, model and thinking per session | CHAT-1, CHAT-2 | ☑ |
| Chat attachments: files, folders, notebooks, outputs and web pages (fetched again for every turn; carried over as web page sources when a chat becomes a notebook); kept for later turns (`tess chat --attach PATH|URL`, desktop Attach → Web page) | CHAT-3 | ☑ |
| Chat uses MCP tools the chat enables, with the agent rules (per-chat remembered answers, then app rules; ask by default): up to 8 tool turns, results ≤ 32 KiB and redacted, calls audited by digest, a note of the tools used; the desktop asks with an approval card, `tess chat --mcp` asks on the terminal (denies when piped) | CHAT-4 | ☑ |
| Global chat memory, separate from notebook memory | CHAT-5 | ☑ |
| Make a chat repeatable as a notebook (suggested work type, template, prompt, context) | CHAT-6 | ☑ |
| Save a chat reply as a validated output in the library | CHAT-7 | ☑ |
| `tess chat`: one-shot, piped and interactive; list, show, rename, pin, archive, delete, memory, repeat, save | CHAT-9 | ☑ |
| Ask about an output in its own thread (output, notebook memory and context; stop; model per thread) | OUT-1 | ☑ |
| New version of an output from its thread, revised through the pipeline and linked to its parent | OUT-2 | ☑ |
| Memory and prompt suggestions from a thread (model tools or Suggest improvements); apply with reach, staleness and variable checks; dismiss | OUT-3 | ☑ |
| Desktop three-pane notebook page: sources, run bar with the open output and its thread, outputs with lineage; `tess output` | OUT-4 | ☑ |

## CLI reference (current)

`tess new`, `tess notebook rename|archive|unarchive|delete --yes`, `tess prompt list|show|save|rm`, `tess usage`, `tess search`, `tess diagnostics`, `tess context add|list|enable|disable`, `tess run` (notebook or piped stdin, `--prompt`,
`--prompt-name`, `--var`, `--model`, `--thinking`, `--mode agent`, `--tool name=rule`, `--json`, `--out`, `--no-save`),
`tess run pending|approve|deny|resume|cancel` (agent approvals; `--notebook DIR`, `--remember run|notebook`), `tess outputs`,
`tess output ask|suggest|suggestions|apply|dismiss|refine|export|rerun|diff <notebook> <output-id>` (output threads; `refine --from ID`; `export --format md|docx|pdf --out FILE`; `rerun --model M`; `diff --against ID`),
`tess schedule add|list|remove|enable|disable|next`,
`tess research new|list|show|delete|source add|list|level|remove|refresh|ask|note add|list|rm`, `tess template`, `tess memory`, `tess serve`, `tess secret`, `tess audit`,
`tess models`, `tess worktypes`, `tess config init|show|path|get|set|export|import`, `tess version`.
Exit codes: 0 ok, 1 error, 2 usage, 3 output failed schema validation, 4 agent run waiting for approval.

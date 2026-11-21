# Delivery checklist

Status: ☐ not started · ◐ in progress · ☑ done. Story IDs refer to the product spec.

## M0/M1 foundations

| Item | Stories | Status |
| --- | --- | --- |
| Repo, Taskfile, quality gates (lint, gosec, lengths, coverage ≥ 80%, govulncheck, CodeQL, Dependabot) | NFR Quality | ☑ |
| Layered settings with source tracking | SET-5 | ☑ |
| Secrets from environment (keychain next) | SET-1 | ☑ |
| OpenAI-compatible client: models, chat, streaming, structured output, reasoning effort | SET-2, SET-3 | ☑ |
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
| Dev setup: `task install` (tools + hooks), `task doctor` (diagnose and repair), `task update` (latest Go toolchain and tools) | NFR Quality | ☑ |
| Desktop gates on par with Go: stricter analyzer rules (leaked streams/sinks/futures are errors), osv-scanner on `pubspec.lock`, `--enforce-lockfile`, Dart in `task check` and the pre-commit hook, Flutter CI on Windows, macOS and Linux | NFR Quality | ☑ |
| Desktop runners for Windows, macOS and Linux committed (`desktop/windows`, `macos`, `linux`; org `dev.tessivo`); `desktop:init` removed; fuller `.gitignore` (IDE, OS, logs, local secrets, Flutter ephemeral output); `task text` also checks Xcode project files, `.metadata` and `pubspec.lock` | NFR Quality | ☑ |
| Hardened HTTP: TLS 1.2+, no redirects, bounded timeouts, idle-stream timeout; plaintext to remote hosts refused | NFR Security | ☑ |
| Parallel code-base extraction (bounded pool); benchmarks; fuzz targets for schema, CSV/JSON, Office, audit | NFR Performance | ☑ |
| Desktop ↔ core contract: local JSON API + SSE (ADR 0004, docs/api.md) | — | ☑ |
| Flutter desktop shell: theme, sidebar, workspace, new notebook, library, settings on a demo core | — | ☑ |
| Shared service layer; `tess serve` local API (token, origin/host checks, limits, SSE runs) | — | ☑ |
| Flutter client for the local API (Unix socket/loopback, token, SSE, endpoint discovery, demo fallback) | — | ☑ |
| Desktop reuses a healthy core or starts `tess serve --exit-with-parent` (bundled binary or PATH), waits for health, stops it on exit; core exits if the app dies | — | ☑ |
| Extractors: DOCX (paragraphs, tables), XLSX (all sheets, computed stats), PPTX (slides + notes); zip-bomb, entry, size and XML-token limits; fuzzed | SRC-1 | ☑ |
| OS keychain secrets, built in-house: `tess secret set/delete/check`; env then keychain lookup | SET-1 | ☑ |
| SQLite index (FTS) alongside the folder store | NB-3 | ☐ |
| Agent executor on Eino + MCP tools | WT/E11 | ☐ |

## M2: scheduling

| Item | Stories | Status |
| --- | --- | --- |
| Cron parser (names, steps, macros, CRON_TZ, DST-safe), fuzzed | SCH-1 | ☑ |
| Schedules per notebook; scheduler inside `tess serve` with no overlap and bounded concurrency | SCH-1 | ☑ |
| Missed-run policy (skip / run_once) after downtime | SCH-2 | ☑ |
| Events stream for notifications (`/v1/events`) | SCH-3 | ☑ |
| Desktop schedules screen and in-app notice banner from `/v1/events` | SCH-3, SCH-4 | ☑ |
| OS notifications for finished, failed and skipped scheduled runs (`tess serve --notify`, on from the desktop app; `tess notify test`) | SCH-3 | ☑ |
| Schedule history (newest 50 per schedule) via API and `tess schedule list` | SCH-4 | ☑ |
| Template library: 12 built-ins + user templates; create notebook from template (context slots, disabled schedule) | TPL-1, TPL-2 | ☑ |
| Prompt library via template prompts; save notebook as template; extend with parent and version; parent-update notice with changed prompts | TPL-3–TPL-6 | ☑ |
| Pinned notebook memory included in every run (`tess memory`, API) | MEM-1 | ☑ |
| Desktop template gallery: filter by work type, use, save notebook as template, remove user templates | TPL-1, TPL-4 | ☑ |
| Desktop: live settings and models, add context by path, memory, open outputs | SET-1, NB-1, REP-1 | ☑ |
| Chat core: sessions (new, rename, pin, search, archive, delete), streaming replies with stop, model and thinking per session | CHAT-1, CHAT-2 | ☑ |
| Chat attachments: files, folders, notebooks, outputs; kept for later turns | CHAT-3 | ◐ (files, folders, notebooks ☑; URLs ☐) |
| Chat uses enabled MCP tools | CHAT-4 | ☐ (needs MCP) |
| Global chat memory, separate from notebook memory | CHAT-5 | ☑ |
| Make a chat repeatable as a notebook (suggested work type, template, prompt, context) | CHAT-6 | ☑ |
| Save a chat reply as a validated output in the library | CHAT-7 | ☑ |
| `tess chat`: one-shot, piped and interactive; list, show, rename, pin, archive, delete, memory, repeat, save | CHAT-9 | ☑ |

## CLI reference (current)

`tess new`, `tess context add|list|enable|disable`, `tess run` (notebook or piped stdin, `--prompt`,
`--prompt-name`, `--var`, `--model`, `--thinking`, `--json`, `--out`, `--no-save`), `tess outputs`,
`tess schedule add|list|remove|enable|disable|next`, `tess template`, `tess memory`, `tess serve`, `tess secret`, `tess audit`,
`tess models`, `tess worktypes`, `tess config init|show|path`, `tess version`.
Exit codes: 0 ok, 1 error, 2 usage, 3 output failed schema validation.

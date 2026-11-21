# Architecture

```
 Flutter desktop ──JSON + SSE (Unix socket / loopback, token)──┐
                                        ▼
 tess CLI ─────────────────────────► Go core ──► LiteLLM / OpenAI-compatible providers
                                        │
                     notebooks on disk ◄┴► MCP servers (agent mode)
```

## Go packages (`internal/`)

| Package | Responsibility |
| --- | --- |
| `config` | Layered settings: app → notebook → work type → prompt → run. Most specific wins; records the source of each value. `agent` block: default tool rules (allow, ask, block; exact names or `prefix*`) and limits (40 steps, 400k tokens, 30 min, approvals expire after 7 days). |
| `secrets` | Secret lookup and redaction; the default chain is environment variables, then the OS keychain. |
| `keychain` | OS credential stores: macOS keychain, Linux Secret Service, Windows Credential Manager. Values pass through stdin or memory, never argv. |
| `llm` | OpenAI-compatible client: models list, chat completions, SSE streaming, JSON-schema `response_format`, reasoning effort, tool calling (≤ 16 calls and 64 KiB of arguments per call, streamed deltas assembled). |
| `llm/fallback` logic (in `llm`) | Alias → ordered model chain; retry with backoff; falls back on timeout, rate limit, 5xx, context overflow; reports which model answered. |
| `worktype` | Built-in work types (Analyze, Summarize, Document, Report): starter prompt, suggested context, output schema. |
| `schema` | Validates model output against a JSON-schema subset. |
| `extract` | Context extractors by type: text, Markdown, code, CSV/TSV, JSON, DOCX, XLSX, PPTX, folders/code bases. Office files are read in memory with bounded decompression (per-part 32 MiB, total 128 MiB, ratio 200:1, 10k entries, 5M XML tokens). |
| `notebook` | Notebook folder layout, manifest, context items, prompts, run records (with status, tool calls and approvals), outputs; agent checkpoints (`runs/<id>.checkpoint.json`, 0600, ≤ 64 MiB) and cross-process run locks (`runs/<id>.lock`, refreshed every 30 s, stale after 2 min). |
| `executor` | `Executor` interface; `pipeline` mode now, `agent` mode (Eino) next. |
| `agenttools` | The agent's read-only tools (`read_file`, `list_directory`, `glob`, `grep`). Every path is resolved through `Roots`: cleaned, symlinks evaluated, refused outside the run's folders. Bounded: 256 KiB per read, 2,000 entries, 500 matches. |
| `audit` | Tamper-evident audit log: append-only JSONL, each entry's SHA-256 covers the previous one; secrets redacted, prompts stored as digests; `Verify` detects edits, deletions and reordering. |
| `service` | Application layer shared by CLI and server: workspace notebooks by validated ID, runs (settings, audit, records), outputs, settings, models. |
| `server` | Local API: routes, auth/host/origin checks, strict JSON, SSE runs, listeners, endpoint file. See [api.md](api.md). |
| `cron` | Five-field cron parser; DST-safe next-time computation in the schedule's zone. |
| `scheduler` | Runs notebook schedules inside `tess serve`: records each occurrence before running, no overlap, bounded concurrency, skip/run_once after downtime, history and events. |
| `template` | Built-in and user templates (0600 JSON in the config dir): validate (limits, no secrets), apply to a notebook, capture from a notebook, extend with parent lineage, parent-update detection. |
| `chat` | Chat sessions and global chat memory as 0600 JSON files (atomic, fsynced); limits on titles, messages, attachments and session size; search; one streaming reply per session; prompt assembly (system prompt, memory, history trimmed to the context budget, attached context fenced as data). |
| `notify` | OS notifications without a library: AppleScript on stdin (macOS), `notify-send` with `--` (Linux), a fixed PowerShell toast script reading environment variables (Windows); text cleaned and shortened; a queued, rate-limited dispatcher (5 per minute, extras summarized). |
| `cli` | `tess` commands (including `tess serve`); `cmd/tess` only calls it. |

## Chat store

```
~/Tessivo/chat/          (next to the notebooks workspace; Service.ChatDir overrides)
  sessions/<id>.json     one session: settings and messages (id is 16 hex characters)
  memory.json            global chat memory, separate from notebook memory
```

Chat replies are audited in the user-level audit log as `chat.message` entries (prompt digest,
attachment refs, model, outcome), never the text itself.

## Notebook folder

```
<notebook>/
  notebook.json      manifest: id, name, work type, settings overrides, context items
  prompts/           one JSON file per prompt
  context/           copied files (code bases and folders are referenced in place)
  runs/<run-id>.json run record: inputs, effective settings, model used, tokens, timing, result
  outputs/<id>.json  validated structured output (+ rendered .md)
  schedules.json     schedule definitions (user-edited)
  schedule-state.json  last handled occurrence and run history (scheduler-only)
  audit.jsonl        tamper-evident log of runs, model attempts, tool calls, saved outputs
```

SQLite (FTS, retrieval) arrives with the desktop MVP; the folder stays the source of truth so
notebooks remain portable and diff-able.

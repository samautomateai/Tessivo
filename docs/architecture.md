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
| `config` | Layered settings: app → notebook → work type → prompt → run. Most specific wins; records the source of each value. `agent` block: default tool rules (allow, ask, block; exact names or `prefix*`) and limits (40 steps, 400k tokens, 30 min, approvals expire after 7 days). `mcp` block: up to 50 servers (`name`, `command`+`args`+`env` or `url`+`headers`); env and header values hold `${secret:NAME}` placeholders resolved at connect time, never credentials. |
| `secrets` | Secret lookup and redaction; the default chain is environment variables, then the OS keychain. |
| `keychain` | OS credential stores: macOS keychain, Linux Secret Service, Windows Credential Manager. Values pass through stdin or memory, never argv. |
| `llm` | Provider clients behind one `Chatter` interface (`NewProviderClient` picks by `kind`): OpenAI-compatible (default), and native Anthropic Messages (prompt caching on the system prompt and earlier turns, thinking budgets, structured output via a forced tool) and Gemini generateContent (thinking budgets, JSON-schema output; key in a header) clients. OpenAI-compatible client: models list, chat completions, SSE streaming, JSON-schema `response_format`, reasoning effort, tool calling (≤ 16 calls and 64 KiB of arguments per call, streamed deltas assembled). |
| `llm/fallback` logic (in `llm`) | Alias → ordered model chain. Rate limits, 5xx and timeouts are retried on the same model (`max_retries`, default 2; 0 turns it off) with exponential backoff plus up to 25% jitter, or after the wait the provider asks for (`Retry-After`, `retry-after-ms`, Gemini `retryDelay`); a wait over 30 s falls back to the next model at once, or fails saying when to retry. Falls back on timeout, rate limit, 5xx and context overflow (OpenAI, Anthropic and Gemini wording); never mid-stream. `max_concurrent` per provider queues requests beyond a cap (cancellable). Errors say what to do for 401, 403, 404 and 429. Reports which model answered and every attempt. |
| `worktype` | Built-in work types (Analyze, Summarize, Document, Report): starter prompt, suggested context, output schema. |
| `schema` | Validates model output against a JSON-schema subset. |
| `extract` | Context extractors by type: text, Markdown, code, CSV/TSV, JSON, DOCX, XLSX, PPTX, PDF (pdfium in WebAssembly via wazero: sandboxed, ≤ 64 MiB, ≤ 2,000 pages, 2 MiB of text, 60 s, a stuck parse is killed), folders/code bases. Office files are read in memory with bounded decompression (per-part 32 MiB, total 128 MiB, ratio 200:1, 10k entries, 5M XML tokens). |
| `notebook` | Notebook folder layout, manifest, context items, prompts, run records (with status, tool calls and approvals), outputs; agent checkpoints (`runs/<id>.checkpoint.json`, 0600, ≤ 64 MiB) and cross-process run locks (`runs/<id>.lock`, refreshed every 30 s, stale after 2 min). |
| `executor` | `Executor` interface; `pipeline` mode (extract → map-reduce → `Generate`: schema call, validate, retry). A `Revision` in the spec turns a run into a revision of an earlier output (the parent and the requests take their share of the context budget). |
| `executor/agent` | Agent mode: tool-using loop over the router with allow/ask/block rules (`Policy`), up to 4 tools in parallel, results ≤ 32 KiB, old results elided past 60% of the context budget, limits (steps, tokens, time). Saves a checkpoint after every step; an unanswered "ask" pauses the run (`PausedError`); `Continue` resumes, reporting calls in flight at a crash as "outcome unknown". Ends with `executor.Generate` over the findings. |
| `agenttools` | The agent's read-only tools (`read_file`, `list_directory`, `glob`, `grep`). Every path is resolved through `Roots`: cleaned, symlinks evaluated, refused outside the run's folders. Bounded: 256 KiB per read, 2,000 entries, 500 matches. |
| `export` | Outputs as files: Markdown, and Word (.docx: zip + WordprocessingML from the standard library; headings, lists, bold, inline code; tables as text lines; ≤ 4 MiB in; `FuzzDOCX`). |
| `logfile` | The core log: size-capped, rotated into numbered copies, every write redacted; `Tail` for diagnostics. |
| `format` | Folder format versions: step-by-step upgrades of `notebook.json` and `research.json`; newer folders refused. |
| `webfetch` | Reads web pages added as sources: http(s) only, no credentials, ≤ 3 redirects, 20 s, ≤ 2 MiB, text-like types; HTML reduced to text (`HTMLText`, fuzzed). |
| `mcp` | MCP client, ported from Zero (MIT): JSON-RPC framing (line-delimited or Content-Length, ≤ 4 MiB per message), stdio servers (program + args, no shell, allow-listed environment, stopped by closing stdin then killing the direct process after 2 s) and Streamable HTTP (the llm package's hardened client, session header). `tools/list`, `tools/call`, `resources/list`, `resources/read` (text ≤ 2 MiB), `prompts/list`, paginated up to 500/500/200; 30 s per request, 120 s per tool call. MCP types stay in this package and the service. |
| `research` | Research notebooks, separate from job notebooks: folder store (`research.json` manifest, ≤ 200 sources, ≤ 500 notes of ≤ 64 KiB), sources (files copied, folders in place, MCP resources) with a level each (`full`, `summary`, `off`), summaries (≤ 8 KiB, keyed by a SHA-256 fingerprint of what was read), the prompt builder (sources as numbered `<source>` blocks in 3/5 of the budget, largest cut first; included notes; history trimmed) and citations (`[n]`, `[n:path]` resolved against the numbered items, relabelled to names before an answer is sent again). |
| `audit` | Tamper-evident audit log: append-only JSONL, each entry's SHA-256 covers the previous one; secrets redacted, prompts stored as digests; `Verify` detects edits, deletions and reordering. |
| `service` | Application layer shared by CLI and server: workspace notebooks by validated ID, runs (settings, audit, records), outputs, settings, models. MCP: servers added, removed (refused while a notebook attaches them) and tested; config writes go through one lock to the config file; attached servers connect when an agent run starts or resumes and close when it ends or pauses; MCP resource sources are read per run, one connection per server. Output threads: one conversation per output (notebook memory and context, `suggest_memory`/`suggest_prompt` tools or one structured call for suggestions; apply checks staleness and new variables; audited as `thread.message` and `suggestion.apply`) and new versions (the parent revised through the pipeline, linked by `parentId`). Agent runs are jobs: the run holds its lock while executing; `Answer`, `Resume`, `Cancel`, `Pending`, `Recover` (startup: executing runs become interrupted, expired approvals fail); `RunEvent`s for approvals and outcomes; tool calls audited as digests (`tool.call`, `tool.decision`, `run.resume`). |
| `server` | Local API: routes, auth/host/origin checks, strict JSON, SSE runs, listeners, endpoint file. See [api.md](api.md). |
| `cron` | Five-field cron parser; DST-safe next-time computation in the schedule's zone. |
| `scheduler` | Runs notebook schedules inside `tess serve`: records each occurrence before running, no overlap, bounded concurrency, skip/run_once after downtime, history and events. A paused agent run keeps its schedule from running again (later occurrences are skipped) until its approval is answered. |
| `template` | Built-in and user templates (0600 JSON in the config dir): validate (limits, no secrets), apply to a notebook, capture from a notebook, extend with parent lineage, parent-update detection. |
| `chat` | Chat sessions (and keyed stores for output threads; `note` messages are never sent to the model) and global chat memory as 0600 JSON files (atomic, fsynced); limits on titles, messages, attachments and session size; search; one streaming reply per session; prompt assembly (system prompt, memory, history trimmed to the context budget, attached context fenced as data). |
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
  notebook.json      manifest: formatVersion, id, name, work type, settings overrides, context items
  prompts/           one JSON file per prompt
  context/           copied files (code bases and folders are referenced in place)
  runs/<run-id>.json run record: inputs, effective settings, model used, tokens, timing, result
  outputs/<id>.json  validated structured output (+ rendered .md); parentId links a revised version
  threads/<output-id>.json      the output's conversation (a chat session; created on first message)
  suggestions/<output-id>.json  memory and prompt suggestions from that thread (open, applied or dismissed)
  schedules.json     schedule definitions (user-edited)
  schedule-state.json  last handled occurrence and run history (scheduler-only)
  audit.jsonl        tamper-evident log of runs, model attempts, tool calls, saved outputs
```

## Research notebook folder

Research notebooks live beside the workspace (`<workspace parent>/research/<id>/`), so job notebooks, their
listing and their runs never see them.

```
<research>/
  research.json        manifest: formatVersion, id, name, description, settings, sources (kind, path or server+URI, level)
  sources/             copied files (folders are referenced in place)
  summaries/<sid>.json summary text, fingerprint of what was read, model, status (ready or failed) and error
  notes/<nid>.json     note: title, text, origin (human or ai), include in context
  chats/<cid>.json     conversations (chat sessions); <cid>.citations.json maps answers to their citations
```

Summaries are made in the core's background by a runner started with `tess serve`: at most 2 at a time, a queue of
up to 1024, a source whose fingerprint matches its stored summary is not sent again. Each job emits
`research.summary.started|ready|failed` on `/v1/events`. A ready summary switches a new source to the `summary`
level unless the user already picked one. The CLI summarizes in-process and waits.

Opening a notebook and sending a question check summarized sources against their content: a source is read for its
fingerprint at most every 30 s, a changed one is summarized again at most every 5 minutes (so a source that changes on
every read cannot loop), and a source that cannot be read shows as `unreadable` and is skipped with a note. Context
totals use each source's cached size and never re-read it. A source too large for 16 condensing calls is summarized
from evenly spaced samples, and its summary says so. A file source whose manifest path is not its own copy under
`sources/` is treated as unreadable and is never read or deleted.

Both manifests record `formatVersion` (1 today; `internal/format`). A folder without one is from before versions were recorded and opens as 1; an older one is upgraded step by step when opened and saved with the current version; one from a newer Tessivo is refused ("update Tessivo") instead of being read and overwritten. A refused notebook is left out of lists.

SQLite (FTS, retrieval) arrives with the desktop MVP; the folder stays the source of truth so
notebooks remain portable and diff-able.

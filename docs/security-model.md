# Security and performance model

Tessivo reads untrusted material (documents, archives, code bases, model output, MCP results) and
holds credentials for model providers. Every control below exists in code and is tested.

## Threats and controls

| Threat | Control | Where |
| --- | --- | --- |
| Credential leak in config or argv | Config stores only `api_key_ref`; keys come from env or the OS keychain; keychain writes pass values via stdin (hex on macOS), never argv; terminal input has echo off; `config show` and `secret check` never print values | `config`, `secrets`, `keychain` |
| Credential leak in logs, errors, audit | Value redaction plus pattern redaction (Bearer, `sk-…`, `api_key=`/`token=`/`password=`); provider error bodies redacted | `secrets`, `llm`, `audit` |
| Key sent in clear text | Plain `http://` refused for non-loopback hosts unless `allow_insecure_http`; URLs with embedded credentials refused; TLS 1.2+ | `config`, `llm/transport.go` |
| Headers or keys forwarded by redirects | All HTTP redirects refused | `llm/transport.go` |
| Hung or slow provider | Bounded dial, TLS handshake and time-to-first-byte; whole-call timeout for non-streaming; idle timeout between stream chunks; response bodies capped (32 MiB, SSE line 4 MiB) | `llm` |
| Hostile PDFs | Parsed by pdfium compiled to WebAssembly and run in a wazero sandbox inside the core: no access to the process memory or files; ≤ 64 MiB, ≤ 2,000 pages, 2 MiB of text, 60 s per document, after which the instance is killed; at most 2 at a time; fuzzed (`FuzzPDF`) | `extract/pdf.go` |
| Zip bombs, huge Office files | Compressed size ≤ 64 MiB; ≤ 10k entries; per-part 32 MiB, total 128 MiB, ratio ≤ 200:1, enforced while reading, never from declared sizes | `extract/archive.go` |
| XML attacks (XXE, billion laughs) | Go's decoder never expands external or custom entities; ≤ 5M tokens per part | `extract/office.go` |
| Oversized inputs | Text files ≤ 2 MiB; code-base files ≤ 256 KiB, ≤ 2000 files; sheets ≤ 100k rows × 1000 cols; JSON ≤ 200k chars | `extract` |
| Path tricks in archives | Zip entries are only looked up by cleaned name; nothing is written to disk | `extract/archive.go` |
| Path tricks in notebook names | Prompt and file names restricted to `[A-Za-z0-9._-]`; copied context files renamed | `notebook` |
| Untrusted model output | Validated against the work type's schema; invalid output is never saved as an output | `schema`, `executor` |
| Repudiation / silent edits of history | Hash-chained, fsynced, 0600 audit log; prompts recorded as digests; `tess audit verify` | `audit` |
| Local file exposure | Files written 0600, directories 0700/0750, atomic replace | `config`, `notebook`, `audit` |
| Other local users or processes calling the core | 0600 socket in a 0700 dir (loopback-only TCP on Windows); 256-bit per-session token, constant-time compare; token only in a 0600 endpoint file. The token is equivalent to running programs as the user: it can add an MCP server (a program) and start it with `test` | `server` |
| Websites attacking the loopback API (CSRF, DNS rebinding) | Requests with `Origin`/`Sec-Fetch-*` refused; only loopback `Host` values accepted | `server/middleware.go` |
| API abuse | 1 MiB bodies, strict JSON, header/read timeouts, ≤ 4 concurrent runs, panic recovery, errors redacted | `server` |
| Path traversal via API | Notebooks addressed by `[a-z0-9-]` IDs inside the workspace; context paths must be absolute files the user picked | `service` |
| Secrets or data leaking via shared templates | Templates hold prompts, slots, settings and memory only; context data is never captured; credential-looking text is rejected; files ≤ 256 KiB, strict JSON, 0600 | `template` |
| Prompt injection through attached chat context | Attached text sits in `<source>` tags the system prompt marks as data; closing tags inside it are escaped; context capped at 3/5 of the budget | `chat/prompt.go` |
| Chat store abuse | Sessions addressed by 16-hex IDs only; messages ≤ 64 KiB, replies ≤ 512 KiB, ≤ 20 attachments, ≤ 2000 messages, session files read ≤ 32 MiB; one reply per session at a time | `chat`, `service` |
| Injection through notification text | Text never becomes code or a command line: AppleScript literals are escaped and sent on stdin, PowerShell reads environment variables in a fixed script, `notify-send` gets argv after `--`; control characters removed; errors redacted | `notify`, `cli/cmd_notify.go` |
| Agent tools reaching outside the run | Tool paths are cleaned, symlinks evaluated and refused outside the notebook's context folders; reads ≤ 256 KiB, listings ≤ 2,000, grep ≤ 500 matches (RE2, linear time) | `agenttools` |
| Prompt injection through tool results | The system prompt marks tool output as data; tools that are not read-only default to ask; unknown tools and non-JSON arguments are answered without running | `executor/agent` |
| Runaway or abusive agents | ≤ 40 steps, 400k tokens, 30 min per run; ≤ 16 tool calls and 64 KiB of arguments per reply; results ≤ 32 KiB; checkpoints ≤ 64 MiB | `llm`, `executor/agent`, `notebook` |
| Unsupervised side effects | allow/ask/block per tool (run → notebook → app → built-in); "ask" pauses the run until answered (expires after 7 days); answers and tool calls are audited as digests; a call in flight at a crash is never replayed | `executor/agent`, `service` |
| Model-made changes to a notebook (output threads) | Suggestions are only proposals: tool arguments are parsed as hostile (known tools only, JSON, size caps, ≤ 3 per reply, ≤ 20 open per output), redacted before storing, shown verbatim and applied only by the user; a prompt change is refused when the prompt changed since (digest) or it adds an unset variable; applies are audited as digests | `service`, `notebook` |
| Thread files and folders outside the workspace | Thread and suggestion files are named by validated output IDs; the API addresses notebooks by ID only; a notebook folder is accepted only when the CLI lists it in `NotebookDirs` | `notebook`, `service` |
| MCP server processes | Started from the configured program and arguments with `os/exec` (no shell); environment limited to an allow-list (PATH, home, temp, locale and the like) plus the server's own values; stopped by closing stdin, then killing the process after 2 s; stderr kept ≤ 64 KiB and shown only, redacted, when the handshake fails | `mcp` |
| Credentials for MCP servers | Config holds `${secret:NAME}` placeholders only: credential-shaped text, any literal under a sensitive env or header name, a value given to a sensitive flag (`--token x`) and credentials in the URL are refused; values resolve from the environment or keychain at connect time; everything the server sends back (errors, tool results, resource text) is redacted of those values, the provider keys and credential-shaped text | `config`, `service` |
| Hostile MCP servers and results | Messages ≤ 4 MiB, lists capped (500 tools, 500 resources, 200 prompts), timeouts per request and call; plain http only to loopback unless allowed; redirects refused; tool names sanitized; schemas ≤ 16 KiB; results capped and redacted like other tool output; MCP tools ask before running by default; resources are text only, ≤ 2 MiB | `mcp`, `agenttools`, `service` |
| Prompt injection through research sources and notes | Source text, summaries and notes sit in numbered `<source>`/`<note>` blocks the system prompt marks as data; closing tags inside them are escaped; sources capped at 3/5 of the budget; MCP resource text is redacted like other MCP output; a source that cannot be read is skipped and named, never guessed | `research/prompt.go`, `service` |
| Citations as untrusted model output | `[n]` and `[n:path]` are parsed from the answer with an RE2 pattern (linear time, paths ≤ 512 characters) and kept only when `n` names an item that was sent; markers inside code blocks and inline code are not citations; a path is kept only as text, never opened from the citation; earlier answers are relabelled before being sent again so old numbers cannot point at new items | `research/cite.go` |
| Summaries as stored model output | Summaries are capped at 8 KiB, made from at most 16 condensing calls (larger sources are sampled), re-made at most every 5 minutes per source (cut at a rune boundary), stored 0600 with the fingerprint and model that made them, shown to the user, and re-made only when the source's content changes; a failed summary keeps its redacted error and leaves the source at `full` | `research`, `service` |
| Research files outside their folder | Research notebooks, sources, notes and conversations are addressed by validated IDs; a copied file source is read or deleted only when its manifest path is its own copy under `sources/`; copied files ≤ 64 MiB; ≤ 200 sources and ≤ 500 notes; MCP URIs ≤ 2,048 characters | `research`, `service` |
| API keys set from the desktop | `PUT /v1/secrets/{ref}` hands the key to the OS keychain inside the core (same code as `tess secret set`); the value is never returned (`GET` says only whether it is set), never logged, and errors are redacted; the config keeps only the reference | `service/settings.go`, `keychain` |
| Settings changed while work runs | Every read of the config goes through one lock and gets a copy; writers replace whole blocks, so a run keeps the settings it started with | `service` |
| Web pages as sources | Fetched only from http(s) URLs without credentials, never with cookies or auth headers; ≤ 3 redirects, each checked the same way (no `file:` or other schemes); 20 s, ≤ 2 MiB, text-like content types only; HTML reduced to text in one pass without a parser tree (fuzzed); the text is sent as data like other sources. Pages are chosen by the user, never by model output | `webfetch`, `notebook` |
| Logs and diagnostics leaking data | The core log (`<config dir>/logs/core.log`, 0600, 5 MiB × 4 files) records start, failures and event kinds with run IDs, never prompts, content, names or tokens; every line is redacted as it is written. Diagnostics bundles hold versions, the config (references only), yes/no for each key, counts and the log tail, redacted again | `logfile`, `service/diagnostics.go` |
| Data sent to cloud providers | Self-hosted by default: model calls go only to providers you configure. Native Anthropic and Gemini providers send prompts and context straight to those services; they exist only when you add them, and the Add provider dialog says so. Keys travel in headers (`x-api-key`, `x-goog-api-key`), never in URLs, and are redacted from provider errors | `llm`, `config` |
| MCP tools in chat | Only servers the chat enables; the same allow/ask/block layers as agent runs (remembered per chat, then app rules, ask by default); an unanswered ask is denied after 5 minutes; at most 8 tool turns, then an answer without tools; results ≤ 32 KiB and redacted like other MCP output; calls audited by digest | `service/chat_tools.go`, `server/chat_approvals.go` |
| Vulnerable dependencies | Standard library first; `go mod verify`, govulncheck, osv-scanner (desktop `pubspec.lock`), gosec, CodeQL, Dependabot | CI, `task audit`, `task vuln` |

## Performance budgets

| Path | Approach | Benchmark |
| --- | --- | --- |
| Code-base extraction | Walk once, then a bounded worker pool (≤ 16, GOMAXPROCS-based) keeping order | `BenchmarkTree500Files` (~9 ms for 500 files on 2 vCPU) |
| Tabular data | Single pass for statistics; small tables in full, large ones sampled | `BenchmarkCSV10kRows` (~8 ms) |
| Office parsing | In-memory, streaming XML tokens; only the needed parts are decompressed | fuzz + unit tests |
| Output validation | Single decode and walk of the schema | `BenchmarkValidateJSON` |
| Audit log | Opening reads only the last line (O(1)); one fsync per entry | `BenchmarkAppend` |
| Model calls | Connection reuse (keep-alive, HTTP/2), streaming deltas | — |

Run `task bench` to reproduce and `task fuzz` to exercise every parser.

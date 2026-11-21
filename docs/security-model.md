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
| Zip bombs, huge Office files | Compressed size ≤ 64 MiB; ≤ 10k entries; per-part 32 MiB, total 128 MiB, ratio ≤ 200:1, enforced while reading, never from declared sizes | `extract/archive.go` |
| XML attacks (XXE, billion laughs) | Go's decoder never expands external or custom entities; ≤ 5M tokens per part | `extract/office.go` |
| Oversized inputs | Text files ≤ 2 MiB; code-base files ≤ 256 KiB, ≤ 2000 files; sheets ≤ 100k rows × 1000 cols; JSON ≤ 200k chars | `extract` |
| Path tricks in archives | Zip entries are only looked up by cleaned name; nothing is written to disk | `extract/archive.go` |
| Path tricks in notebook names | Prompt and file names restricted to `[A-Za-z0-9._-]`; copied context files renamed | `notebook` |
| Untrusted model output | Validated against the work type's schema; invalid output is never saved as an output | `schema`, `executor` |
| Repudiation / silent edits of history | Hash-chained, fsynced, 0600 audit log; prompts recorded as digests; `tess audit verify` | `audit` |
| Local file exposure | Files written 0600, directories 0700/0750, atomic replace | `config`, `notebook`, `audit` |
| Other local users or processes calling the core | 0600 socket in a 0700 dir (loopback-only TCP on Windows); 256-bit per-session token, constant-time compare; token only in a 0600 endpoint file | `server` |
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

# Security and performance model

Tessivo reads untrusted material (documents, archives, code bases, model output, MCP results) and
holds credentials for model providers. Every control below exists in code and is tested.

## Threats and controls

| Threat | Control | Where |
| --- | --- | --- |
| Credential leak in config or argv | Config stores only `api_key_ref`; keys come from env/keychain; `config show` never prints values | `config`, `secrets` |
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
| Vulnerable dependencies | Standard library first; `go mod verify`, govulncheck, gosec, CodeQL, Dependabot | CI, `task audit` |

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

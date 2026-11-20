# ADR 0003: Standard library first; every dependency justified

- Status: accepted (2026-11-20)

## Rule

Use the Go standard library and our own small packages unless building it ourselves makes no
sense: a protocol we must interoperate with, OS integration, a database engine, or a large,
well-maintained framework we deliberately chose (ADR 0002). Each direct dependency is listed
below with its reason in the same change that adds it.

## Built ourselves (no dependency)

| Need | Why we build it |
| --- | --- |
| OpenAI-compatible client (chat, streaming SSE, models, fallback, retry) | Small HTTP + JSON surface; full control of fallback and logging |
| JSON-schema subset validator for outputs | We only need object/array/string/number/enum/required |
| Layered settings and config (JSON) | Simple merge rules; no YAML/TOML dependency yet |
| Text, Markdown, CSV, JSON and code-base extractors | Stdlib `encoding/*`, `io/fs` cover them |
| DOCX, XLSX, PPTX extractors | They are zip + XML; `archive/zip` and `encoding/xml` suffice |
| CLI flags | `flag` package is enough |
| Length and coverage gates | Tiny tools in `tools/` |

## Planned dependencies (added when their story starts)

| Dependency | Reason |
| --- | --- |
| `github.com/cloudwego/eino` (+ eino-ext openai, mcp) | Agent mode (ADR 0002) |
| `github.com/modelcontextprotocol/go-sdk` | Official MCP client/server; protocol interop |
| `modernc.org/sqlite` | Embedded SQLite without cgo; writing a database makes no sense |
| `github.com/zalando/go-keyring` | OS keychain access on macOS, Windows, Linux |
| `google.golang.org/grpc`, `google.golang.org/protobuf` | Desktop ↔ core contract |
| PDF text extraction library (TBD) | PDF parsing is large and error-prone; pick a maintained pure-Go one |

## Current direct dependencies

Go: none — standard library only.

Flutter (`desktop/pubspec.yaml`):

| Dependency | Reason |
| --- | --- |
| `material_ui` | Flutter's official Material library, moved out of the framework in Flutter 3.47 |
| `flutter_lints` (dev) | Official lint set for `flutter analyze` |

State management uses the SDK's `ChangeNotifier`; no third-party state package.

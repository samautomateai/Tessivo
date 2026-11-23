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
| OS keychain (macOS `security -i` with hex secrets on stdin, Linux `secret-tool` via stdin, Windows `CredReadW`/`CredWriteW` via `syscall`) | Each is a small wrapper over an OS-provided tool or API; no third-party code needed |
| Desktop ↔ core API (JSON + SSE over Unix socket/loopback) | `net/http` and `dart:io` cover it; see ADR 0004 |
| Length, text-file (encoding, line endings), coverage (Go and LCOV) gates; fuzz runner | Tiny tools in `tools/` |
| Debian package writer, installer scripts (`tools/release`, ADR 0008) | A `.deb` is `ar` + two tar.gz files; Inno Setup and hdiutil are driven with generated scripts and fixed arguments, no packaging library |
| Agent loop, tool calling, code tools (ADR 0005) | Small and specific to our rules, checkpoints and validation; Eino/Fantasy would bypass our router |
| Anthropic and Gemini provider clients (`internal/llm/anthropic*.go`, `gemini.go`) | Written for Tessivo with the standard library, following the provider split in Zero (MIT): one native client per API family, everything else on the OpenAI-compatible path. A few hundred lines each; no vendor SDKs |
| MCP client: JSON-RPC framing, stdio and Streamable HTTP, tools, resources, prompts (`internal/mcp`) | Ported from Zero (MIT, stdlib-only, commit `8288ae81bd6a`, notice kept in `internal/mcp/doc.go` and `THIRD_PARTY.md`); a few hundred lines, no MCP SDK dependency |
| Dev tool setup (`task install/doctor/update`) | `tools/devtools` runs `go install …@latest`, `git` and `flutter`; tools are installed binaries, never module dependencies. It does not install Flutter: PATH and the native desktop toolchain (Visual Studio C++, Xcode + CocoaPods, clang/CMake/Ninja/GTK) differ per OS and need admin rights, so `doctor` prints per-OS install steps instead (approved OS-specific text, `tools/devtools/flutter.go`) |

## Planned dependencies (added when their story starts)

| Dependency | Reason |
| --- | --- |
| nca (bundled binary, not a Go module; MIT) | Coding agent the agent delegates code changes to over ACP (ADR 0005, phase 5); pinned release, checksums verified at build time |
| `modernc.org/sqlite` | Embedded SQLite without cgo; writing a database makes no sense |

## Current direct dependencies

Go:

| Dependency | Reason |
| --- | --- |
| `github.com/klippa-app/go-pdfium` (MIT) with pdfium (BSD-3/Apache-2.0) compiled to WebAssembly, run by `github.com/tetratelabs/wazero` (Apache-2.0); pulls `jolestar/go-commons-pool` (Apache-2.0), `google/uuid` (BSD-3), `golang.org/x/sys` and `x/text` (BSD-3) | PDF text extraction. pdfium is the PDF engine browsers use; in WebAssembly it runs sandboxed in the core process (no cgo, no external program), so a hostile PDF cannot reach memory or files outside the sandbox. Writing a PDF parser makes no sense. Costs about 5 MB of binary |
| `golang.org/x/sys` (BSD-3, the Go team) | Windows Job Objects for stopping an MCP server with every process it started (ADR 0006, `internal/proctree`); `syscall` lacks them. Already in the tree through wazero |

Flutter (`desktop/pubspec.yaml`):

| Dependency | Reason |
| --- | --- |
| `material_ui` | Flutter's official Material library, moved out of the framework in Flutter 3.47 |
| `tray_manager` 0.5.3, `window_manager` 0.5.2 (MIT, leanflutter; pull `menu_base`, `screen_retriever` (MIT) and `shortid` (BSD-3)) | System tray icon and menu, and keeping the window open on close, on Windows, macOS and Linux (ADR 0007). Both need native code per OS; the mature 0.5 lines are used, not the November 2026 rewrites on `nativeapi` |
| `flutter_lints` (dev) | Official lint set for `flutter analyze` |

State management uses the SDK's `ChangeNotifier`; no third-party state package.

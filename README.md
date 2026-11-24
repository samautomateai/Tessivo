# Tessivo

Analyze, summarize, document and report with the models you choose. A native desktop app and a CLI (`tess`) built
on one Go core. It works with any OpenAI-compatible endpoint, self-hosted (LiteLLM, vLLM, Ollama) or hosted, and
with Anthropic and Gemini through their own APIs.

> Status: feature-complete for the first release; see the [user guide](docs/user-guide.md) and the
> [delivery checklist](docs/checklist.md).

## Concepts

- **Notebook**: one project for one recurring job: a work type, sources, prompts, memory, runs, outputs and
  schedules in one portable folder.
- **Work type**: Analyze, Summarize, Document or Report. Sets the starter prompt, expected context and the
  structure of the output.
- **Sources**: any mix of documents (PDF, Word, Excel, PowerPoint), code bases, git changes, web pages, pasted
  text and MCP resources.
- **Agent mode, chat and research**: tools under allow/ask/block rules, chats that can become notebooks, and
  research notebooks with cited answers.

## Download

Each release on the GitHub Releases page has:

- Installers:
  - Windows: `…_windows_amd64_setup.exe`, a per-user install with no admin rights needed.
  - macOS: `…_macos_arm64.dmg`.
  - Linux: `…_linux_amd64.deb` and `.AppImage`.
- `tessivo-desktop_<version>_<platform>_<arch>` archives: the desktop app with `tess` bundled beside it.
- `tess_<version>_<os>_<arch>`: the CLI for Linux, macOS and Windows on amd64 and arm64.
- `SHA256SUMS`: check a download with `sha256sum -c SHA256SUMS --ignore-missing`.

Releases are signed and notarized once the signing secrets are configured ([ADR 0008](docs/adr/0008-installers-signing.md)).
Until then, Windows SmartScreen and macOS Gatekeeper warn on the first launch; the [user guide](docs/user-guide.md)
says how to proceed. Builds from `main` are kept for 30 days as artifacts of the `release` workflow run.

To build locally:
- `task release:cli` builds every CLI target.
- `task release:desktop PLATFORM=linux|windows|macos` builds the desktop app for the OS you are on.
- `go run ./tools/release installer` builds the installers.

## Quick start

```sh
tess config init                       # writes the config with a LiteLLM provider
tess secret set TESSIVO_LITELLM_KEY    # stores the key in your OS keychain
tess new risk --type analyze
tess context add risk ./my-repo
tess run risk --model litellm/<model>
tess schedule add risk --cron "0 8 * * MON" --missed run_once   # runs while tess serve / the app is open
tess chat --attach ./runbook.md "How often do we rotate keys?"   # prints the session id; --session continues it
tess chat repeat <session> --name "Key rotation"                 # turn the chat into a notebook
```

## Develop

Requires Go (any recent release; it fetches the toolchain pinned in `go.mod`) and, for the desktop
app, Flutter stable (on Windows also Developer Mode, which Flutter plugins need for symlinks; `task doctor`
checks it). Everything else is installed for you:

```sh
go run ./tools/devtools install   # first time, before Task exists: Task, golangci-lint v2, govulncheck, gosec, osv-scanner, git hooks
task doctor         # diagnose the setup and repair what it can (task doctor -- -check only reports)
task update         # move go.mod to the latest Go toolchain, upgrade every tool and Flutter
task install        # reinstall every tool at its latest version
task check          # every quality gate, Go and desktop: lengths, lint, coverage ≥ 80%, vulnerabilities
task ci             # everything CI runs, locally: + text, tidy, build, race tests, fuzz smoke, desktop
task build          # bin/tess
task desktop:run    # run the desktop app on this OS
task desktop:check  # format, analyze, test (coverage ≥ 80%) the desktop app
```

Docs: [user guide](docs/user-guide.md) · [architecture](docs/architecture.md) · [decisions](docs/adr) ·
[security](SECURITY.md) ·
[rules for contributors](CLAUDE.md)

## License

[MIT](LICENSE). Third-party notices, including the MIT-licensed code ported from Zero, are in
[THIRD_PARTY.md](THIRD_PARTY.md); every release archive carries both files.

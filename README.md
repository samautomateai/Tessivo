# Tessivo

Analyze, summarize, document and report with self-hosted models. A native desktop app and a CLI
(`tess`) built on one Go core, using any OpenAI-compatible endpoint such as a LiteLLM proxy.

> Status: M1 foundations in progress. See [docs/checklist.md](docs/checklist.md).

## Concepts

- **Notebook**: one project for one recurring job: a work type, context, prompts, memory, runs,
  outputs and schedules in one portable folder.
- **Work type**: Analyze, Summarize, Document or Report. Sets the starter prompt, expected context
  and the structure of the output.
- **Context**: any mix of documents, spreadsheets, code bases, diffs, URLs, text and MCP resources.

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

Requires the latest stable Go (see `go.mod`), [Task](https://taskfile.dev), golangci-lint v2 and,
for the desktop app, Flutter stable.

```sh
task check          # every quality gate: lengths, lint, coverage ≥ 80%, govulncheck
task build          # bin/tess
task desktop:init   # once: generate Flutter platform runners
task desktop:check  # analyze, format, test the desktop app
```

Docs: [architecture](docs/architecture.md) · [decisions](docs/adr) · [security](SECURITY.md) ·
[rules for contributors](CLAUDE.md)

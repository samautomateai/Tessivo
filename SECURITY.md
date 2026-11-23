# Security policy

## Reporting a vulnerability

Please report vulnerabilities privately through GitHub's
[security advisories](https://github.com/samautomateai/tessivo/security/advisories/new).
Do not open a public issue. We aim to acknowledge reports within 3 working days.

## How Tessivo protects you

- **Secrets** (API keys, MCP tokens) live in the OS keychain or environment variables, never in
  config files, notebooks, templates or logs.
- **Self-hosted by default**: model traffic goes only to the providers you configure: an OpenAI-compatible endpoint such as LiteLLM, or Anthropic and Gemini directly if you add them. No telemetry.
- **Local-only core**: the desktop UI talks to the Go core over a 0600 Unix socket (loopback on Windows)
  with a per-session token; browser-origin and non-loopback Host requests are refused.
- **Audit trail**: every run, model attempt, tool call and saved output is appended to a hash-chained
  `audit.jsonl` (0600). Prompts are stored as SHA-256 digests, secrets are redacted, and
  `tess audit verify` detects any edit, deletion or reordering.
- **Tool calls** from agent mode and MCP servers follow allow / ask / block rules and are audited.
- **Supply chain**: standard library first; every dependency is justified in
  [docs/adr/0003-dependency-policy.md](docs/adr/0003-dependency-policy.md), scanned with `govulncheck`,
  and kept current by Dependabot.

Full threat model and controls: [docs/security-model.md](docs/security-model.md).

## Automated checks on every push

| Check | Tool |
| --- | --- |
| Static security analysis | gosec (in golangci-lint and standalone SARIF), CodeQL |
| Known vulnerabilities | govulncheck, `go mod verify`; osv-scanner on `desktop/pubspec.lock` |
| Parser robustness | Go fuzz targets (CI smoke run per target) |
| Formatting | gofmt -s, gofumpt, goimports |
| Code quality | golangci-lint, `flutter analyze --fatal-infos` |
| Coverage | 80% minimum for Go and Flutter |
| Size limits | 120-character lines, 500-line files (800 for tests) |

# ADR 0005: our own agent loop; MCP and coding agents built in-house or delegated

- Status: accepted (2026-11-21); supersedes [ADR 0002](0002-agent-engine-eino.md)
- Design: [docs/superpowers/specs/2026-11-21-agent-mode-design.md](../superpowers/specs/2026-11-21-agent-mode-design.md)

## Context

Agent mode is a coding-style agent for Tessivo's own purpose. It explores code and documents, runs checks and
calls MCP tools, and ends with a validated Summarize, Analyze, Document or Report output. Every tool has an allow,
ask or block rule, and an unanswered "ask" must pause the run durably, scheduled runs included. The core is
standard library only (ADR 0003). Model calls must go through our router, for its fallback chains, hardened HTTP
and audit of every attempt.

## Decision

- **Our own loop** (`internal/executor/agent`) over `llm.Router`, which gained OpenAI tool calling. The loop calls
  tools (up to 4 in parallel), applies the rules, saves a checkpoint after every step, and ends with
  `executor.Generate`, the same schema call, validation and retry as the pipeline.
- **Durable approvals.** An unanswered "ask" saves the checkpoint and pauses the run. The user answers in the CLI,
  the API or the desktop app. Resume works across core restarts, and nothing resumes on its own: a run that was
  executing when the core died becomes `interrupted`.
- **Code tools** (`internal/agenttools`): read-only, rooted in the run's folders, bounded, plus
  `run_command` (phase 2, done): programs without a shell; calls on the notebook allow-list run without asking.
- **MCP** (phases 3–4, done): our own client in `internal/mcp`, ported from Zero's MIT-licensed, stdlib-only client
  (commit `8288ae81bd6a`; framing, stdio and Streamable HTTP; resources and prompts written here).
  No MCP SDK dependency.
- **Code changes** (phase 5): delegated over the Agent Client Protocol to a coding agent, with nca (MIT, Rust)
  bundled as the default. The delegate works only in a separate copy (git worktree, or a snapshot when there is
  no git). Tessivo computes the diff, and the user applies it, with a hash check against conflicting edits.

## Alternatives considered

| Option | Why not |
| --- | --- |
| Eino (ADR 0002) + official MCP SDK | Two pre-1.0 dependency trees, an adapter so it uses our router, and still no coding tools |
| Fantasy (Charm) + MCP SDK | Preview API, brings its own provider clients (bypasses our router) |
| Crush's code | FSL-1.1-MIT forbids competing use for two years per release |
| Zero or nca as the engine | Neither is a library: separate processes, own model clients, no work-type validation; nca is kept as the delegate |

## Consequences

- The Go core still has no third-party modules. We own the loop, and its limits are configurable: 40 steps,
  400k tokens and 30 minutes by default, with approvals expiring after 7 days.
- Agent runs are jobs the core owns. They outlive the API connection, can be cancelled, and hold a run lock while
  executing.
- Delegation means bundling and pinning nca releases, with checksums, per platform (phase 5).
- Stopping a server or command with its child processes is OS-specific; approved and built in ADR 0006.

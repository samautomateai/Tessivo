# ADR 0002: Eino for agent mode, behind our own Executor interface

- Status: superseded by [ADR 0005](0005-agent-loop-mcp-acp.md) (2026-11-21); kept for the record

## Decision

- Two execution modes behind one `executor.Executor` interface:
  - **Pipeline** (default): our own Go code. Extract → chunk → map-reduce or retrieve → one
    structured-output call → validate. Deterministic and schedulable.
  - **Agent**: tool-using loop on [Eino](https://github.com/cloudwego/eino) (OpenAI-compatible chat
    model pointed at LiteLLM, eino-ext MCP tools, interrupt/resume for tool approvals).
- Eino types stay inside `internal/executor/agent`; the version is pinned and upgrades get their own
  tested change (Eino is pre-1.0).

## Alternatives considered

Koine (our own Go agent; early, coding-focused), goose (Rust, architecture in flux), ADK Go
(Gemini-leaning), a hand-rolled loop (possible, but Eino gives MCP, streaming and checkpoints).

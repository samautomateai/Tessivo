# ADR 0006: stop a child process together with everything it started

- Status: accepted (2026-11-23); approved OS-specific code (CLAUDE.md engineering rules)

## Context

MCP stdio servers are often a tree, not one process: `npx`, `uvx`, `docker run` or a shell script start the real
server as a child. Tessivo used to kill only the direct process when it closed a server, when a server flooded
it or stopped reading its input. The rest of the tree kept running, held the pipes open and leaked until the
user found it. There is no portable way to name "this process and its descendants": the standard library's
`os/exec` kills one process.

## Decision

`internal/proctree` gives one API, `Prepare(cmd)` before start, `Attach(cmd)` after, `Tree.Kill` and
`Tree.Release`, with one file per OS family behind an internal interface:

| OS | How | File |
| --- | --- | --- |
| Linux, macOS, BSD (`unix` build tag) | The child gets its own process group (`Setpgid`); Kill sends `SIGKILL` to the group | `proctree_unix.go` |
| Windows | The child is put in a Job Object with kill-on-close; Kill terminates the job. If Tessivo itself dies, Windows closes the handle and the tree goes with it | `proctree_windows.go` (`golang.org/x/sys/windows`) |
| Anything else | Only the direct process is killed, as before | `proctree_other.go` |

`internal/mcp` uses it for every stdio server: on close (after the grace period), on a flooding or stalled server,
and once the server has exited (descendants it left behind). If `Attach` fails, the connection still works and
falls back to killing the direct process.

## Alternatives considered

| Option | Why not |
| --- | --- |
| Kill only the direct process (before) | Leaks the real server behind `npx`/`uvx`/scripts |
| Walk the process table and kill descendants by parent id | Needs `/proc` or OS tools per OS, races with new children, and parent ids are reused |
| `taskkill /T` / `pkill -g` | OS tools in argv, a process per kill, and no cleanup when Tessivo crashes |
| `syscall` only on Windows | It has no Job Object calls; `x/sys/windows` is the Go team's package and already in the tree |

## Consequences

- A tree is named per OS; the test (`internal/proctree`) starts a child that starts a grandchild sharing stdout and
  checks the pipe reaches EOF after Kill. It runs on every CI OS and fails when the tree kill is turned off.
- Windows: the child runs for a moment before it joins the job; a descendant it starts in that moment is outside
  it. Servers start their work after the first message, which is sent after `Attach`.
- Unix: a descendant that leaves the group (`setsid`, daemonizing) is outside it by its own choice.
- Agent commands (when they come) and other long-lived children should use the same package.

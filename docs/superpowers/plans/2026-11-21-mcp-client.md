# MCP Client Implementation Plan (agent mode phases 3 and 4)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or
> superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Tessivo can connect to MCP servers (stdio and Streamable HTTP) configured in settings. Notebooks can attach
them: agent runs call their tools as `mcp__<server>__<tool>` (default rule: ask), and pipeline and agent runs read
their resources as context.

**Architecture:**
- New package `internal/mcp`. It holds JSON-RPC framing and two transports, stdio and Streamable HTTP, ported from
  Zero (`github.com/Twigpine/zero`, commit `8288ae81bd6a`, MIT; the notice is kept in each ported file's header). It
  also holds a `Client` with `ListTools`, `CallTool`, `ListResources`, `ReadResource` and `ListPrompts`. MCP types
  never leave `internal/mcp` and `internal/agenttools/mcp.go` (CLAUDE.md: engine-agnostic core).
- Server definitions live in the app config (`mcp.servers`). The service changes them through one locked path that
  rewrites the config file.
- Notebooks attach servers by name (`agent.mcpServers`), and resources as context items of kind `mcp-resource`.

**Tech Stack:** Go standard library only (no new dependency), Flutter for the desktop screens.

**Spec:** `docs/superpowers/specs/2026-11-21-agent-mode-design.md`, sections 5.3, 5.6, 5.11, 6 and 7 (phases 3–4).
Stories MCP-1, MCP-2, MCP-3.

## Global Constraints

- OS independent. A stdio server is started with `os/exec` (program + args, no shell). It is stopped by closing its
  stdin, waiting 2 s, then `Process.Kill()` on the direct process only. Killing child processes needs approval
  (spec §9) and is out of scope.
- No new dependencies. Port from Zero only these: framing (`protocol.go`), the stdio client's reader/writer/dispatch
  core (`client.go`) and the Streamable HTTP client (`network_client.go`, `networkClient` only). Drop the legacy SSE
  transport, OAuth, permissions, images and `aimlapi`.
- Bounds:
  - message ≤ 4 MiB (Zero's 64 MiB is lowered)
  - ≤ 500 tools, ≤ 500 resources and ≤ 200 prompts listed per server
  - `tools/call` 120 s, other requests 30 s, handshake 30 s
  - tool results ≤ 32 KiB, as the agent already caps
  - stderr kept ≤ 64 KiB and only quoted, redacted, when the handshake fails
  - a resource read ≤ 2 MiB text
- Secrets:
  - `env` and `headers` values in config are literal text that may contain `${secret:NAME}` placeholders, resolved
    through `Service.Secrets` (environment, then keychain) at connect time and never written back.
  - A literal value that `secrets.RedactPatterns` would change is rejected on add (400): no credentials in config.
  - Child environments get only `PATH`, `HOME`/`USERPROFILE`, `SYSTEMROOT`, `TEMP`/`TMP`, `LANG` plus the server's
    own `env`.
- HTTP servers use the llm package's hardened client (exported as `llm.NewHTTPClient`): TLS 1.2+, no redirects,
  timeouts. Plain `http://` is allowed only to loopback, unless the server sets `allow_insecure_http`.
- Names: server names match `^[a-z0-9][a-z0-9_-]{0,31}$`. Tool names exposed to models are
  `mcp__<server>__<tool>`, with every character outside `[a-z0-9_]` mapped to `_`, cut to 64 characters, and a clash
  gets a `_2` suffix.
- Rules: `mcp__*` tools default to `ask` (already the built-in rule for anything not read-only). Runs record the
  server and tool in the audit log as digests, as `tool.call` already does.
- Gates as in CLAUDE.md: lines ≤ 120, files ≤ 500 (tests ≤ 800), funlen 80, gocyclo 15, coverage ≥ 80%.
  - Fuzz the framing (`FuzzReadMessage`) and the tool-name sanitizer.
  - Commits: `git -c user.name=Claude -c user.email=noreply@anthropic.com commit`, each message ending with
    `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`.
- Docs in the same task as the code: `docs/api.md`, `docs/architecture.md`, `docs/security-model.md`,
  `docs/checklist.md` (MCP-1, MCP-2, MCP-3), and `docs/adr/0003-dependency-policy.md` (it notes that nothing was
  added).

## Review Focus

1. **A server that hangs or floods:** a stdio server that never answers `initialize`, or writes 100 MiB without a
   newline. Expected: a clear error within the timeout, bounded memory, and the process stopped. Covered in Task 2.
2. **A server that exits mid-run:** the next tool call errors with "server exited" and the run continues, with no
   hang and no panic. Covered in Task 2 and Task 6.
3. **Credentials:** a header such as `Authorization: Bearer ${secret:GH_TOKEN}` never appears in config, logs,
   errors, audit entries or API responses, and an `add` with a literal token is refused. Covered in Task 4.
4. **A paused agent run:** stdio servers are closed when the run pauses and reconnected on resume, and the tool
   names stay the same across the pause. Covered in Task 6.
5. **A resource that fails:** an unreachable server or an over-size resource makes a pipeline run fail with a
   message naming the server and URI. It never makes the run silently drop the source. Covered in Task 7.

---

### Task 1: JSON-RPC framing (ported)

**Files:**
- Create: `internal/mcp/protocol.go`, `internal/mcp/doc.go` (package doc and the Zero MIT notice)
- Test: `internal/mcp/protocol_test.go`

**Interfaces:**
- Produces:
  ```go
  type message struct { // JSON-RPC 2.0
      JSONRPC string          `json:"jsonrpc"`
      ID      *int64          `json:"id,omitempty"`
      Method  string          `json:"method,omitempty"`
      Params  json.RawMessage `json:"params,omitempty"`
      Result  json.RawMessage `json:"result,omitempty"`
      Error   *rpcError       `json:"error,omitempty"`
  }
  type rpcError struct{ Code int `json:"code"`; Message string `json:"message"` }
  func newReader(r io.Reader) *reader      // line-delimited JSON; also accepts Content-Length framing
  func (r *reader) read() (message, error) // ≤ MaxMessage bytes, else ErrTooLarge
  func writeMessage(w io.Writer, m message) error // one line + '\n'
  const MaxMessage = 4 << 20
  var ErrTooLarge = errors.New("mcp: message exceeds 4 MiB")
  ```

- [ ] **Step 1: Write the failing tests**
  - `TestReadLineDelimited`: two messages, in order.
  - `TestReadHeaderFramed`: `Content-Length: N\r\n\r\n{…}`.
  - `TestReadTooLarge`: 5 MiB without a newline returns `ErrTooLarge` without allocating more than about 4 MiB.
  - `TestReadBadJSON`: an error naming the problem.
  - `TestWriteRoundTrip`.
  - `FuzzReadMessage`: never panics, and every accepted message re-encodes.
- [ ] **Step 2: Run** `go test ./internal/mcp`. Expected: FAIL (undefined).
- [ ] **Step 3: Port** Zero `internal/mcp/protocol.go`. Rename the types, lower the limit, and drop the
  Zero-specific helpers.
- [ ] **Step 4: Run** `go test -race ./internal/mcp` and
  `go test -run=^$ -fuzz=FuzzReadMessage -fuzztime=20s ./internal/mcp`. Expected: PASS.
- [ ] **Step 5: Commit** `feat(mcp): JSON-RPC framing ported from Zero`

### Task 2: Client over stdio and Streamable HTTP

**Files:**
- Create: `internal/mcp/client.go` (requests, dispatch, the methods below), `internal/mcp/stdio.go`,
  `internal/mcp/http.go`
- Modify: `internal/llm/transport.go` (export `NewHTTPClient`)
- Test: `internal/mcp/client_test.go`, `internal/mcp/http_test.go`. The fake stdio server is the test binary itself,
  through `testutil.HelperCommand`.

**Interfaces:**
- Produces:
  ```go
  type Server struct { // resolved: no placeholders left
      Name, Command string; Args []string; Env map[string]string
      URL string; Headers map[string]string; AllowInsecureHTTP bool
  }
  type Tool struct{ Name, Description string; InputSchema json.RawMessage }
  type Resource struct{ URI, Name, Description, MimeType string }
  type Prompt struct{ Name, Description string; Arguments []PromptArgument }
  type PromptArgument struct{ Name, Description string; Required bool }
  type CallResult struct{ Text string; IsError bool } // text blocks joined; other block types described in one line
  type Client struct{ /* unexported */ }
  func Connect(ctx context.Context, s Server) (*Client, error) // starts or dials, then initialize + initialized
  func (c *Client) ListTools(ctx context.Context) ([]Tool, error)
  func (c *Client) CallTool(ctx context.Context, name string, args json.RawMessage) (CallResult, error)
  func (c *Client) ListResources(ctx context.Context) ([]Resource, error)
  func (c *Client) ReadResource(ctx context.Context, uri string) (text, mimeType string, err error)
  func (c *Client) ListPrompts(ctx context.Context) ([]Prompt, error)
  func (c *Client) Close() error // stdio: close stdin, wait 2 s, kill
  var ErrServerExited = errors.New("mcp: server exited")
  ```
  - Pagination: follow `nextCursor` up to the list caps (500/500/200), then stop with a truncated flag dropped
    silently, since the caps are documented.
  - `ReadResource` accepts `text` contents. A `blob` content is decoded only when its MIME type is `text/*` or
    `application/json`; otherwise the error is "binary resource not supported".

- [ ] **Step 1: Write the failing tests** (the fake server mode is chosen by an env var in the helper process)
  - `TestStdioHandshakeAndTools`: initialize, `tools/list` returning 2 tools, and `tools/call` returning text.
  - `TestStdioPagination`: three pages of resources are joined.
  - `TestStdioInitializeTimeout`: a server that never answers gives an error in at most about 1 s (the timeout is
    shortened by the test), and the process is gone after `Close`.
  - `TestStdioServerExits`: a call after the server exits returns `ErrServerExited`.
  - `TestStdioFloodBounded`: 6 MiB without a newline gives `ErrTooLarge`, and the client closes.
  - `TestHTTPJSONAndSSEReplies`: an `httptest` server answers one request with JSON and one with
    `text/event-stream`, and both work. `Mcp-Session-Id` is echoed.
  - `TestHTTPRefusesRedirectAndPlainRemote`: a 302 response gives an error; `http://example.com` without
    `AllowInsecureHTTP` is refused before dialing.
  - `TestCallResultJoinsText`: image and resource blocks become `[image: image/png]` lines.
- [ ] **Step 2: Run** `go test ./internal/mcp`. Expected: FAIL.
- [ ] **Step 3: Implement.**
  - Port Zero's stdio `Client` (one reader goroutine, a pending map keyed by ID, a writer queue) and its
    `networkClient` (POST; a JSON or SSE reply; the session ID header).
  - Add `resources/list`, `resources/read` and `prompts/list`, which Zero's client does not have.
- [ ] **Step 4: Run** `go test -race ./internal/mcp ./internal/llm`. Expected: PASS.
- [ ] **Step 5: Commit** `feat(mcp): stdio and Streamable HTTP client with tools, resources and prompts`

### Task 3: Server settings in the config

**Files:**
- Create: `internal/config/mcp.go`
- Modify: `internal/config/file.go` (`File.MCP MCPConfig \`json:"mcp,omitzero"\``; `Validate` calls `MCP.Check`)
- Test: `internal/config/mcp_test.go`

**Interfaces:**
- Produces:
  ```go
  type MCPConfig struct{ Servers []MCPServer `json:"servers,omitempty"` }
  type MCPServer struct {
      Name              string            `json:"name"`
      Command           string            `json:"command,omitempty"`
      Args              []string          `json:"args,omitempty"`
      Env               map[string]string `json:"env,omitempty"`
      URL               string            `json:"url,omitempty"`
      Headers           map[string]string `json:"headers,omitempty"`
      AllowInsecureHTTP bool              `json:"allow_insecure_http,omitempty"`
  }
  func (c MCPConfig) Check() error         // names unique and valid; exactly one of command or url; ≤ 50 servers
  func (s MCPServer) Check() error         // url https or loopback http unless AllowInsecureHTTP; no literal secrets
  func (c MCPConfig) Find(name string) (MCPServer, bool)
  func ExpandSecrets(v string, get func(ref string) (string, error)) (string, error) // "${secret:NAME}" only
  ```

- [ ] **Step 1: Write the failing tests**
  - `TestMCPCheck`: a table of bad names, both or neither of command and url, `http://remote` without the flag,
    duplicate names, 51 servers.
  - `TestMCPRejectsLiteralSecret`: a header `Bearer sk-abc123…` is an error, while `Bearer ${secret:GH}` is accepted.
  - `TestExpandSecrets`: one or two placeholders; a missing secret is an error naming the reference but never a
    value; text without placeholders is unchanged.
  - `TestConfigRoundTripsMCP`: `Save` and `Load` keep the servers.
- [ ] **Step 2: Run** `go test ./internal/config`. Expected: FAIL.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run** `go test -race ./internal/config`. Expected: PASS.
- [ ] **Step 5: Commit** `feat(config): MCP server settings with secret placeholders`

### Task 4: Service: manage, connect and test servers

**Files:**
- Create: `internal/service/mcp.go`
- Modify: `internal/service/service.go` (`ConfigPath string` field; a `cfgMu sync.RWMutex` guarding `Config.MCP`)
- Test: `internal/service/mcp_test.go`

**Interfaces:**
- Consumes: Tasks 2 and 3.
- Produces:
  ```go
  type MCPServerInfo struct { // API view: header and env values masked as "set" or "${secret:NAME}"
      config.MCPServer
  }
  // the mcp types stay inside the service
  type MCPTestResult struct{ Tools []mcp.Tool; Resources []mcp.Resource; Prompts []mcp.Prompt }
  func (s *Service) MCPServers() []MCPServerInfo
  // Check; 409 if the name exists; writes ConfigPath
  func (s *Service) AddMCPServer(srv config.MCPServer) (MCPServerInfo, error)
  // 404 if unknown; 409 if a notebook attaches it
  func (s *Service) RemoveMCPServer(name string) error
  func (s *Service) TestMCPServer(ctx context.Context, name string) (MCPTestResult, error)
  func (s *Service) connectMCP(ctx context.Context, name string) (*mcp.Client, error) // expands secrets, connects
  ```
- Writing the config: reload the file at `ConfigPath`, replace only `mcp`, then `Save` (atomic, 0600), all under
  `cfgMu`. With an empty `ConfigPath` (tests, embedded use), only memory is changed.

- [ ] **Step 1: Write the failing tests**
  - `TestAddListRemoveMCP`: add writes the file; the list masks a header `Bearer ${secret:GH}` as itself and a
    literal value as `set`; remove deletes it.
  - `TestAddMCPDuplicate409`.
  - `TestRemoveAttachedMCP409`: a notebook with `agent.mcpServers: ["gh"]` blocks the remove.
  - `TestTestMCPServerUsesSecrets`: the helper stdio server echoes its env, the expanded secret reaches the child
    env, and the error and result text never contain the secret.
  - `TestMCPErrorsRedacted`: a server that fails the handshake and prints the secret to stderr gives an error with
    the secret redacted.
- [ ] **Step 2: Run** `go test ./internal/service -run MCP`. Expected: FAIL.
- [ ] **Step 3: Implement.** Set `ConfigPath` in `cli.newService` (from `env.ConfigPath`).
- [ ] **Step 4: Run** `go test -race ./internal/service ./internal/cli`. Expected: PASS.
- [ ] **Step 5: Commit** `feat(service): add, list, remove and test MCP servers`

### Task 5: CLI and API for servers (MCP-1)

**Files:**
- Create: `internal/cli/cmd_mcp.go`, `internal/server/mcp.go`
- Modify: `internal/cli/cli.go` (command `mcp`, usage), `internal/server/server.go` (routes), `docs/api.md`
- Test: `internal/cli/mcp_test.go`, `internal/server/mcp_test.go`

**Interfaces:**
- CLI:
  - `tess mcp add <name> (--command PROG [--arg A]... | --url URL) [--env K=V]... [--header K=V]...`
    `[--allow-insecure-http]`
  - `tess mcp list` (table: NAME, TRANSPORT, TARGET)
  - `tess mcp test <name>` (counts, then the names of tools, resources and prompts)
  - `tess mcp remove <name>`
- API:
  - `GET /v1/mcp/servers` returns `{servers}`.
  - `POST /v1/mcp/servers` returns `201 MCPServerInfo` (400 or 409).
  - `DELETE /v1/mcp/servers/{name}` returns `204` (404 or 409).
  - `POST /v1/mcp/servers/{name}/test` returns `{tools:[{name,description}], resources:[{uri,name,mimeType}],
    prompts:[{name,description}]}`. It takes a run slot and has a 60 s total timeout.

- [ ] **Step 1: Write the failing tests**
  - CLI: `TestMCPAddListTestRemove` (a helper stdio server); `TestMCPAddRejectsLiteralToken` (exit 2);
    `TestMCPUsage`.
  - Server: `TestMCPServersAPI` (201, list, test, 204); `TestMCPAddConflict409`; `TestMCPTestUnknown404`.
- [ ] **Step 2: Run** `go test ./internal/cli ./internal/server -run MCP`. Expected: FAIL.
- [ ] **Step 3: Implement.** Document the routes and the masking rule in `docs/api.md`.
- [ ] **Step 4: Run** `go test -race ./internal/cli ./internal/server`. Expected: PASS.
- [ ] **Step 5: Commit** `feat(cli,server): tess mcp and /v1/mcp/servers`

### Task 6: Notebook attachment and agent tools (MCP-2)

**Files:**
- Create: `internal/agenttools/mcp.go` (adapter), `internal/service/agent_mcp.go`
- Modify:
  - `internal/notebook/notebook.go` (`AgentSettings.MCPServers []string \`json:"mcpServers,omitempty"\``)
  - `internal/service/agent.go` (`buildAgent` adds MCP tools and returns a closer; the run closes it on finish or
    pause)
  - `internal/cli/cmd_notebook.go` (`tess mcp attach|detach <notebook> <server>`, in `cmd_mcp.go`)
  - `internal/server/server.go` (`PUT /v1/notebooks/{id}/mcp` with body `{servers:[name]}` returns `Notebook`)
- Test: `internal/agenttools/mcp_test.go`, `internal/service/agent_mcp_test.go`

**Interfaces:**
- Produces:
  ```go
  // agenttools: the agent sees plain Tools; MCP types stay behind this interface
  type MCPCaller interface {
      CallTool(ctx context.Context, name string, args json.RawMessage) (text string, isError bool, err error)
  }
  type MCPToolSpec struct{ Server, Name, Description string; Schema json.RawMessage }
  // names: mcp__<server>__<tool>, sanitized and deduped
  func MCPTools(server string, specs []MCPToolSpec, c MCPCaller) []Tool
  func SanitizeToolName(server, tool string) string
  // service
  // tools for the notebook's attached servers; closeAll stops their connections
  func (s *Service) mcpTools(ctx context.Context, nb *notebook.Notebook,
  ) (tools []agenttools.Tool, closeAll func(), err error)
  func (s *Service) SetNotebookMCP(notebookID string, servers []string) (NotebookInfo, error) // names must exist
  ```
  - An attached server that fails to connect fails the agent run at start, with an error naming the server. It is
    never skipped silently.
  - The schema is passed through as the tool's parameters when it is a JSON object ≤ 16 KiB; otherwise
    `{"type":"object"}`.
  - A result with `isError` is returned to the model as `error: <text>`.

- [ ] **Step 1: Write the failing tests**
  - `TestSanitizeToolName` (table, including the 64-character cut) and `FuzzSanitizeToolName` (the output always
    matches `^[a-z0-9_]{1,64}$`).
  - `TestMCPToolsDedupe`: two tools that sanitize to the same name give `_2`.
  - `TestMCPToolRunsAndCapsResult`: a fake caller returns 100 KiB, and the agent's 32 KiB cap still applies through
    `runTool`.
  - `TestAgentRunCallsAttachedMCPTool` (service, helper stdio server): the scripted model calls
    `mcp__fake__echo`; the rule is `allow` through the run rules; the result reaches the next request; and the audit
    has `tool.call` with the tool name and a digest.
  - `TestAgentMCPDefaultsToAsk`: without rules, the call pauses with an approval.
  - `TestMCPServersClosedOnPauseAndReopenedOnResume`: the helper process is gone after the pause, a new process runs
    after the resume, and the tool name is unchanged.
  - `TestAgentAttachedServerDownFailsRun`: the error names the server.
  - `TestSetNotebookMCPUnknown400`.
- [ ] **Step 2: Run** `go test ./internal/agenttools ./internal/service -run 'MCP|Sanitize'`. Expected: FAIL.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run** `go test -race ./internal/agenttools ./internal/service ./internal/executor/...` and
  `go test -run=^$ -fuzz=FuzzSanitizeToolName -fuzztime=20s ./internal/agenttools`. Expected: PASS.
- [ ] **Step 5: Commit** `feat(agent): attach MCP servers to notebooks; their tools in agent runs`

### Task 7: MCP resources as context (MCP-3)

**Files:**
- Modify:
  - `internal/notebook/notebook.go` (`ContextItem.Kind string \`json:"kind,omitempty"\`` with "" meaning a path,
    plus `Server`, `URI string`; `AddMCPResource(server, uri, label string) (ContextItem, error)`)
  - `internal/service/service.go` (`notebookDocs` becomes `(s *Service) notebookDocs(ctx, nb)`, reading resources
    through `connectMCP` and one connection per server per call; update every caller: `buildSpec`, chat
    attachments, `threadDocs`)
  - `internal/service/workspace.go` (`AddMCPContext`)
  - CLI `tess context add <notebook> mcp://<server>/<uri>`; API `POST /v1/notebooks/{id}/context` also takes
    `{server, uri, label?}`
- Test: `internal/service/mcp_context_test.go`, `internal/notebook/notebook_test.go`

**Interfaces:**
- Produces: `func (s *Service) AddMCPContext(notebookID, server, uri, label string) (NotebookInfo, error)`. The
  server must exist and the URI must be ≤ 2048 characters.
- Each enabled `mcp-resource` item becomes `extract.Document{Source: "mcp:<server>/<uri>", Kind: "text", Text}`. It
  counts toward the budget like any other source. A read error fails the run with
  `mcp resource <server> <uri>: <cause>` (redacted).

- [ ] **Step 1: Write the failing tests**
  - `TestPipelineRunReadsMCPResource`: the helper server serves `file:///notes.md`, the run's request contains its
    text, and `run.Context` lists `mcp:fake/file:///notes.md`.
  - `TestMCPResourceErrorFailsRun`.
  - `TestMCPResourceTooLarge`: 3 MiB gives an error naming the 2 MiB cap.
  - `TestAddMCPContextUnknownServer400`.
  - `TestContextItemOldManifestLoads`: a manifest without `kind` loads as a path item.
  - `TestDisabledMCPResourceNotRead`: no connection is made.
- [ ] **Step 2: Run** `go test ./internal/service ./internal/notebook -run 'MCP|OldManifest'`. Expected: FAIL.
- [ ] **Step 3: Implement.** The token estimate in `Describe` for `mcp-resource` items is 0 (unknown until read),
  and the desktop shows "read at run time".
- [ ] **Step 4: Run** `go test -race ./...`. Expected: PASS.
- [ ] **Step 5: Commit** `feat(context): MCP resources as notebook context`

### Task 8: Desktop: Settings › MCP servers, notebook attachment and resources

**Files:**
- Create: `desktop/lib/src/screens/mcp_settings.dart`, `desktop/lib/src/core/mcp_models.dart`
- Modify: `core_client.dart` (interface and demo), `http_core_client.dart`, `json_mapping.dart`,
  `settings_screen.dart` (an "MCP servers" section), `notebook_sources_pane.dart` (an "MCP servers" card with
  attach toggles, and "Add resource" in Sources)
- Test: `desktop/test/mcp_test.dart`

**Interfaces:**
- `CoreClient` gains:
  - `listMcpServers()`
  - `addMcpServer(McpServerInput)`
  - `removeMcpServer(name)`
  - `testMcpServer(name)` returning `McpTestResult(tools, resources, prompts)`
  - `setNotebookMcp(notebookId, List<String>)`
  - `addMcpContext(notebookId, server, uri)`
- Keys:
  - `mcp-add`, `mcp-name`, `mcp-command`, `mcp-url`, `mcp-save`
  - `mcp-test-<name>`, `mcp-remove-<name>`
  - `nb-mcp-<name>` (attach switch)
  - `context-add-mcp`
- The add form takes headers and env as `KEY=VALUE` lines. A hint explains `${secret:NAME}`. The form never shows a
  stored value, only the masked form from the API.

- [ ] **Step 1: Write the failing tests** (demo client and an HTTP fake)
  - JSON mapping.
  - Add, test, remove in settings.
  - The test dialog lists tool names.
  - Attaching in the notebook calls `setNotebookMcp`.
  - Adding a resource shows it in Sources marked "read at run time".
  - A 409 on remove shows the server's message.
- [ ] **Step 2: Run** `flutter test test/mcp_test.dart` in `desktop/`. Expected: FAIL.
- [ ] **Step 3: Implement.** Keep every file ≤ 500 lines.
- [ ] **Step 4: Run** `task desktop:check`. Expected: PASS, coverage ≥ 80%.
- [ ] **Step 5: Commit** `feat(desktop): MCP server settings, notebook attachment and resources`

### Task 9: Docs and full gate

**Files:** `docs/architecture.md` (an `mcp` package row, the config `mcp` block, server lifecycle),
`docs/security-model.md` (rows: MCP server processes, credentials in headers and env, hostile MCP results),
`docs/checklist.md` (MCP-1, MCP-2, MCP-3 checked), `docs/adr/0005-agent-loop-mcp-acp.md` (the ported Zero commit),
`docs/adr/0003-dependency-policy.md` (no new dependency), and `CLAUDE.md` (layout: `internal/mcp`).

- [ ] **Step 1: Update the docs.**
- [ ] **Step 2: Run** `task ci`. Expected: all gates green.
- [ ] **Step 3: Commit** `docs: MCP client, settings, attachment and resources`

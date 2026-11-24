# Tessivo user guide

Tessivo analyzes, summarizes, documents and reports on your files, code and notes with the models you choose:
a self-hosted model behind an OpenAI-compatible endpoint (LiteLLM, vLLM, Ollama and the like), or a hosted one
(OpenAI-compatible, Anthropic, Gemini). It is a desktop app and a command-line tool, `tess`, that share one core.
Your notebooks stay in folders on your disk; keys stay in your OS keychain.

## Install

Download from the project's GitHub Releases page.

| OS | Installer | Also available |
| --- | --- | --- |
| Windows (x64) | `tessivo-desktop_<version>_windows_amd64_setup.exe` installs for you only (no admin rights); choose "all users" in the first screen if you want | `.zip` to unpack anywhere |
| macOS (Apple silicon) | `tessivo-desktop_<version>_macos_arm64.dmg`: open it and drag Tessivo to Applications | `.tar.gz` |
| Debian, Ubuntu (x64) | `tessivo-desktop_<version>_linux_amd64.deb`: `sudo apt install ./tessivo-desktop_…deb` | `.AppImage` (any distribution: make it executable and run it), `.tar.gz` |

The desktop app includes `tess`. On Linux the `.deb` puts `tess` and `tessivo` on your PATH. On Windows and macOS,
`tess` sits next to the app (`%LOCALAPPDATA%\Programs\Tessivo` or `Tessivo.app/Contents/MacOS`). If you only want
the CLI, download `tess_<version>_<os>_<arch>`.

Check a download against `SHA256SUMS`: `sha256sum -c SHA256SUMS --ignore-missing`.

**Unsigned builds.** Until a release is signed, the first start shows a warning.
- On Windows, choose *More info → Run anyway*.
- On macOS, right-click the app, choose *Open*, then *Open* again.

Linux tray icon: GNOME needs the AppIndicator extension.

## The window

Tessivo's window is laid out like a code editor:

- **Header**: where you are (for example *Notebooks / Change risk review*), the search box, *Ctrl K* for the command
  palette, approvals waiting and notifications.
- **Activity bar** on the left: Notebooks, Research, Chat, Schedules, Templates, Outputs, and Settings at the bottom.
- **Side bar**: the list for that section, for example your notebooks and the open notebook's outputs. Drag its
  edge to resize it.
- **Page**: the open notebook, chat or setting.
- **Panel** on the right: the conversation about the open output, or *In this chat* on a chat.
- **Status bar** at the bottom: whether the core is connected, the default model, today's tokens, the next
  scheduled run and approvals waiting. Click an item to go there.

| Keys (⌘ on macOS) | Does |
| --- | --- |
| Ctrl K | Command palette: commands, notebooks, and search for what you type |
| Ctrl B | Show or hide the side bar |
| Ctrl J | Show or hide the panel |
| Ctrl N | New notebook |
| Ctrl , | Settings |
| Ctrl = / Ctrl - / Ctrl 0 | Zoom in, out, back to 100 % |

The side bar, the panel and their widths are remembered. In a narrow window they close; Ctrl B and Ctrl J still
open them. **Settings → Appearance** chooses the theme (system, light, dark or high contrast) and the zoom
(80–200 %). Text also follows the text size you set in your operating system (on Windows, *Settings →
Accessibility → Text size*).

**When the core is not running.** The window says *Starting the Tessivo core…* while it starts. If it cannot start,
the window says what happened, with *Retry*, *Copy details* for a bug report, and how to start it yourself with
`tess serve`. If the core stops while you work, the status bar shows *Reconnecting…* and the app reconnects on its
own; your work is kept.

## Set up a model

Until a model is set up, the Notebooks page shows *Connect a model*, which opens **Settings → Models**. A notebook
names an **alias** (for
example `smart`). The alias points to one or more models in order, so when a model is down, rate-limited or out
of context, Tessivo tries the next one.

1. **Add a provider.** Choose *Add provider* and pick a preset: LiteLLM, OpenAI, Anthropic, Gemini, Ollama,
   LM Studio, vLLM, OpenRouter, Groq, DeepSeek or Custom. Anthropic and Gemini use their own APIs (prompt caching,
   thinking and tools included); everything else uses the OpenAI-compatible API.
2. **Set its API key.** The key goes to the OS keychain: Windows Credential Manager, macOS Keychain, or the Secret
   Service on Linux. It is never written to the config file.
3. **Choose models for each alias.** Use *Edit…* on the alias to list them in order of preference, then *Test* to
   send each one a short request and see which answer (`tess models test <alias>` in the terminal).

From the terminal:

```sh
tess config init                       # writes a config with a LiteLLM provider at http://localhost:4000/v1
tess secret set TESSIVO_LITELLM_KEY    # reads the key from stdin into the keychain
tess models                            # lists what the providers offer
tess config path                       # where the config file lives, to add providers by hand
```

The app (and `tess serve`) starts on default settings when there is no config file yet; the first save in
Settings writes it. Other commands ask you to run `tess config init` first.

A provider in the config file looks like this; `kind` is `openai` (the default), `anthropic` or `gemini`, and the
native kinds need no `base_url`:

```json
{"name": "claude", "kind": "anthropic", "api_key_ref": "ANTHROPIC_API_KEY", "max_concurrent": 4}
```

**Rate limits and failures.** Each alias can set `max_retries`; the default is 2 retries per model. A wait the
provider asks for (`Retry-After`) is honoured for up to 30 seconds, with jitter. A longer wait moves on to the next
model. `max_concurrent` caps parallel calls to one provider.

## Notebooks

A notebook is one recurring job: a work type, its sources, prompts, pinned memory, runs, outputs and schedules,
all in one folder you can copy or put under version control.

| Work type | Answers |
| --- | --- |
| Analyze | What is wrong or risky here, and what should we do? |
| Summarize | What matters in all of this? |
| Document | How does this work, written up for others? |
| Report | What happened, for a reader who was not there? |

Create one with *New* in the app, start from a template, or run `tess new <dir> --type analyze`.

*Saved prompts* on the Prompt tab saves prompts, and *From the library…* fills the box with a starter prompt
(change risk, triage, action items, status report and others; `tess prompt library` in the terminal).

### Sources

Choose *Add* on the Sources tab, or *More sources* for more kinds:

- **Files and folders.** Files are copied into the notebook; folders and code bases are referenced in place and
  read on every run. Supported: text, Markdown, CSV, JSON, code, Word, Excel, PowerPoint, PDF, `.diff` and
  `.patch` files.
- **Web pages**, fetched again on every run.
- **Pasted text**, saved in the notebook as Markdown.
- **Git changes.** The diff of a repository against a branch, tag or commit (for example everything on your branch
  since `main`), read with git on every run.
- **MCP resources** from the MCP servers you configured.

Untick a source to leave it out of runs without removing it. In the terminal:
`tess context add <notebook> <path|url>`, `--text -`, or `--git-diff <repo> --base main`.

### Runs and outputs

*Run* sends the prompt with the sources and pinned memory to the notebook's alias. The result is a structured
output that is checked against the work type, then saved with its sources, model, settings and token use. From an
output you can:

- **Talk to it.** Ask questions about it, refine it, and accept suggested improvements to the notebook's prompt or
  memory.
- **Run it again**, on the same model or another one (*Run again*, or `tess output rerun --model`).
- **Compare** it with the version it revised (*Compare*, or `tess output diff`).
- **Edit** its fields in a form and save them as a new version, without running the model again (*Edit*, or
  `tess output data` and `tess output edit`).
- **Export** it as Markdown, Word or PDF (`tess output export --format md|docx|pdf`).

### Agent mode

Agent mode lets the model explore code and documents and call MCP tools before it writes the output. Limits on
steps, tool calls, tokens and time, and the default rule for each tool, are in **Settings → Agent**. Every tool
has a rule: *allow*, *ask* or *block*. The agent can also run programs (`run_command`, without a shell); list the
commands that may run without asking on the notebook's **Agent** tab (`go test *`, where `*` is one
argument), or with `tess notebook commands`. An *ask* pauses the run until you answer in the app or with
`tess run approve|deny`, even if the run was scheduled.

### Schedules

Schedules run a notebook on a cron expression, for example `0 8 * * MON`. They run while the app, or `tess serve`,
is open. A schedule can skip a time it missed while closed, or run once to catch up. To keep scheduled work going
with the window closed, turn on **Settings → Desktop → Keep running in the tray**. Closing the window then hides
it; the tray menu opens Tessivo again or quits it.

## Chat and research

- **Chat** answers questions with attachments: files, folders, notebooks, outputs or web pages (*Attach* in the
  message box). Turn on MCP tools for a chat with *Tools* in the message box; tools that are not allowed by a rule
  ask you first, right in the conversation. Each tool call shows as one line you can open to see its output, tagged
  *allow-listed* when a rule let it run or *approved* when you did. The panel lists what the chat has read, its
  tools and the tool activity. A good chat can become a notebook (*Make this repeatable*) or a saved output.
- **Research notebooks** collect sources, summarize each one, and answer questions with citations. The tabs are
  *Sources* (set each one to Full, Summary or Off), *Ask* and *Notes*. Keep the answers you like as notes.
- **Approvals** (the header button or the status bar) lists every tool call waiting for you, with *Allow*, more
  ways to allow, and *Deny*.
- **Search** (the box at the top, or `tess search <words>`) looks through notebooks, outputs, prompts and research
  notes.

## Privacy and safety

- Sources are sent only to the models you configure. Nothing else leaves your machine, and there is no telemetry.
- Keys live in the OS keychain. Tessivo redacts them from logs, errors and the audit log; `tess audit verify`
  checks that the audit log has not been changed.
- Files, archives, PDFs, web pages, model output and tool results are all treated as untrusted and bounded in
  size and time. PDFs are parsed in a WebAssembly sandbox.
- MCP servers start without your shell environment's secrets. When Tessivo closes a server, it stops every process
  that server started.

## Troubleshooting

- **The model does not answer.** Press *Test* on the alias in Settings, or run `tess models test <alias>`. The error
  says whether the key, the URL or the model name is wrong (401, 404) or whether the provider is rate-limiting
  you (429).
- **A bug report.** *Settings → Save diagnostics…* or `tess diagnostics` writes a zip with versions, settings (key
  names only) and the end of the log. It contains no keys, notebook names or content.
- **Token limits.** `tess usage` shows today's use. Set limits in Settings or with
  `tess config set limits.max_tokens_per_day 50000`.

import 'dart:async';
import 'dart:io';

import 'package:material_ui/material_ui.dart';

import 'chat_state.dart';
import 'connection.dart';
import 'core/core_client.dart';
import 'core/models.dart';
import 'core/settings_draft.dart';
import 'design/theme.dart';
import 'design/zoom.dart';
import 'research_state.dart';
import 'shell/layout_prefs.dart';
import 'thread_state.dart';

part 'app_state_extras.dart';
part 'app_state_sources.dart';
part 'app_state_appearance.dart';
part 'app_state_layout.dart';
part 'app_state_organize.dart';

enum Section { chat, notebooks, research, templates, schedules, library, settings, approvals }

/// The parts of the Settings page, listed in its side bar.
enum SettingsSection { models, defaults, agent, mcp, appearance, desktop, diagnostics }

/// App-wide state on a plain [ChangeNotifier] (no state-management package).
class AppState extends ChangeNotifier {
  AppState(this.core);

  final CoreClient core;

  Section section = Section.notebooks;
  List<Notebook> notebooks = const [];
  String? selectedId;
  bool loading = true;

  // Current run.
  bool running = false;
  final StringBuffer _stream = StringBuffer();
  String status = '';
  RunOutcome? outcome;

  String get streamedText => _stream.toString();

  // Agent mode: tool calls of the current run, and approvals it waits for.
  final List<String> toolLog = [];
  List<PendingApproval> paused = const [];

  // Approvals waiting across all notebooks (sidebar badge).
  List<PendingApproval> approvals = const [];

  /// Reloads pending approvals; errors keep the last list.
  Future<void> refreshApprovals() async {
    try {
      approvals = await core.pendingApprovals();
    } on Object {
      return;
    }
    if (!_disposed) notifyListeners();
  }

  /// Answers an approval. [remember] is `run` or `notebook`.
  Future<void> answer(PendingApproval a, {required bool allow, String? remember}) =>
      _act(() => core.answerApproval(a.runId, a.approvalId, allow: allow, remember: remember), a);

  /// Resumes an interrupted run in the core.
  Future<void> resumeRun(PendingApproval a) => _act(() => core.resumeRun(a.runId), a);

  /// Cancels a paused or interrupted run.
  Future<void> cancelRun(PendingApproval a) => _act(() => core.cancelRun(a.runId), a);

  /// Runs an approval action; a refusal (expired, already answered, in use)
  /// becomes a notice, and the list is reloaded either way.
  Future<void> _act(Future<void> Function() action, PendingApproval a) async {
    try {
      await action();
      paused = [
        for (final p in paused)
          if (p.runId != a.runId || p.approvalId != a.approvalId) p,
      ];
    } on CoreException catch (e) {
      notices.add('Could not update ${a.tool.isEmpty ? 'the run' : a.tool}: ${e.message}');
    }
    await refreshApprovals();
    notifyListeners();
  }

  // Notifications from the core's event stream (newest last, capped).
  final List<String> notices = [];
  StreamSubscription<CoreEvent>? _events;
  int _attempt = 0;

  /// True while the core is unreachable (its event stream broke); the app
  /// keeps retrying and clears it once the core answers again.
  bool connectionLost = false;

  /// Theme and zoom from `desktop.theme` / `desktop.zoom` (Settings → Appearance).
  TvThemeMode themeMode = TvThemeMode.system;
  double zoom = 1;

  /// Workbench panels (`desktop.layout`), the default model and today's usage for the status bar.
  LayoutPrefs layout = const LayoutPrefs();

  /// The side bar or panel opened by its shortcut in a window too narrow for
  /// it; not remembered.
  bool sideBarPeek = false;
  bool panelPeek = false;

  /// The part of Settings that is open.
  SettingsSection settingsSection = SettingsSection.models;
  String? defaultModel;
  UsageInfo? usage;

  /// The next scheduled run: its notebook and when (as the core says it).
  ({String notebook, String at})? nextRun;

  /// Subscribes to core events and reconnects with capped backoff.
  void listenForEvents() {
    unawaited(_events?.cancel());
    _events = core.events().listen(
      (e) {
        if (e.isResearchEvent) return _researchEvent(e);
        if (e.isRunEvent || e.kind == 'schedule.paused') unawaited(refreshApprovals());
        if (e.kind == 'schedule.started' || e.kind == 'run.resumed') return; // only outcomes are worth a notice
        notices.add(e.message);
        if (notices.length > 20) notices.removeAt(0);
        notifyListeners();
      },
      onError: (Object _) => _reconnect(),
      onDone: _reconnect,
      cancelOnError: true,
    );
  }

  void _reconnect() {
    if (!connectionLost) {
      connectionLost = true;
      notifyListeners();
    }
    final wait = reconnectDelay(_attempt++);
    Future<void>.delayed(wait, () async {
      if (_events == null) return;
      try {
        await core.getSettings(); // a quiet stream says nothing, so ask
      } on Object {
        return _reconnect();
      }
      _attempt = 0;
      connectionLost = false;
      notifyListeners();
      listenForEvents();
    });
  }

  void dismissNotice() {
    if (notices.isNotEmpty) {
      notices.removeLast();
      notifyListeners();
    }
  }

  /// Stops listening; pending reconnects become no-ops.
  void stopEvents() {
    unawaited(_events?.cancel());
    _events = null;
  }

  bool _disposed = false;

  @override
  void dispose() {
    _disposed = true;
    stopEvents();
    thread?.dispose();
    researchState?.dispose();
    chat.dispose();
    super.dispose();
  }

  Notebook? get selected {
    for (final n in notebooks) {
      if (n.id == selectedId) return n;
    }
    return null;
  }

  Future<void> load() async {
    await loadAppearance();
    notebooks = await core.listNotebooks();
    selectedId ??= notebooks.isEmpty ? null : notebooks.first.id;
    loading = false;
    notifyListeners();
    await refreshApprovals();
    if (selectedId case final id?) await _refresh(id);
    await refreshMcpServers();
    await loadOutputs();
    await loadPrompts();
    await loadResearch();
    await checkSetup();
    await refreshUsage();
    await refreshNextRun();
  }

  void go(Section s) {
    section = s;
    notifyListeners();
  }

  void select(String id) {
    selectedId = id;
    section = Section.notebooks;
    outcome = null;
    _stream.clear();
    outputs = const [];
    _openOutput(null);
    unawaited(_refresh(id));
    unawaited(loadOutputs());
    prompts = const [];
    unawaited(loadPrompts());
  }

  /// Loads the full notebook (context stats, memory); the list view omits them.
  Future<void> _refresh(String id) async {
    try {
      _put(await core.getNotebook(id));
    } on Object {
      // Keep the summary we have; the next action surfaces any real error.
    }
  }

  void _put(Notebook updated) {
    if (_disposed) return;
    notebooks = [for (final n in notebooks) n.id == updated.id ? updated : n];
    notifyListeners();
  }

  /// Configured MCP servers (loaded with the notebooks; refreshed on demand).
  List<McpServer> mcpServers = const [];

  Future<void> createNotebook(String name, WorkTypeId type) async {
    final nb = await core.createNotebook(name: name, workType: type);
    notebooks = await core.listNotebooks();
    select(nb.id);
  }

  /// Research notebooks (sidebar RESEARCH group) and the one open.
  List<ResearchNotebook> research = const [];
  ResearchState? researchState;

  /// Reloads the research notebooks; an older core without them shows none.
  Future<void> loadResearch() async {
    try {
      research = await core.listResearch();
    } on Object {
      research = const [];
    }
    if (!_disposed) notifyListeners();
  }

  /// Opens a research notebook.
  void selectResearch(String id) {
    researchState?.dispose();
    final r = researchState = ResearchState(core, id, onError: _notice, onChanged: _putResearch);
    section = Section.research;
    notifyListeners();
    unawaited(r.load());
  }

  void _putResearch(ResearchNotebook nb) {
    research = [for (final r in research) r.id == nb.id ? nb : r];
    if (!_disposed) notifyListeners();
  }

  /// Creates a research notebook and opens it.
  Future<void> createResearch(String name, {String description = ''}) async {
    final nb = await core.createResearch(name, description: description);
    await loadResearch();
    selectResearch(nb.id);
  }

  /// A summary started, is ready or failed: the open notebook reloads, and only a failure is a notice.
  void _researchEvent(CoreEvent e) {
    final open = researchState;
    if (open != null && open.notebookId == e.notebookId) unawaited(open.reload());
    if (e.kind == 'research.summary.failed') _notice(e.message);
  }

  /// Whether the sidebar lists archived notebooks.
  bool showArchived = false;

  /// Text typed in the side bar's tag filter; not remembered.
  String tagFilter = '';

  void toggleArchived() {
    showArchived = !showArchived;
    notifyListeners();
  }

  /// Renames a notebook; a refusal becomes a notice.
  Future<void> renameNotebook(String id, String name) => _manage(id, () => core.updateNotebook(id, name: name));

  /// Archives or restores a notebook; a refusal becomes a notice.
  Future<void> setArchived(String id, bool archived) => _manage(id, () => core.updateNotebook(id, archived: archived));

  Future<void> _manage(String id, Future<Notebook> Function() action) async {
    try {
      final u = await action();
      final old = notebooks.where((n) => n.id == id).firstOrNull;
      _put(old == null ? u : old.copyWith(name: u.name, archived: u.archived, tags: u.tags, pinned: u.pinned));
    } on CoreException catch (e) {
      _notice(e.message);
    }
  }

  /// Deletes a notebook (the core refuses while a run is active or paused)
  /// and opens the next one.
  Future<void> deleteNotebook(String id) async {
    try {
      await core.deleteNotebook(id);
    } on CoreException catch (e) {
      _notice(e.message);
      return;
    }
    notebooks = await core.listNotebooks();
    if (selectedId != id) {
      notifyListeners();
      return;
    }
    final next = notebooks.where((n) => !n.archived).firstOrNull;
    if (next != null) {
      select(next.id);
      return;
    }
    selectedId = null;
    outputs = const [];
    _openOutput(null);
  }

  /// Chat view state, created on first use.
  late final ChatState chat = ChatState(core);

  /// Reloads the notebook list and opens [id] (e.g. a notebook made from a chat).
  Future<void> openNotebook(String id) async {
    notebooks = await core.listNotebooks();
    select(id);
  }

  /// Creates a notebook from a template and opens it.
  Future<void> createFromTemplate(String templateId, {String? name}) async {
    final nb = await core.createFromTemplate(templateId, name: name);
    notebooks = await core.listNotebooks();
    select(nb.id);
  }

  Future<void> toggleContext(String contextId, bool enabled) async {
    final nb = selected;
    if (nb == null) return;
    _put(await core.setContextEnabled(notebookId: nb.id, contextId: contextId, enabled: enabled));
  }

  /// The saved prompt the last run used, if any.
  String? lastPromptName;

  /// The open notebook's saved prompts.
  List<SavedPrompt> prompts = const [];

  /// What the user must set up before runs can work (shown as a banner), or null.
  String? setupMessage;

  void _changed() {
    if (!_disposed) notifyListeners();
  }

  /// The model the last run asked for, if any (null: the settings that apply).
  String? lastModel;

  Future<void> run(String prompt, Map<String, String> vars, {String? promptName, String? model}) async {
    final nb = selected;
    if (nb == null || running) return;
    running = true;
    lastPromptName = promptName;
    lastModel = model;
    outcome = null;
    _stream.clear();
    toolLog.clear();
    paused = const [];
    notifyListeners();
    try {
      await for (final e in core.startRun(
        notebookId: nb.id,
        prompt: prompt,
        vars: vars,
        promptName: promptName,
        model: model,
      )) {
        switch (e) {
          case RunStatus(:final text):
            status = text;
          case RunDelta(:final text):
            _stream.write(text);
          case RunFinished(outcome: final o):
            outcome = o;
          case RunToolCall(:final tool, :final args):
            toolLog.add('→ $tool $args');
          case RunToolResult(:final tool, :final result):
            toolLog.add('← $tool: ${result.split('\n').first}');
          case RunPaused(approvals: final list):
            paused = list;
            unawaited(refreshApprovals());
        }
        notifyListeners();
      }
    } on Object catch (err) {
      status = 'Run failed: $err';
    } finally {
      running = false;
      notifyListeners();
      unawaited(loadOutputs(latest: true));
    }
  }

  /// The selected notebook's outputs (newest first) and the one open.
  List<OutputSummary> outputs = const [];
  String? selectedOutputId;

  /// The open output's thread, or null when no output is open.
  ThreadState? thread;

  /// Reloads the outputs; keeps the open one, else opens the newest
  /// ([latest] opens the newest anyway, e.g. after a run; [open] a given one).
  Future<void> loadOutputs({bool latest = false, String? open}) async {
    final nb = selected;
    if (nb == null) return;
    try {
      outputs = await core.listOutputs(notebookId: nb.id);
    } on Object {
      outputs = const [];
    }
    if (_disposed || selected?.id != nb.id) return;
    final keep = !latest && outputs.any((o) => o.id == selectedOutputId);
    final want = open ?? (keep ? selectedOutputId : outputs.firstOrNull?.id);
    if (want == selectedOutputId && thread != null) {
      notifyListeners();
    } else {
      _openOutput(want);
    }
  }

  /// Opens an output and its thread.
  void selectOutput(String id) => _openOutput(id);

  void _openOutput(String? id) {
    thread?.dispose();
    thread = null;
    selectedOutputId = id;
    final nb = selected;
    if (id != null && nb != null) {
      final t = thread = ThreadState(
        core,
        nb.id,
        id,
        onError: _notice,
        onNotice: _notice,
        onApplied: () => _refresh(nb.id),
      );
      unawaited(t.load());
    }
    if (!_disposed) notifyListeners();
  }

  void _notice(String message) {
    notices.add(message);
    if (!_disposed) notifyListeners();
  }

  /// Saves a revised version of the open output and opens it.
  Future<void> newVersion() async {
    final v = await thread?.newVersion();
    if (v != null) await loadOutputs(open: v.id);
  }

  /// Saves the open output's structured result as edited, as a new version, and opens it.
  Future<void> editOutput(Map<String, Object?> data) async {
    final t = thread;
    if (t == null) return;
    final v = await core.editOutput(t.notebookId, t.outputId, data);
    await loadOutputs(open: v.id);
  }
}

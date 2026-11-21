import 'dart:async';

import 'package:material_ui/material_ui.dart';

import 'chat_state.dart';
import 'core/core_client.dart';
import 'core/models.dart';

enum Section { chat, notebooks, templates, schedules, library, settings }

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
  Future<void> answer(PendingApproval a, {required bool allow, String? remember}) async {
    await core.answerApproval(a.runId, a.approvalId, allow: allow, remember: remember);
    paused = [
      for (final p in paused)
        if (p.approvalId != a.approvalId) p,
    ];
    await refreshApprovals();
    notifyListeners();
  }

  // Notifications from the core's event stream (newest last, capped).
  final List<String> notices = [];
  StreamSubscription<CoreEvent>? _events;
  Duration _retry = const Duration(seconds: 2);

  /// Subscribes to core events and reconnects with capped backoff.
  void listenForEvents() {
    unawaited(_events?.cancel());
    _events = core.events().listen(
      (e) {
        _retry = const Duration(seconds: 2);
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
    final wait = _retry;
    _retry = _retry * 2 > const Duration(minutes: 1) ? const Duration(minutes: 1) : _retry * 2;
    Future<void>.delayed(wait, () {
      if (_events != null) listenForEvents();
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
    notebooks = await core.listNotebooks();
    selectedId ??= notebooks.isEmpty ? null : notebooks.first.id;
    loading = false;
    notifyListeners();
    await refreshApprovals();
    if (selectedId case final id?) await _refresh(id);
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
    notifyListeners();
    unawaited(_refresh(id));
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

  /// Adds a file or folder by absolute path to the selected notebook.
  Future<void> addContext(String path) async {
    final nb = selected;
    if (nb != null) _put(await core.addContext(nb.id, path));
  }

  /// Replaces the selected notebook's memory.
  Future<void> setMemory(List<String> memory) async {
    final nb = selected;
    if (nb != null) _put(await core.setMemory(nb.id, memory));
  }

  Future<void> createNotebook(String name, WorkTypeId type) async {
    final nb = await core.createNotebook(name: name, workType: type);
    notebooks = await core.listNotebooks();
    select(nb.id);
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

  Future<void> run(String prompt, Map<String, String> vars) async {
    final nb = selected;
    if (nb == null || running) return;
    running = true;
    outcome = null;
    _stream.clear();
    toolLog.clear();
    paused = const [];
    notifyListeners();
    try {
      await for (final e in core.startRun(notebookId: nb.id, prompt: prompt, vars: vars)) {
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
    }
  }
}

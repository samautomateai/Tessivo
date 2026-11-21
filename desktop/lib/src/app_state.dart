import 'dart:async';

import 'package:material_ui/material_ui.dart';

import 'core/core_client.dart';
import 'core/models.dart';

enum Section { notebooks, templates, schedules, library, settings }

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

  // Notifications from the core's event stream (newest last, capped).
  final List<String> notices = [];
  StreamSubscription<CoreEvent>? _events;
  Duration _retry = const Duration(seconds: 2);

  /// Subscribes to core events and reconnects with capped backoff.
  void listenForEvents() {
    _events?.cancel();
    _events = core.events().listen(
      (e) {
        _retry = const Duration(seconds: 2);
        if (e.kind == 'schedule.started') return; // only outcomes are worth a notice
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
    _events?.cancel();
    _events = null;
  }

  @override
  void dispose() {
    stopEvents();
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
  }

  Future<void> createNotebook(String name, WorkTypeId type) async {
    final nb = await core.createNotebook(name: name, workType: type);
    notebooks = await core.listNotebooks();
    select(nb.id);
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
    final updated = await core.setContextEnabled(notebookId: nb.id, contextId: contextId, enabled: enabled);
    notebooks = [for (final n in notebooks) n.id == updated.id ? updated : n];
    notifyListeners();
  }

  Future<void> run(String prompt, Map<String, String> vars) async {
    final nb = selected;
    if (nb == null || running) return;
    running = true;
    outcome = null;
    _stream.clear();
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
        }
        notifyListeners();
      }
    } catch (err) {
      status = 'Run failed: $err';
    } finally {
      running = false;
      notifyListeners();
    }
  }
}

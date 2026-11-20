import 'package:material_ui/material_ui.dart';

import 'core/core_client.dart';
import 'core/models.dart';

enum Section { notebooks, library, settings }

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

part of 'app_state.dart';

/// The open notebook's saved prompts.
extension PromptActions on AppState {
  /// Reloads the open notebook's saved prompts; an older core without them shows none.
  Future<void> loadPrompts() async {
    final nb = selected;
    if (nb == null) return;
    try {
      final list = await core.listPrompts(nb.id);
      if (selected?.id == nb.id) prompts = list;
    } on Object {
      prompts = const [];
    }
    _changed();
  }

  /// Saves [text] as prompt [name]; a refusal becomes a notice.
  Future<bool> savePromptAs(String name, String text) => _promptAction(() async {
    final nb = selected;
    if (nb != null) await core.savePrompt(nb.id, SavedPrompt(name: name, text: text));
  });

  Future<bool> deletePrompt(String name) => _promptAction(() async {
    final nb = selected;
    if (nb != null) await core.deletePrompt(nb.id, name);
  });

  Future<bool> _promptAction(Future<void> Function() action) async {
    try {
      await action();
    } on CoreException catch (e) {
      _notice(e.message);
      return false;
    }
    await loadPrompts();
    return true;
  }
}

/// First-run checks: a provider, a default model and its API key.
extension SetupCheck on AppState {
  /// Recomputes [setupMessage]; an unreachable core leaves it as it was.
  Future<void> checkSetup() async {
    final String? message;
    try {
      message = await _setupProblem();
    } on Object {
      return;
    }
    setupMessage = message;
    _changed();
  }

  Future<String?> _setupProblem() async {
    final s = await core.getSettings();
    if (s.providers.isEmpty) return 'Add a model provider (an OpenAI-compatible URL such as a LiteLLM proxy).';
    if ((s.defaultModel ?? '').isEmpty) return 'Choose a default model so notebooks and chat can run.';
    for (final p in s.providers) {
      final ref = p.keyRef;
      if (ref != null && !await core.secretSet(ref)) return 'Set the API key for ${p.name} ($ref).';
    }
    return null;
  }
}

/// Opening a search match.
extension SearchActions on AppState {
  /// Shows where a match lives: its notebook (and output), or its research notebook.
  Future<void> openHit(SearchHit h) async {
    switch (h.kind) {
      case 'research' || 'note':
        selectResearch(h.notebookId);
      case 'output':
        select(h.notebookId);
        await loadOutputs(open: h.id);
      default:
        select(h.notebookId);
    }
  }
}

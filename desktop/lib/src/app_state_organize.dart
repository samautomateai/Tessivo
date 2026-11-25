part of 'app_state.dart';

/// Tags and pins (NB-5).
extension OrganizeActions on AppState {
  /// Replaces a notebook's tags; a refusal (bad tag, too many) becomes a notice.
  Future<void> setTags(String id, List<String> tags) => _manage(id, () => core.updateNotebook(id, tags: tags));

  /// Pins or unpins a notebook; pinned notebooks are listed first.
  Future<void> setPinned(String id, bool pinned) => _manage(id, () => core.updateNotebook(id, pinned: pinned));

  /// Sets what the core does when the notebook's folders change: off, notify or run (SRC-8).
  Future<void> setWatch(String id, String mode) => _manage(id, () => core.updateNotebook(id, watch: mode));

  /// Filters the side bar by tag text (any case, part of a tag).
  void setTagFilter(String text) {
    tagFilter = text;
    _changed();
  }

  /// Whether [nb] passes the tag filter.
  bool matchesTagFilter(Notebook nb) {
    final q = tagFilter.trim().toLowerCase();
    return q.isEmpty || nb.tags.any((t) => t.toLowerCase().contains(q));
  }
}

/// Notebook archives (NB-4).
extension ArchiveActions on AppState {
  /// Saves a notebook as an archive at [path]; returns an error message or null.
  Future<String?> exportNotebook(String id, String path, {bool outputs = false}) async {
    try {
      await core.exportNotebook(id, path, outputs: outputs);
      return null;
    } on CoreException catch (e) {
      return e.message;
    } on FileSystemException catch (e) {
      return 'Could not save: ${e.message}';
    }
  }

  /// Adds a notebook from an archive and opens it; a refusal becomes a notice.
  Future<void> importNotebook(String path) async {
    try {
      final nb = await core.importNotebook(path);
      notebooks = await core.listNotebooks();
      select(nb.id);
    } on CoreException catch (e) {
      _notice(e.message);
    } on FileSystemException catch (e) {
      _notice('Could not read ${e.path ?? path}: ${e.message}');
    }
  }

  /// Points a source that needs relinking at [path] on this computer.
  Future<void> relinkContext(String id, String contextId, String path) async {
    try {
      _put(await core.relinkContext(id, contextId, path));
    } on CoreException catch (e) {
      _notice(e.message);
    }
  }

  /// Adds a research notebook from an archive and opens it.
  Future<void> importResearch(String path) async {
    try {
      final r = await core.importResearch(path);
      await loadResearch();
      selectResearch(r.id);
    } on CoreException catch (e) {
      _notice(e.message);
    } on FileSystemException catch (e) {
      _notice('Could not read ${e.path ?? path}: ${e.message}');
    }
  }
}

/// Settings profiles (SET-10) and the work type a notebook's sources suggest (WT-7).
extension ProfileActions on AppState {
  /// Re-reads the active profile and default model after a switch, and re-checks the setup.
  Future<void> refreshProfile() async {
    await loadAppearance();
    _changed();
    await checkSetup();
  }

  /// Changes a notebook's work type (only while it has no outputs); a refusal becomes a notice.
  Future<void> setWorkType(String id, WorkTypeId workType) =>
      _manage(id, () => core.updateNotebook(id, workType: workType));
}

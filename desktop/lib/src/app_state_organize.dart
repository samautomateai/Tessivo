part of 'app_state.dart';

/// Tags and pins (NB-5).
extension OrganizeActions on AppState {
  /// Replaces a notebook's tags; a refusal (bad tag, too many) becomes a notice.
  Future<void> setTags(String id, List<String> tags) => _manage(id, () => core.updateNotebook(id, tags: tags));

  /// Pins or unpins a notebook; pinned notebooks are listed first.
  Future<void> setPinned(String id, bool pinned) => _manage(id, () => core.updateNotebook(id, pinned: pinned));

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

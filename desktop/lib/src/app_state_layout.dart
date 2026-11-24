part of 'app_state.dart';

/// The workbench's panels and the status bar's numbers.
extension LayoutActions on AppState {
  /// Whether the side bar shows in a window [width] wide.
  bool sideBarShown(double width) => width >= sideBarMinWindow ? layout.sideBarOpen : sideBarPeek;

  /// Whether the secondary panel shows in a window [width] wide.
  bool panelShown(double width) => width >= panelMinWindow ? layout.panelOpen : panelPeek;

  /// Ctrl B: show or hide the side bar (remembered). In a window [width] too
  /// narrow for it, it opens for the session only.
  Future<void> toggleSideBar({double width = double.infinity}) async {
    if (width < sideBarMinWindow) {
      sideBarPeek = !sideBarPeek;
      return _changed();
    }
    await _setLayout(layout.copyWith(sideBarOpen: !layout.sideBarOpen));
  }

  /// Ctrl J: show or hide the secondary panel, like [toggleSideBar].
  Future<void> togglePanel({double width = double.infinity}) async {
    if (width < panelMinWindow) {
      panelPeek = !panelPeek;
      return _changed();
    }
    await _setLayout(layout.copyWith(panelOpen: !layout.panelOpen));
  }

  /// Resizes the side bar while dragging; [save] once the drag ends.
  Future<void> setSideBarWidth(double width, {bool save = false}) =>
      _setLayout(layout.copyWith(sideBarWidth: width), save: save);

  Future<void> setPanelWidth(double width, {bool save = false}) =>
      _setLayout(layout.copyWith(panelWidth: width), save: save);

  Future<void> _setLayout(LayoutPrefs next, {bool save = true}) async {
    layout = next;
    _changed();
    if (save) await _saveAppearance((d) => d.layout = next.toJson());
  }

  /// Today's tokens for the status bar; an older core without usage shows none.
  Future<void> refreshUsage() async {
    try {
      usage = await core.usage();
    } on Object {
      usage = null;
    }
    _changed();
  }
}

/// Opening Settings at one of its sections.
extension SettingsNavigation on AppState {
  void openSettings([SettingsSection? at]) {
    if (at != null) settingsSection = at;
    go(Section.settings);
  }
}

/// The status bar's next scheduled run.
extension NextRun on AppState {
  /// Finds the earliest next run among the enabled schedules of notebooks
  /// that have one; a time the core gave that does not parse ranks last.
  Future<void> refreshNextRun() async {
    ({String notebook, String at, DateTime? when})? best;
    for (final nb in notebooks.where((n) => n.schedule != null && !n.archived)) {
      final List<ScheduleInfo> list;
      try {
        list = await core.listSchedules(nb.id);
      } on Object {
        continue;
      }
      for (final s in list.where((s) => s.enabled && s.nextRun != null)) {
        final when = DateTime.tryParse(s.nextRun!);
        final better = best == null || (when != null && (best.when == null || when.isBefore(best.when!)));
        if (better) best = (notebook: nb.name, at: s.nextRun!, when: when);
      }
    }
    nextRun = best == null ? null : (notebook: best.notebook, at: best.at);
    _changed();
  }
}

/// "today 08:00", "tomorrow 08:00", "Mon 08:00"; text the core sent that is
/// not a time stays as it is.
String nextRunLabel(String at, DateTime now) {
  final t = DateTime.tryParse(at)?.toLocal();
  if (t == null) return at;
  final hm = '${t.hour.toString().padLeft(2, '0')}:${t.minute.toString().padLeft(2, '0')}';
  final day = DateTime(t.year, t.month, t.day).difference(DateTime(now.year, now.month, now.day)).inDays;
  if (day == 0) return 'today $hm';
  if (day == 1) return 'tomorrow $hm';
  const names = ['Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat', 'Sun'];
  return day < 7
      ? '${names[t.weekday - 1]} $hm'
      : '${t.year}-${t.month.toString().padLeft(2, '0')}-${t.day.toString().padLeft(2, '0')} $hm';
}

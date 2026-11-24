part of 'app_state.dart';

/// The workbench's panels and the status bar's numbers.
extension LayoutActions on AppState {
  /// Ctrl B: show or hide the side bar (remembered).
  Future<void> toggleSideBar() => _setLayout(layout.copyWith(sideBarOpen: !layout.sideBarOpen));

  /// Ctrl J: show or hide the secondary panel (remembered).
  Future<void> togglePanel() => _setLayout(layout.copyWith(panelOpen: !layout.panelOpen));

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

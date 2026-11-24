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

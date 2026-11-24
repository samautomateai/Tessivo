/// Fixed sizes in logical pixels (spec 6.3); zoom multiplies them (TvZoom).
abstract final class TvSize {
  static const double header = 40;
  static const double toolbar = 38;
  static const double tabBar = 36;
  static const double treeRow = 26;
  static const double tableRow = 36;
  static const double tableHead = 32;
  static const double button = 28;
  static const double buttonDense = 24;
  static const double input = 32;
  static const double statusBar = 22;
  static const double activityBar = 64;
  static const double sideBar = 236;
  static const double panel = 290;
  static const double readingWidth = 760;
  static const double icon = 16;
  static const double iconLarge = 20;
}

/// The Carbon spacing scale.
abstract final class TvSpace {
  static const double s2 = 2;
  static const double s4 = 4;
  static const double s8 = 8;
  static const double s12 = 12;
  static const double s16 = 16;
  static const double s24 = 24;
  static const double s32 = 32;
  static const double s40 = 40;
  static const double s48 = 48;
  static const all = [s2, s4, s8, s12, s16, s24, s32, s40, s48];
}

/// Corner radii after VS Code: square regions, barely rounded controls.
abstract final class TvRadius {
  /// Panels, rows, tabs, bars.
  static const double none = 0;

  /// Buttons, inputs, tags, tool-call blocks, cards.
  static const double small = 2;

  /// Menus, the palette, dialogs, tooltips.
  static const double medium = 4;
}

/// Motion (Carbon productive); none when the OS asks for reduced motion.
abstract final class TvMotion {
  static const fast = Duration(milliseconds: 70);
  static const moderate = Duration(milliseconds: 110);
}

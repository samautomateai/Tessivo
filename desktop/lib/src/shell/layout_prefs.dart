import '../design/sizes.dart';

/// Below these widths the secondary panel, then the side bar, close; their
/// shortcuts still open them for the session (AppState.panelPeek).
const panelMinWindow = 1100.0;
const sideBarMinWindow = 860.0;

/// Which workbench panels are open and how wide (`desktop.layout`).
class LayoutPrefs {
  const LayoutPrefs({
    this.sideBarOpen = true,
    this.sideBarWidth = TvSize.sideBar,
    this.panelOpen = true,
    this.panelWidth = TvSize.panel,
  });

  /// From the config's `desktop.layout`; missing or out-of-range values give defaults.
  factory LayoutPrefs.fromJson(Map<String, Object?> j) {
    double width(Object? v, double fallback) {
      final w = (v as num?)?.toDouble();
      return w != null && w >= minWidth && w <= maxWidth ? w : fallback;
    }

    return LayoutPrefs(
      sideBarOpen: j['side_bar_open'] != false,
      sideBarWidth: width(j['side_bar_width'], TvSize.sideBar),
      panelOpen: j['panel_open'] != false,
      panelWidth: width(j['panel_width'], TvSize.panel),
    );
  }

  static const double minWidth = 180;
  static const double maxWidth = 480;

  final bool sideBarOpen;
  final double sideBarWidth;
  final bool panelOpen;
  final double panelWidth;

  LayoutPrefs copyWith({bool? sideBarOpen, double? sideBarWidth, bool? panelOpen, double? panelWidth}) => LayoutPrefs(
    sideBarOpen: sideBarOpen ?? this.sideBarOpen,
    sideBarWidth: (sideBarWidth ?? this.sideBarWidth).clamp(minWidth, maxWidth),
    panelOpen: panelOpen ?? this.panelOpen,
    panelWidth: (panelWidth ?? this.panelWidth).clamp(minWidth, maxWidth),
  );

  /// Only what differs from the defaults is saved.
  Map<String, Object?> toJson() => {
    if (!sideBarOpen) 'side_bar_open': false,
    if (sideBarWidth != TvSize.sideBar) 'side_bar_width': sideBarWidth.round(),
    if (!panelOpen) 'panel_open': false,
    if (panelWidth != TvSize.panel) 'panel_width': panelWidth.round(),
  };
}

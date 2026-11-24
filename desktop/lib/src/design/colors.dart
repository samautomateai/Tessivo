import 'package:material_ui/material_ui.dart';

import '../core/models.dart';

/// A work-type tag: background, text and the small dot used in lists.
@immutable
class TagColors {
  const TagColors(this.background, this.text, this.dot);
  final Color background;
  final Color text;
  final Color dot;
}

/// Carbon colour tokens (docs/superpowers/specs/2026-11-24-enterprise-design-system-design.md, 6.1).
/// Widgets read them with [TvColors.of]; nothing else in the app names a colour.
@immutable
class TvColors extends ThemeExtension<TvColors> {
  const TvColors({
    required this.brightness,
    required this.background,
    required this.layer01,
    required this.layer02,
    required this.layerSelected,
    required this.borderSubtle,
    required this.borderStrong,
    required this.fieldUnderline,
    required this.textPrimary,
    required this.textSecondary,
    required this.textHelper,
    required this.interactive,
    required this.buttonPrimary,
    required this.error,
    required this.warning,
    required this.success,
    required this.info,
    required this.activityBar,
    required this.statusBar,
    required this.statusBarText,
    required this.tags,
  });

  final Brightness brightness;
  final Color background;
  final Color layer01;
  final Color layer02;
  final Color layerSelected;
  final Color borderSubtle;
  final Color borderStrong;
  final Color fieldUnderline;
  final Color textPrimary;
  final Color textSecondary;
  final Color textHelper;
  final Color interactive;
  final Color buttonPrimary;
  final Color error;
  final Color warning;
  final Color success;
  final Color info;
  final Color activityBar;
  final Color statusBar;
  final Color statusBarText;
  final Map<WorkTypeId, TagColors> tags;

  TagColors tag(WorkTypeId id) => tags[id]!;

  static const _darkTags = {
    WorkTypeId.analyze: TagColors(Color(0xFF510224), Color(0xFFFFAFD2), Color(0xFFFF7EB6)),
    WorkTypeId.summarize: TagColors(Color(0xFF004144), Color(0xFF9EF0F0), Color(0xFF3DDBD9)),
    WorkTypeId.document: TagColors(Color(0xFF002D9C), Color(0xFFD0E2FF), Color(0xFF78A9FF)),
    WorkTypeId.report: TagColors(Color(0xFF491D8B), Color(0xFFE8DAFF), Color(0xFFBE95FF)),
  };

  /// Carbon Gray 100.
  static const dark = TvColors(
    brightness: Brightness.dark,
    background: Color(0xFF161616),
    layer01: Color(0xFF262626),
    layer02: Color(0xFF393939),
    layerSelected: Color(0xFF353535),
    borderSubtle: Color(0xFF2E2E2E),
    borderStrong: Color(0xFF393939),
    fieldUnderline: Color(0xFF525252),
    textPrimary: Color(0xFFF4F4F4),
    textSecondary: Color(0xFFC6C6C6),
    textHelper: Color(0xFF8D8D8D),
    interactive: Color(0xFF4589FF),
    buttonPrimary: Color(0xFF0F62FE),
    error: Color(0xFFFA4D56),
    warning: Color(0xFFF1C21B),
    success: Color(0xFF42BE65),
    info: Color(0xFF4589FF),
    activityBar: Color(0xFF0D0D0D),
    statusBar: Color(0xFF0F62FE),
    statusBarText: Color(0xFFFFFFFF),
    tags: _darkTags,
  );

  /// Carbon White.
  static const light = TvColors(
    brightness: Brightness.light,
    background: Color(0xFFFFFFFF),
    layer01: Color(0xFFF4F4F4),
    layer02: Color(0xFFE0E0E0),
    layerSelected: Color(0xFFE0E0E0),
    borderSubtle: Color(0xFFE0E0E0),
    borderStrong: Color(0xFFC6C6C6),
    fieldUnderline: Color(0xFFC6C6C6),
    textPrimary: Color(0xFF161616),
    textSecondary: Color(0xFF525252),
    textHelper: Color(0xFF6F6F6F),
    interactive: Color(0xFF0F62FE),
    buttonPrimary: Color(0xFF0F62FE),
    error: Color(0xFFDA1E28),
    warning: Color(0xFFF1C21B),
    success: Color(0xFF24A148),
    info: Color(0xFF0043CE),
    activityBar: Color(0xFFF4F4F4),
    statusBar: Color(0xFF0F62FE),
    statusBarText: Color(0xFFFFFFFF),
    tags: {
      WorkTypeId.analyze: TagColors(Color(0xFFFFD6E8), Color(0xFF9F1853), Color(0xFFD02670)),
      WorkTypeId.summarize: TagColors(Color(0xFF9EF0F0), Color(0xFF005D5D), Color(0xFF007D79)),
      WorkTypeId.document: TagColors(Color(0xFFD0E2FF), Color(0xFF0043CE), Color(0xFF0F62FE)),
      WorkTypeId.report: TagColors(Color(0xFFE8DAFF), Color(0xFF6929C4), Color(0xFF8A3FFC)),
    },
  );

  /// High contrast (dark), after VS Code's default high-contrast theme.
  static const highContrast = TvColors(
    brightness: Brightness.dark,
    background: Color(0xFF000000),
    layer01: Color(0xFF000000),
    layer02: Color(0xFF1A1A1A),
    layerSelected: Color(0xFF0F4A85),
    borderSubtle: Color(0xFF6FC3DF),
    borderStrong: Color(0xFF6FC3DF),
    fieldUnderline: Color(0xFFFFFFFF),
    textPrimary: Color(0xFFFFFFFF),
    textSecondary: Color(0xFFFFFFFF),
    textHelper: Color(0xFFD0D0D0),
    interactive: Color(0xFFF38518),
    buttonPrimary: Color(0xFF0F62FE),
    error: Color(0xFFFF8389),
    warning: Color(0xFFFDDC69),
    success: Color(0xFF6FDC8C),
    info: Color(0xFFA6C8FF),
    activityBar: Color(0xFF000000),
    statusBar: Color(0xFF000000),
    statusBarText: Color(0xFFFFFFFF),
    tags: _darkTags,
  );

  /// The tokens of the nearest theme (dark when a test pumps no theme).
  static TvColors of(BuildContext context) => Theme.of(context).extension<TvColors>() ?? dark;

  @override
  TvColors copyWith() => this;

  @override
  TvColors lerp(TvColors? other, double t) => other == null || t < 0.5 ? this : other;
}

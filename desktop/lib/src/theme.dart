import 'package:material_ui/material_ui.dart';

import 'core/models.dart';

/// Design tokens from the Tessivo GUI design: graphite sidebar, light content,
/// indigo accent, one colour per work type.
abstract final class Tokens {
  static const accent = Color(0xFF3B5BDB);
  static const sidebar = Color(0xFF1E2129);
  static const sidebarSelected = Color(0xFF2C303B);
  static const sidebarText = Color(0xFFC9CDD6);
  static const canvas = Color(0xFFF4F5F7);
  static const border = Color(0xFFE3E5EA);
  static const textMuted = Color(0xFF5E6573);
}

class WorkTypeColors {
  const WorkTypeColors(this.dot, this.background, this.foreground);
  final Color dot;
  final Color background;
  final Color foreground;
}

WorkTypeColors colorsFor(WorkTypeId id) => switch (id) {
  WorkTypeId.analyze => const WorkTypeColors(Color(0xFFF08C3A), Color(0xFFFDEBD9), Color(0xFF8A4510)),
  WorkTypeId.summarize => const WorkTypeColors(Color(0xFF2FB59A), Color(0xFFD8F3EC), Color(0xFF0E5E4E)),
  WorkTypeId.document => const WorkTypeColors(Color(0xFF4C8DFF), Color(0xFFDEEAFF), Color(0xFF1E4FA8)),
  WorkTypeId.report => const WorkTypeColors(Color(0xFFA58BFA), Color(0xFFEDE6FF), Color(0xFF5B3DB5)),
};

String workTypeLabel(WorkTypeId id) => switch (id) {
  WorkTypeId.analyze => 'Analyze',
  WorkTypeId.summarize => 'Summarize',
  WorkTypeId.document => 'Document',
  WorkTypeId.report => 'Report',
};

ThemeData buildTheme(Brightness brightness) {
  final scheme = ColorScheme.fromSeed(seedColor: Tokens.accent, brightness: brightness);
  final light = brightness == Brightness.light;
  return ThemeData(
    colorScheme: scheme,
    useMaterial3: true,
    scaffoldBackgroundColor: light ? Tokens.canvas : const Color(0xFF15171C),
    dividerColor: light ? Tokens.border : const Color(0xFF2C303B),
    cardTheme: CardThemeData(
      elevation: 0,
      margin: EdgeInsets.zero,
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.circular(12),
        side: BorderSide(color: light ? Tokens.border : const Color(0xFF2C303B)),
      ),
    ),
    visualDensity: VisualDensity.compact,
  );
}

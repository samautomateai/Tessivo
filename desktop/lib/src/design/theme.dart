import 'package:material_ui/material_ui.dart';

import 'colors.dart';
import 'sizes.dart';
import 'type.dart';

/// The theme choice in Settings → Appearance (`desktop.theme`).
enum TvThemeMode { system, light, dark, highContrast }

/// The tokens for [mode]: System follows the OS light/dark setting and its
/// high-contrast switch; an explicit choice ignores the OS.
TvColors resolveColors(TvThemeMode mode, {required Brightness platform, required bool highContrast}) => switch (mode) {
  TvThemeMode.light => TvColors.light,
  TvThemeMode.dark => TvColors.dark,
  TvThemeMode.highContrast => TvColors.highContrast,
  TvThemeMode.system when highContrast => TvColors.highContrast,
  TvThemeMode.system => platform == Brightness.dark ? TvColors.dark : TvColors.light,
};

/// Material theme built from the tokens, so stock widgets look Carbon too.
ThemeData tvTheme(TvColors c) {
  RoundedRectangleBorder corners(double r) => RoundedRectangleBorder(borderRadius: BorderRadius.circular(r));
  final scheme = ColorScheme(
    brightness: c.brightness,
    primary: c.buttonPrimary,
    onPrimary: const Color(0xFFFFFFFF),
    primaryContainer: c.layerSelected,
    onPrimaryContainer: c.textPrimary,
    secondary: c.interactive,
    onSecondary: c.background,
    secondaryContainer: c.layer02,
    onSecondaryContainer: c.textPrimary,
    tertiary: c.interactive,
    onTertiary: c.background,
    tertiaryContainer: c.layer02,
    onTertiaryContainer: c.textPrimary,
    errorContainer: c.layer02,
    onErrorContainer: c.error,
    error: c.error,
    onError: c.background,
    surface: c.background,
    onSurface: c.textPrimary,
    onSurfaceVariant: c.textSecondary,
    surfaceContainerLowest: c.background,
    surfaceContainerLow: c.layer01,
    surfaceContainer: c.layer01,
    surfaceContainerHigh: c.layer02,
    surfaceContainerHighest: c.layer02,
    outline: c.borderStrong,
    outlineVariant: c.borderSubtle,
  );
  TextStyle t(TextStyle s, [Color? color]) => s.copyWith(color: color ?? c.textPrimary);
  final buttonText = WidgetStatePropertyAll(TvType.body.copyWith(fontWeight: FontWeight.w500));
  final buttonShape = WidgetStatePropertyAll(corners(TvRadius.small));
  const buttonSize = WidgetStatePropertyAll(Size(0, TvSize.button));
  const buttonPadding = WidgetStatePropertyAll(EdgeInsets.symmetric(horizontal: TvSpace.s12));
  return ThemeData(
    useMaterial3: true,
    colorScheme: scheme,
    fontFamily: TvType.sans,
    scaffoldBackgroundColor: c.background,
    canvasColor: c.background,
    dividerColor: c.borderSubtle,
    visualDensity: VisualDensity.compact,
    extensions: [c],
    textTheme: TextTheme(
      titleLarge: t(TvType.pageTitle),
      titleMedium: t(TvType.heading02),
      titleSmall: t(TvType.heading01),
      bodyLarge: t(TvType.bodyLong),
      bodyMedium: t(TvType.body),
      bodySmall: t(TvType.helper, c.textSecondary),
      labelLarge: t(TvType.body.copyWith(fontWeight: FontWeight.w500)),
      labelMedium: t(TvType.helper),
      labelSmall: t(TvType.caption, c.textSecondary),
    ),
    iconTheme: IconThemeData(color: c.textSecondary, size: TvSize.icon),
    dividerTheme: DividerThemeData(color: c.borderSubtle, thickness: 1, space: 1),
    filledButtonTheme: FilledButtonThemeData(
      style: ButtonStyle(
        shape: buttonShape,
        minimumSize: buttonSize,
        padding: buttonPadding,
        textStyle: buttonText,
        backgroundColor: WidgetStatePropertyAll(c.buttonPrimary),
        foregroundColor: const WidgetStatePropertyAll(Color(0xFFFFFFFF)),
      ),
    ),
    textButtonTheme: TextButtonThemeData(
      style: ButtonStyle(
        shape: buttonShape,
        minimumSize: buttonSize,
        padding: buttonPadding,
        textStyle: buttonText,
        foregroundColor: WidgetStatePropertyAll(c.textPrimary),
      ),
    ),
    outlinedButtonTheme: OutlinedButtonThemeData(
      style: ButtonStyle(
        shape: buttonShape,
        minimumSize: buttonSize,
        padding: buttonPadding,
        textStyle: buttonText,
        foregroundColor: WidgetStatePropertyAll(c.textPrimary),
        side: WidgetStatePropertyAll(BorderSide(color: c.borderStrong)),
      ),
    ),
    inputDecorationTheme: InputDecorationTheme(
      filled: true,
      fillColor: c.layer01,
      isDense: true,
      labelStyle: t(TvType.helper, c.textSecondary),
      hintStyle: t(TvType.body, c.textHelper),
      border: UnderlineInputBorder(
        borderRadius: BorderRadius.circular(TvRadius.small),
        borderSide: BorderSide(color: c.fieldUnderline),
      ),
      enabledBorder: UnderlineInputBorder(
        borderRadius: BorderRadius.circular(TvRadius.small),
        borderSide: BorderSide(color: c.fieldUnderline),
      ),
      focusedBorder: UnderlineInputBorder(
        borderRadius: BorderRadius.circular(TvRadius.small),
        borderSide: BorderSide(color: c.interactive, width: 2),
      ),
    ),
    cardTheme: CardThemeData(
      elevation: 0,
      margin: EdgeInsets.zero,
      color: c.layer01,
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.circular(TvRadius.small),
        side: BorderSide(color: c.borderSubtle),
      ),
    ),
    dialogTheme: DialogThemeData(backgroundColor: c.layer01, shape: corners(TvRadius.medium)),
    popupMenuTheme: PopupMenuThemeData(color: c.layer01, shape: corners(TvRadius.medium)),
    tooltipTheme: TooltipThemeData(
      decoration: BoxDecoration(color: c.layer02, borderRadius: BorderRadius.circular(TvRadius.medium)),
      textStyle: t(TvType.helper),
    ),
    chipTheme: ChipThemeData(shape: corners(TvRadius.small), backgroundColor: c.layer02),
    snackBarTheme: SnackBarThemeData(
      backgroundColor: c.layer02,
      contentTextStyle: t(TvType.body),
      shape: corners(TvRadius.medium),
    ),
  );
}

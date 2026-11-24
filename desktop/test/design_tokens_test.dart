import 'dart:math';

import 'package:flutter_test/flutter_test.dart';
import 'package:material_ui/material_ui.dart';
import 'package:tessivo_desktop/src/core/models.dart';
import 'package:tessivo_desktop/src/design/colors.dart';
import 'package:tessivo_desktop/src/design/sizes.dart';
import 'package:tessivo_desktop/src/design/theme.dart';
import 'package:tessivo_desktop/src/design/type.dart';

/// WCAG 2.1 contrast ratio of two opaque colours.
double contrast(Color a, Color b) {
  double lum(Color c) {
    double ch(double v) => v <= 0.03928 ? v / 12.92 : pow((v + 0.055) / 1.055, 2.4).toDouble();
    return 0.2126 * ch(c.r) + 0.7152 * ch(c.g) + 0.0722 * ch(c.b);
  }

  final (x, y) = (lum(a), lum(b));
  return (max(x, y) + 0.05) / (min(x, y) + 0.05);
}

void main() {
  test('token values match the spec', () {
    expect(TvColors.dark.background, const Color(0xFF161616));
    expect(TvColors.dark.layer01, const Color(0xFF262626));
    expect(TvColors.dark.textPrimary, const Color(0xFFF4F4F4));
    expect(TvColors.dark.interactive, const Color(0xFF4589FF));
    expect(TvColors.light.background, const Color(0xFFFFFFFF));
    expect(TvColors.light.interactive, const Color(0xFF0F62FE));
    expect(TvColors.highContrast.background, const Color(0xFF000000));
    expect(TvColors.highContrast.borderSubtle, const Color(0xFF6FC3DF));
    expect(TvColors.highContrast.interactive, const Color(0xFFF38518));
    expect(TvColors.dark.buttonPrimary, const Color(0xFF0F62FE));
    expect(TvColors.dark.tag(WorkTypeId.analyze).background, const Color(0xFF510224));
    expect(TvColors.dark.tag(WorkTypeId.summarize).text, const Color(0xFF9EF0F0));
    expect(TvColors.light.tag(WorkTypeId.report).dot, const Color(0xFF8A3FFC));
  });

  test('text pairs meet contrast', () {
    for (final (name, c, minimum) in [
      ('dark', TvColors.dark, 4.5),
      ('light', TvColors.light, 4.5),
      ('high contrast', TvColors.highContrast, 7.0),
    ]) {
      expect(contrast(c.textPrimary, c.background), greaterThanOrEqualTo(minimum), reason: '$name primary');
      expect(contrast(c.textSecondary, c.layer01), greaterThanOrEqualTo(minimum), reason: '$name secondary');
      expect(contrast(c.textHelper, c.background), greaterThanOrEqualTo(minimum), reason: '$name helper');
      expect(contrast(c.statusBarText, c.statusBar), greaterThanOrEqualTo(4.5), reason: '$name status bar');
      for (final w in WorkTypeId.values) {
        final t = c.tag(w);
        expect(contrast(t.text, t.background), greaterThanOrEqualTo(4.5), reason: '$name ${w.name} tag');
      }
    }
  });

  test('type scale matches the spec', () {
    expect((TvType.body.fontSize, TvType.body.height! * 13), (13, 18));
    expect((TvType.pageTitle.fontSize, TvType.pageTitle.fontWeight), (20, FontWeight.w400));
    expect((TvType.heading01.fontSize, TvType.heading01.fontWeight), (14, FontWeight.w600));
    expect((TvType.label.fontSize, TvType.label.fontWeight, TvType.label.letterSpacing), (11, FontWeight.w600, 0.32));
    expect((TvType.helper.fontSize, TvType.caption.fontSize), (12, 10));
    expect(TvType.body.fontFamily, 'IBMPlexSans');
    expect((TvType.code.fontFamily, TvType.code.fontSize), ('IBMPlexMono', 12));
  });

  test('sizes and corners match the spec', () {
    expect([TvSize.header, TvSize.toolbar, TvSize.tabBar, TvSize.treeRow], [40, 38, 36, 26]);
    expect([TvSize.button, TvSize.buttonDense, TvSize.input, TvSize.statusBar], [28, 24, 32, 22]);
    expect([TvSize.activityBar, TvSize.sideBar, TvSize.panel, TvSize.readingWidth], [64, 236, 290, 760]);
    expect([TvRadius.none, TvRadius.small, TvRadius.medium], [0, 2, 4]);
    expect(TvSpace.all, [2, 4, 8, 12, 16, 24, 32, 40, 48]);
  });

  test('resolveColors follows the OS unless a theme is chosen', () {
    TvColors r(TvThemeMode m, Brightness b, {bool hc = false}) => resolveColors(m, platform: b, highContrast: hc);
    expect(r(TvThemeMode.system, Brightness.dark), TvColors.dark);
    expect(r(TvThemeMode.system, Brightness.light), TvColors.light);
    expect(r(TvThemeMode.system, Brightness.light, hc: true), TvColors.highContrast);
    expect(r(TvThemeMode.light, Brightness.dark, hc: true), TvColors.light);
    expect(r(TvThemeMode.highContrast, Brightness.light), TvColors.highContrast);
  });

  test('tvTheme carries the tokens, Plex and VS Code corners', () {
    final t = tvTheme(TvColors.dark);
    expect(t.extension<TvColors>(), TvColors.dark);
    expect(t.scaffoldBackgroundColor, const Color(0xFF161616));
    expect(t.textTheme.bodyMedium!.fontFamily, 'IBMPlexSans');
    final shape = t.filledButtonTheme.style!.shape!.resolve({})! as RoundedRectangleBorder;
    expect(shape.borderRadius, BorderRadius.circular(2));
  });
}

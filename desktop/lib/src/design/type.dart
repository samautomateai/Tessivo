import 'package:material_ui/material_ui.dart';

/// IBM Plex type at VS Code density (spec 6.2). Sizes are logical pixels; the
/// app's text scaler (system text size × zoom) applies on top, so never set
/// textScaleFactor anywhere.
abstract final class TvType {
  static const sans = 'IBMPlexSans';
  static const mono = 'IBMPlexMono';

  static const pageTitle = TextStyle(fontFamily: sans, fontSize: 20, height: 26 / 20, fontWeight: FontWeight.w400);
  static const heading02 = TextStyle(fontFamily: sans, fontSize: 16, height: 22 / 16, fontWeight: FontWeight.w600);
  static const heading01 = TextStyle(
    fontFamily: sans,
    fontSize: 14,
    height: 18 / 14,
    fontWeight: FontWeight.w600,
    letterSpacing: 0.16,
  );
  static const body = TextStyle(
    fontFamily: sans,
    fontSize: 13,
    height: 18 / 13,
    fontWeight: FontWeight.w400,
    letterSpacing: 0.16,
  );
  static const bodyLong = TextStyle(
    fontFamily: sans,
    fontSize: 13,
    height: 20 / 13,
    fontWeight: FontWeight.w400,
    letterSpacing: 0.16,
  );

  /// Side-bar section labels; widgets upper-case the text.
  static const label = TextStyle(
    fontFamily: sans,
    fontSize: 11,
    height: 16 / 11,
    fontWeight: FontWeight.w600,
    letterSpacing: 0.32,
  );
  static const helper = TextStyle(
    fontFamily: sans,
    fontSize: 12,
    height: 16 / 12,
    fontWeight: FontWeight.w400,
    letterSpacing: 0.32,
  );
  static const caption = TextStyle(
    fontFamily: sans,
    fontSize: 10,
    height: 12 / 10,
    fontWeight: FontWeight.w500,
    letterSpacing: 0.2,
  );
  static const code = TextStyle(fontFamily: mono, fontSize: 12, height: 16 / 12, fontWeight: FontWeight.w400);
}

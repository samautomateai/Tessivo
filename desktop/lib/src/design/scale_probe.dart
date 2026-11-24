import 'package:material_ui/material_ui.dart';

/// The display scale (device pixels per logical pixel) and the system text
/// scale the app sees, e.g. "display 1.50 × text 1.25". Shown in Settings →
/// Appearance so a user can check that Windows' scaling reaches the app.
String describeScale(MediaQueryData mq) =>
    'display ${mq.devicePixelRatio.toStringAsFixed(2)} × text ${mq.textScaler.scale(1).toStringAsFixed(2)}';

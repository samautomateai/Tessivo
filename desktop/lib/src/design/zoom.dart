import 'package:material_ui/material_ui.dart';

/// Zoom limits and step (VS Code-style Ctrl + / Ctrl − / Ctrl 0).
const minZoom = 0.8;
const maxZoom = 2.0;
const zoomStep = 0.1;

/// The next zoom after a step: [direction] +1 bigger, −1 smaller, 0 reset to
/// 1.0; clamped to 0.8–2.0 and rounded to one decimal so steps never drift.
double nextZoom(double zoom, int direction) {
  if (direction == 0) return 1;
  final next = ((zoom + direction * zoomStep) * 10).round() / 10;
  return next.clamp(minZoom, maxZoom);
}

/// The app's zoom factor, for widgets that show or change it.
class TvZoom extends InheritedWidget {
  const TvZoom({super.key, required this.factor, required super.child});

  final double factor;

  double scale(double v) => v * factor;

  static TvZoom of(BuildContext context) =>
      context.dependOnInheritedWidgetOfExactType<TvZoom>() ?? const TvZoom(factor: 1, child: SizedBox.shrink());

  @override
  bool updateShouldNotify(TvZoom old) => old.factor != factor;
}

/// Lays [child] out at the window size divided by [factor] and paints it
/// scaled by [factor], so text, icons, borders and sizes all grow together,
/// as VS Code's zoom does. Painting stays vector, so text stays sharp.
class ZoomScope extends StatelessWidget {
  const ZoomScope({super.key, required this.factor, required this.child});

  final double factor;
  final Widget child;

  @override
  Widget build(BuildContext context) {
    if (factor == 1) return TvZoom(factor: 1, child: child);
    final mq = MediaQuery.of(context);
    final logical = mq.size / factor;
    return TvZoom(
      factor: factor,
      child: MediaQuery(
        data: mq.copyWith(
          size: logical,
          devicePixelRatio: mq.devicePixelRatio * factor,
          padding: mq.padding / factor,
          viewPadding: mq.viewPadding / factor,
          viewInsets: mq.viewInsets / factor,
        ),
        child: ClipRect(
          child: OverflowBox(
            alignment: Alignment.topLeft,
            minWidth: logical.width,
            maxWidth: logical.width,
            minHeight: logical.height,
            maxHeight: logical.height,
            child: Transform.scale(scale: factor, alignment: Alignment.topLeft, child: child),
          ),
        ),
      ),
    );
  }
}

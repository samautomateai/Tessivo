import 'package:material_ui/material_ui.dart';

import '../colors.dart';
import '../icons.dart';
import '../sizes.dart';
import '../type.dart';
import 'tags.dart';

/// Primary: the one main action of an area. Secondary: other actions.
/// Ghost: toolbar actions. Danger: destructive actions.
enum TvButtonKind { primary, secondary, ghost, danger }

/// A Carbon button: 28 px (24 dense), 2 px corners, optional leading icon.
class TvButton extends StatelessWidget {
  const TvButton(
    this.label, {
    super.key,
    this.icon,
    this.onPressed,
    this.kind = TvButtonKind.secondary,
    this.dense = false,
    this.tooltip,
  });

  final String label;
  final TvIcon? icon;
  final VoidCallback? onPressed;
  final TvButtonKind kind;
  final bool dense;
  final String? tooltip;

  @override
  Widget build(BuildContext context) {
    final c = TvColors.of(context);
    final (bg, fg) = switch (kind) {
      TvButtonKind.primary => (c.buttonPrimary, const Color(0xFFFFFFFF)),
      TvButtonKind.secondary => (c.layer02, c.textPrimary),
      TvButtonKind.ghost => (const Color(0x00000000), c.textPrimary),
      TvButtonKind.danger => (c.error, const Color(0xFFFFFFFF)),
    };
    final height = dense ? TvSize.buttonDense : TvSize.button;
    final button = SizedBox(
      height: height,
      child: TextButton(
        onPressed: onPressed,
        style: ButtonStyle(
          minimumSize: WidgetStatePropertyAll(Size(0, height)),
          padding: WidgetStatePropertyAll(EdgeInsets.symmetric(horizontal: dense ? TvSpace.s8 : TvSpace.s12)),
          shape: WidgetStatePropertyAll(RoundedRectangleBorder(borderRadius: BorderRadius.circular(TvRadius.small))),
          backgroundColor: WidgetStatePropertyAll(bg),
          foregroundColor: WidgetStatePropertyAll(fg),
          overlayColor: WidgetStatePropertyAll(c.layerSelected.withValues(alpha: 0.5)),
          animationDuration: TvMotion.fast,
          tapTargetSize: MaterialTapTargetSize.shrinkWrap,
        ),
        child: Opacity(
          opacity: onPressed == null ? 0.5 : 1,
          child: Row(
            mainAxisSize: MainAxisSize.min,
            children: [
              if (icon != null) ...[TvIconView(icon!, color: fg), const SizedBox(width: 6)],
              Text(
                label,
                style: TvType.body.copyWith(color: fg, fontWeight: FontWeight.w500),
              ),
            ],
          ),
        ),
      ),
    );
    return tooltip == null ? button : Tooltip(message: tooltip, child: button);
  }
}

/// A 28 px icon-only button. The tooltip is required: it is also the label
/// screen readers announce. [badge] shows a count in its corner.
class TvIconButton extends StatelessWidget {
  const TvIconButton(this.icon, {super.key, required this.tooltip, this.onPressed, this.badge, this.color});

  final TvIcon icon;
  final String tooltip;
  final VoidCallback? onPressed;
  final int? badge;
  final Color? color;

  @override
  Widget build(BuildContext context) {
    final c = TvColors.of(context);
    return Tooltip(
      message: tooltip,
      child: Semantics(
        button: true,
        label: tooltip,
        excludeSemantics: true,
        onTap: onPressed,
        child: SizedBox.square(
          dimension: TvSize.button,
          child: Material(
            type: MaterialType.transparency,
            child: InkWell(
              onTap: onPressed,
              borderRadius: BorderRadius.circular(TvRadius.small),
              hoverColor: c.layer02,
              child: Stack(
                clipBehavior: Clip.none,
                alignment: Alignment.center,
                children: [
                  Opacity(
                    opacity: onPressed == null ? 0.5 : 1,
                    child: TvIconView(icon, color: color ?? c.textSecondary),
                  ),
                  if ((badge ?? 0) > 0) Positioned(top: 1, right: 0, child: TvBadge(badge!)),
                ],
              ),
            ),
          ),
        ),
      ),
    );
  }
}

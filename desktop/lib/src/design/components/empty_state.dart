import 'package:material_ui/material_ui.dart';

import '../colors.dart';
import '../icons.dart';
import '../sizes.dart';
import '../type.dart';
import 'buttons.dart';

/// What an empty area says: an icon, a heading, one sentence and one action.
class TvEmptyState extends StatelessWidget {
  const TvEmptyState({
    super.key,
    required this.icon,
    required this.title,
    required this.message,
    this.actionLabel,
    this.onAction,
    this.secondaryLabel,
    this.onSecondary,
    this.actionKey,
  });

  final TvIcon icon;
  final String title;
  final String message;
  final String? actionLabel;
  final VoidCallback? onAction;
  final String? secondaryLabel;
  final VoidCallback? onSecondary;

  /// The key of the primary action's button.
  final Key? actionKey;

  @override
  Widget build(BuildContext context) {
    final c = TvColors.of(context);
    return Center(
      child: ConstrainedBox(
        constraints: const BoxConstraints(maxWidth: 420),
        child: Padding(
          padding: const EdgeInsets.all(TvSpace.s32),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              TvIconView(icon, size: 32, color: c.textHelper),
              const SizedBox(height: TvSpace.s16),
              Text(
                title,
                textAlign: TextAlign.center,
                style: TvType.heading02.copyWith(color: c.textPrimary),
              ),
              const SizedBox(height: TvSpace.s8),
              Text(
                message,
                textAlign: TextAlign.center,
                style: TvType.body.copyWith(color: c.textSecondary),
              ),
              if (actionLabel != null || secondaryLabel != null) const SizedBox(height: TvSpace.s24),
              Wrap(
                spacing: TvSpace.s8,
                children: [
                  if (actionLabel != null)
                    TvButton(actionLabel!, key: actionKey, kind: TvButtonKind.primary, onPressed: onAction),
                  if (secondaryLabel != null) TvButton(secondaryLabel!, onPressed: onSecondary),
                ],
              ),
            ],
          ),
        ),
      ),
    );
  }
}

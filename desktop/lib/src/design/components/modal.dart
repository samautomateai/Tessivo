import 'package:material_ui/material_ui.dart';

import '../colors.dart';
import '../sizes.dart';
import '../type.dart';
import 'buttons.dart';

/// A button in a modal's footer; tapping it closes the modal with [value].
class TvModalAction<T> {
  const TvModalAction(this.label, {required this.value, this.kind = TvButtonKind.secondary});

  final String label;
  final T value;
  final TvButtonKind kind;
}

/// A Carbon modal: title, body and right-aligned actions, 4 px corners.
/// Esc or a tap outside closes it with null.
Future<T?> showTvModal<T>(
  BuildContext context, {
  required String title,
  required Widget body,
  required List<TvModalAction<T>> actions,
  double width = 480,
}) => showDialog<T>(
  context: context,
  builder: (context) => TvModal<T>(title: title, body: body, actions: actions, width: width),
);

/// The modal's frame; [showTvModal] shows it.
class TvModal<T> extends StatelessWidget {
  const TvModal({super.key, required this.title, required this.body, required this.actions, this.width = 480});

  final String title;
  final Widget body;
  final List<TvModalAction<T>> actions;
  final double width;

  @override
  Widget build(BuildContext context) {
    final c = TvColors.of(context);
    return Dialog(
      backgroundColor: c.layer01,
      surfaceTintColor: c.layer01,
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.circular(TvRadius.medium),
        side: BorderSide(color: c.borderSubtle),
      ),
      child: ConstrainedBox(
        constraints: BoxConstraints(maxWidth: width),
        child: Padding(
          padding: const EdgeInsets.all(TvSpace.s16),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              Text(title, style: TvType.heading02.copyWith(color: c.textPrimary)),
              const SizedBox(height: TvSpace.s12),
              Flexible(
                child: SingleChildScrollView(
                  child: DefaultTextStyle(
                    style: TvType.bodyLong.copyWith(color: c.textSecondary),
                    child: body,
                  ),
                ),
              ),
              const SizedBox(height: TvSpace.s24),
              Wrap(
                alignment: WrapAlignment.end,
                spacing: TvSpace.s8,
                runSpacing: TvSpace.s8,
                children: [
                  for (final a in actions)
                    TvButton(a.label, kind: a.kind, onPressed: () => Navigator.of(context).pop(a.value)),
                ],
              ),
            ],
          ),
        ),
      ),
    );
  }
}

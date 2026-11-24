import 'package:material_ui/material_ui.dart';

import '../../core/core_client.dart';
import '../../design/colors.dart';
import '../../design/components/notices.dart';
import '../../design/sizes.dart';
import '../../design/type.dart';

/// Runs [action] and shows what went wrong as a toast instead of throwing.
Future<void> guarded(BuildContext context, Future<void> Function() action) async {
  try {
    await action();
  } on Object catch (e) {
    if (!context.mounted) return;
    final text = switch (e) {
      CoreException(:final message) => message,
      FormatException(:final message) => message,
      _ => '$e',
    };
    showTvToast(context, text, kind: TvNoticeKind.error);
  }
}

/// A notebook tab's frame: full width up to the reading width, a heading
/// with its helper line and actions, then [child].
class TabSection extends StatelessWidget {
  const TabSection({super.key, required this.title, required this.child, this.helper, this.actions = const []});

  final String title;
  final String? helper;
  final List<Widget> actions;
  final Widget child;

  @override
  Widget build(BuildContext context) {
    final c = TvColors.of(context);
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      mainAxisSize: MainAxisSize.min,
      children: [
        Row(
          children: [
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(title, style: TvType.heading01.copyWith(color: c.textPrimary)),
                  if (helper case final h?) Text(h, style: TvType.helper.copyWith(color: c.textHelper)),
                ],
              ),
            ),
            for (final (i, a) in actions.indexed) ...[if (i > 0) const SizedBox(width: TvSpace.s8), a],
          ],
        ),
        const SizedBox(height: TvSpace.s12),
        child,
      ],
    );
  }
}

/// The scrolling body of a tab, padded and held to the reading width.
class TabBody extends StatelessWidget {
  const TabBody({super.key, required this.children});

  final List<Widget> children;

  @override
  Widget build(BuildContext context) => ListView(
    padding: const EdgeInsets.fromLTRB(TvSpace.s24, TvSpace.s16, TvSpace.s24, TvSpace.s24),
    children: [
      for (final child in children)
        Align(
          alignment: Alignment.topLeft,
          child: ConstrainedBox(
            constraints: const BoxConstraints(maxWidth: TvSize.readingWidth + 200),
            child: child,
          ),
        ),
    ],
  );
}

/// A one-sentence note where a list is empty.
class EmptyNote extends StatelessWidget {
  const EmptyNote(this.text, {super.key});

  final String text;

  @override
  Widget build(BuildContext context) {
    final c = TvColors.of(context);
    return Container(
      padding: const EdgeInsets.all(TvSpace.s16),
      decoration: BoxDecoration(
        color: c.layer01,
        border: Border.all(color: c.borderSubtle),
      ),
      child: Text(text, style: TvType.body.copyWith(color: c.textSecondary)),
    );
  }
}

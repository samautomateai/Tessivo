import 'package:material_ui/material_ui.dart';

import '../colors.dart';
import '../icons.dart';
import '../sizes.dart';
import '../type.dart';
import 'buttons.dart';

/// A tool waiting for the user's say-so: the tool, its arguments in mono,
/// and Allow once, Always in this chat, Deny. A warning edge marks it.
class ApprovalCard extends StatelessWidget {
  const ApprovalCard({
    super.key,
    required this.tool,
    required this.onAllow,
    required this.onDeny,
    this.args = '',
    this.onAlways,
  });

  final String tool;
  final String args;
  final VoidCallback? onAllow;
  final VoidCallback? onAlways;
  final VoidCallback? onDeny;

  @override
  Widget build(BuildContext context) {
    final c = TvColors.of(context);
    return Container(
      decoration: BoxDecoration(
        color: c.layer01,
        border: Border(left: BorderSide(color: c.warning, width: 3)),
      ),
      padding: const EdgeInsets.all(TvSpace.s12),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        mainAxisSize: MainAxisSize.min,
        children: [
          Row(
            children: [
              TvIconView(TvIcon.warning, color: c.warning),
              const SizedBox(width: TvSpace.s8),
              Flexible(
                child: Text('Allow $tool?', style: TvType.heading01.copyWith(color: c.textPrimary)),
              ),
            ],
          ),
          if (args.isNotEmpty) ...[
            const SizedBox(height: TvSpace.s8),
            Container(
              color: c.background,
              padding: const EdgeInsets.all(TvSpace.s8),
              constraints: const BoxConstraints(maxHeight: 160),
              child: SingleChildScrollView(
                child: SelectableText(args, style: TvType.code.copyWith(color: c.textSecondary)),
              ),
            ),
          ],
          const SizedBox(height: TvSpace.s12),
          Wrap(
            spacing: TvSpace.s8,
            runSpacing: TvSpace.s8,
            children: [
              TvButton('Allow once', kind: TvButtonKind.primary, dense: true, onPressed: onAllow),
              if (onAlways != null) TvButton('Always in this chat', dense: true, onPressed: onAlways),
              TvButton('Deny', kind: TvButtonKind.ghost, dense: true, onPressed: onDeny),
            ],
          ),
        ],
      ),
    );
  }
}

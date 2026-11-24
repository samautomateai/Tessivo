import 'dart:async';

import 'package:material_ui/material_ui.dart';

import '../app_state.dart';
import '../core/models.dart';
import '../design/colors.dart';
import '../design/components/buttons.dart';
import '../design/components/empty_state.dart';
import '../design/components/headers.dart';
import '../design/components/menu.dart';
import '../design/components/table.dart';
import '../design/components/tags.dart';
import '../design/icons.dart';
import '../design/sizes.dart';
import '../design/type.dart';

/// "just now", "5 min ago", "2 h ago", "3 days ago"; '' for an unreadable time.
String waitingSince(String at, DateTime now) {
  final t = DateTime.tryParse(at);
  if (t == null) return '';
  final d = now.difference(t);
  if (d.inMinutes < 1) return 'just now';
  if (d.inHours < 1) return '${d.inMinutes} min ago';
  if (d.inDays < 1) return '${d.inHours} h ago';
  return d.inDays == 1 ? '1 day ago' : '${d.inDays} days ago';
}

/// Every tool call waiting across notebooks, and runs the core interrupted.
class ApprovalsPage extends StatelessWidget {
  const ApprovalsPage({super.key, required this.state});

  final AppState state;

  void _answer(PendingApproval a, {required bool allow, String? remember}) =>
      unawaited(state.answer(a, allow: allow, remember: remember));

  List<Widget> _row(BuildContext context, PendingApproval a, DateTime now) {
    final c = TvColors.of(context);
    if (a.isInterrupted) {
      return [
        Text(a.notebookName),
        const TvTag('interrupted', tone: TvTagTone.yellow),
        Text(
          'Stopped when the core did; a tool call in progress is not repeated.',
          style: TextStyle(color: c.textHelper),
        ),
        Text(waitingSince(a.requestedAt, now)),
        Row(
          mainAxisAlignment: MainAxisAlignment.end,
          children: [
            TvButton(
              'Resume',
              key: const Key('resume-run'),
              kind: TvButtonKind.primary,
              dense: true,
              onPressed: () => unawaited(state.resumeRun(a)),
            ),
            const SizedBox(width: TvSpace.s4),
            TvButton(
              'Cancel run',
              key: const Key('cancel-run'),
              kind: TvButtonKind.ghost,
              dense: true,
              onPressed: () => unawaited(state.cancelRun(a)),
            ),
          ],
        ),
      ];
    }
    return [
      Text(a.notebookName),
      Text(a.tool, style: TvType.code.copyWith(color: c.textPrimary)),
      Tooltip(
        message: a.args,
        child: Text(a.args, style: TvType.code),
      ),
      Text(waitingSince(a.requestedAt, now)),
      Row(
        mainAxisAlignment: MainAxisAlignment.end,
        children: [
          TvButton(
            'Allow',
            key: const Key('approve-once'),
            kind: TvButtonKind.primary,
            dense: true,
            onPressed: () => _answer(a, allow: true),
          ),
          TvMenuButton(
            key: Key('approval-more-${a.approvalId}'),
            label: 'More ways to allow',
            icon: TvIcon.more,
            compact: true,
            items: [
              TvMenuItem(
                'Allow for this run',
                key: const Key('approve-run'),
                onSelected: () => _answer(a, allow: true, remember: 'run'),
              ),
              TvMenuItem(
                'Allow for this notebook',
                key: const Key('approve-notebook'),
                onSelected: () => _answer(a, allow: true, remember: 'notebook'),
              ),
            ],
          ),
          TvButton(
            'Deny',
            key: const Key('deny'),
            kind: TvButtonKind.ghost,
            dense: true,
            onPressed: () => _answer(a, allow: false),
          ),
        ],
      ),
    ];
  }

  @override
  Widget build(BuildContext context) {
    final c = TvColors.of(context);
    final list = state.approvals;
    final now = DateTime.now();
    return ColoredBox(
      color: c.background,
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          TvPageHeader(title: 'Approvals', meta: list.isEmpty ? null : '${list.length} waiting'),
          Expanded(
            child: list.isEmpty
                ? const TvEmptyState(
                    key: Key('no-approvals'),
                    icon: TvIcon.approvals,
                    title: 'Nothing is waiting',
                    message: 'When an agent run wants a tool that needs your say-so, it waits here.',
                  )
                : ListView(
                    padding: const EdgeInsets.fromLTRB(TvSpace.s24, TvSpace.s16, TvSpace.s24, TvSpace.s24),
                    children: [
                      TvDataTable(
                        key: const Key('approvals-table'),
                        columns: const ['Run', 'Tool', 'Arguments', 'Waiting since', ''],
                        flex: const [3, 3, 5, 2, 4],
                        rows: [for (final a in list) _row(context, a, now)],
                      ),
                    ],
                  ),
          ),
        ],
      ),
    );
  }
}

/// A tool call of the run on screen waiting for an answer (notebook page).
class ApprovalTile extends StatelessWidget {
  const ApprovalTile({super.key, required this.state, required this.approval});

  final AppState state;
  final PendingApproval approval;

  void _answer({required bool allow, String? remember}) =>
      unawaited(state.answer(approval, allow: allow, remember: remember));

  @override
  Widget build(BuildContext context) {
    final c = TvColors.of(context);
    return Container(
      margin: const EdgeInsets.symmetric(vertical: TvSpace.s8),
      padding: const EdgeInsets.all(TvSpace.s12),
      decoration: BoxDecoration(
        color: c.layer02,
        border: Border(left: BorderSide(color: c.warning, width: 3)),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text('Allow ${approval.tool}?', style: TvType.heading01.copyWith(color: c.textPrimary)),
          if (approval.args.isNotEmpty)
            SelectableText(approval.args, style: TvType.code.copyWith(color: c.textSecondary)),
          const SizedBox(height: TvSpace.s8),
          Wrap(
            spacing: TvSpace.s8,
            runSpacing: TvSpace.s4,
            children: [
              TvButton(
                'Allow once',
                key: const Key('approve-once'),
                kind: TvButtonKind.primary,
                dense: true,
                onPressed: () => _answer(allow: true),
              ),
              TvButton(
                'Allow for this run',
                key: const Key('approve-run'),
                dense: true,
                onPressed: () => _answer(allow: true, remember: 'run'),
              ),
              TvButton(
                'Allow for this notebook',
                key: const Key('approve-notebook'),
                dense: true,
                onPressed: () => _answer(allow: true, remember: 'notebook'),
              ),
              TvButton(
                'Deny',
                key: const Key('deny'),
                kind: TvButtonKind.ghost,
                dense: true,
                onPressed: () => _answer(allow: false),
              ),
            ],
          ),
        ],
      ),
    );
  }
}

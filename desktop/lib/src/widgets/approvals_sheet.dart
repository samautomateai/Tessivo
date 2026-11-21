import 'dart:async';

import 'package:material_ui/material_ui.dart';

import '../app_state.dart';
import '../core/models.dart';
import '../theme.dart';

/// Opens a dialog listing every approval waiting across notebooks.
Future<void> showApprovalsSheet(BuildContext context, AppState state) => showDialog<void>(
  context: context,
  builder: (context) => ListenableBuilder(
    listenable: state,
    builder: (context, _) => AlertDialog(
      title: const Text('Waiting for approval'),
      content: SizedBox(
        width: 640,
        child: state.approvals.isEmpty
            ? const Text('Nothing is waiting.', style: TextStyle(color: Tokens.textMuted))
            : ListView(
                shrinkWrap: true,
                children: [for (final a in state.approvals) ApprovalTile(state: state, approval: a)],
              ),
      ),
      actions: [TextButton(onPressed: () => Navigator.of(context).pop(), child: const Text('Close'))],
    ),
  ),
);

/// One tool call waiting for an answer: what it wants to do and the answers.
class ApprovalTile extends StatelessWidget {
  const ApprovalTile({super.key, required this.state, required this.approval});

  final AppState state;
  final PendingApproval approval;

  void _answer({required bool allow, String? remember}) =>
      unawaited(state.answer(approval, allow: allow, remember: remember));

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 8),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(approval.tool, style: Theme.of(context).textTheme.titleSmall),
          if (approval.notebookName.isNotEmpty)
            Text(approval.notebookName, style: const TextStyle(color: Tokens.textMuted, fontSize: 12)),
          const SizedBox(height: 4),
          SelectableText(approval.args, style: const TextStyle(fontFamily: 'monospace', fontSize: 12)),
          const SizedBox(height: 8),
          Wrap(
            spacing: 8,
            runSpacing: 4,
            children: [
              FilledButton(
                key: const Key('approve-once'),
                onPressed: () => _answer(allow: true),
                child: const Text('Allow once'),
              ),
              OutlinedButton(
                key: const Key('approve-run'),
                onPressed: () => _answer(allow: true, remember: 'run'),
                child: const Text('Allow for this run'),
              ),
              OutlinedButton(
                key: const Key('approve-notebook'),
                onPressed: () => _answer(allow: true, remember: 'notebook'),
                child: const Text('Allow for this notebook'),
              ),
              TextButton(key: const Key('deny'), onPressed: () => _answer(allow: false), child: const Text('Deny')),
            ],
          ),
        ],
      ),
    );
  }
}

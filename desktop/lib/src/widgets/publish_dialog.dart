import 'dart:async';

import 'package:material_ui/material_ui.dart';

import '../app_state.dart';
import '../core/core_client.dart';
import '../core/models.dart';
import '../design/components/buttons.dart';
import '../design/components/fields.dart';
import '../design/components/notices.dart';
import '../design/sizes.dart';
import 'send_menu.dart';

/// Picks a folder, Confluence space or Notion page and publishes the output
/// there; a version published before is replaced. Closes with a message for a toast.
class PublishDialog extends StatefulWidget {
  const PublishDialog({super.key, required this.state, required this.notebookId, required this.outputId});

  final AppState state;
  final String notebookId;
  final String outputId;

  @override
  State<PublishDialog> createState() => _PublishDialogState();
}

class _PublishDialogState extends State<PublishDialog> {
  CoreClient get core => widget.state.core;
  List<IntegrationInfo>? _targets;
  String? _target;
  String? _error;
  bool _busy = false;

  @override
  void initState() {
    super.initState();
    unawaited(_load());
  }

  Future<void> _load() async {
    try {
      final all = await core.listIntegrations();
      if (!mounted) return;
      setState(() {
        _targets = all.where((i) => !i.forIssues).toList();
        _target = _targets!.firstOrNull?.name;
      });
    } on CoreException catch (e) {
      if (mounted) {
        setState(() {
          _targets = const [];
          _error = e.message;
        });
      }
    }
  }

  Future<void> _publish() async {
    final target = _target;
    if (target == null) return;
    setState(() {
      _busy = true;
      _error = null;
    });
    try {
      final d = await core.publishOutput(widget.notebookId, widget.outputId, target);
      if (mounted) Navigator.of(context).pop('Published to $target: ${d.url}');
    } on CoreException catch (e) {
      if (mounted) {
        setState(() {
          _busy = false;
          _error = e.message;
        });
      }
    }
  }

  @override
  Widget build(BuildContext context) {
    final targets = _targets;
    return AlertDialog(
      key: const Key('publish-dialog'),
      title: const Text('Publish'),
      content: SizedBox(
        width: 520,
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            if (_error case final e?) ...[
              TvInlineNotice(kind: TvNoticeKind.error, title: 'Not published', message: e),
              const SizedBox(height: TvSpace.s12),
            ],
            if (targets == null)
              const SizedBox(height: 80, child: Center(child: CircularProgressIndicator()))
            else if (targets.isEmpty)
              NoIntegrations(state: widget.state, what: 'a folder, Confluence or Notion')
            else
              TvDropdown<String>(
                key: const Key('publish-target'),
                label: 'Publish to',
                helper: 'Publishing again, or a newer version, replaces the page or file (Notion makes a new page).',
                items: [for (final t in targets) (t.name, '${t.name} · ${t.kind} · ${t.target}')],
                value: _target,
                onChanged: (v) => setState(() => _target = v),
              ),
          ],
        ),
      ),
      actions: [
        TvButton('Cancel', kind: TvButtonKind.ghost, onPressed: () => Navigator.of(context).pop()),
        TvButton(
          'Publish',
          key: const Key('publish-go'),
          kind: TvButtonKind.primary,
          onPressed: _busy || _target == null ? null : () => unawaited(_publish()),
        ),
      ],
    );
  }
}

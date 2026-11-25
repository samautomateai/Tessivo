import 'dart:async';

import 'package:material_ui/material_ui.dart';

import '../app_state.dart';
import '../core/core_client.dart';
import '../core/models.dart';
import '../design/colors.dart';
import '../design/components/buttons.dart';
import '../design/components/fields.dart';
import '../design/components/notices.dart';
import '../design/components/tags.dart';
import '../design/sizes.dart';
import '../design/type.dart';
import 'send_menu.dart';

/// Picks an Analyze output's findings and a tracker, and files them as issues.
/// Findings already sent to that tracker start unticked and are marked.
/// Closes with a message for a toast.
class IssuesDialog extends StatefulWidget {
  const IssuesDialog({super.key, required this.state, required this.notebookId, required this.outputId});

  final AppState state;
  final String notebookId;
  final String outputId;

  @override
  State<IssuesDialog> createState() => _IssuesDialogState();
}

class _IssuesDialogState extends State<IssuesDialog> {
  CoreClient get core => widget.state.core;
  List<IssueInfo>? _issues;
  List<IntegrationInfo> _trackers = const [];
  List<Delivery> _sent = const [];
  String? _target;
  final Set<int> _picked = {};
  String? _error;
  bool _busy = false;

  @override
  void initState() {
    super.initState();
    unawaited(_load());
  }

  Future<void> _load() async {
    try {
      final issues = await core.outputIssues(widget.notebookId, widget.outputId);
      final all = await core.listIntegrations();
      final sent = await core.deliveries(widget.notebookId, widget.outputId);
      if (!mounted) return;
      setState(() {
        _issues = issues;
        _trackers = all.where((i) => i.forIssues).toList();
        _sent = sent;
        _target = _trackers.firstOrNull?.name;
        _pickUnsent();
      });
    } on CoreException catch (e) {
      if (mounted) {
        setState(() {
          _issues = const [];
          _error = e.message;
        });
      }
    }
  }

  bool _wasSent(int index) => _sent.any((d) => d.integration == _target && d.finding == index);

  void _pickUnsent() {
    _picked
      ..clear()
      ..addAll([
        for (final i in _issues ?? const <IssueInfo>[])
          if (!_wasSent(i.index)) i.index,
      ]);
  }

  Future<void> _push() async {
    final target = _target;
    if (target == null || _picked.isEmpty) return;
    setState(() {
      _busy = true;
      _error = null;
    });
    try {
      final picked = _picked.toList()..sort();
      final res = await core.pushIssues(
        widget.notebookId,
        widget.outputId,
        target,
        findings: picked,
        again: picked.any(_wasSent),
      );
      if (!mounted) return;
      final n = res.sent.length;
      Navigator.of(context).pop('Created $n issue${n == 1 ? '' : 's'} in $target');
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
    final issues = _issues;
    final n = _picked.length;
    return AlertDialog(
      key: const Key('issues-dialog'),
      title: const Text('Create issues'),
      content: SizedBox(
        width: 620,
        child: issues == null
            ? const SizedBox(height: 120, child: Center(child: CircularProgressIndicator()))
            : _body(context, issues),
      ),
      actions: [
        TvButton('Cancel', kind: TvButtonKind.ghost, onPressed: () => Navigator.of(context).pop()),
        TvButton(
          n == 1 ? 'Create 1 issue' : 'Create $n issues',
          key: const Key('issues-push'),
          kind: TvButtonKind.primary,
          onPressed: _busy || n == 0 || _target == null ? null : () => unawaited(_push()),
        ),
      ],
    );
  }

  Widget _body(BuildContext context, List<IssueInfo> issues) {
    final c = TvColors.of(context);
    return Column(
      mainAxisSize: MainAxisSize.min,
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        if (_error case final e?) ...[
          TvInlineNotice(kind: TvNoticeKind.error, title: 'Not sent', message: e),
          const SizedBox(height: TvSpace.s12),
        ],
        if (_trackers.isEmpty && issues.isNotEmpty)
          NoIntegrations(state: widget.state, what: 'GitHub, GitLab or Jira')
        else if (_trackers.isNotEmpty)
          TvDropdown<String>(
            key: const Key('issue-target'),
            label: 'Tracker',
            items: [for (final t in _trackers) (t.name, '${t.name} · ${t.kind} · ${t.target}')],
            value: _target,
            onChanged: (v) => setState(() {
              _target = v;
              _pickUnsent();
            }),
          ),
        const SizedBox(height: TvSpace.s12),
        Flexible(
          child: ListView(
            shrinkWrap: true,
            children: [
              for (final i in issues)
                Padding(
                  padding: const EdgeInsets.only(bottom: TvSpace.s4),
                  child: Wrap(
                    spacing: TvSpace.s8,
                    crossAxisAlignment: WrapCrossAlignment.center,
                    children: [
                      TvCheckbox(
                        key: Key('issue-${i.index}'),
                        label: i.title,
                        value: _picked.contains(i.index),
                        onChanged: (v) => setState(() => v ? _picked.add(i.index) : _picked.remove(i.index)),
                      ),
                      if (i.severity.isNotEmpty) TvTag(i.severity),
                      if (_wasSent(i.index)) const TvTag('Sent', tone: TvTagTone.green),
                    ],
                  ),
                ),
              if (issues.isEmpty && _error == null)
                Text('This output has no findings.', style: TvType.body.copyWith(color: c.textSecondary)),
            ],
          ),
        ),
      ],
    );
  }
}

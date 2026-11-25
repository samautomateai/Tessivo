import 'dart:async';

import 'package:material_ui/material_ui.dart';

import '../app_state.dart';
import '../core/core_client.dart';
import '../core/models.dart';
import '../design/components/buttons.dart';
import '../design/components/notices.dart';
import '../design/sizes.dart';

/// A notice on the Sources tab when the sources point to another work type
/// (WT-7), with Switch and Dismiss. Asked again whenever the sources change.
class WorkTypeHint extends StatefulWidget {
  const WorkTypeHint({super.key, required this.state, required this.notebook});

  final AppState state;
  final Notebook notebook;

  @override
  State<WorkTypeHint> createState() => _WorkTypeHintState();
}

class _WorkTypeHintState extends State<WorkTypeHint> {
  WorkTypeSuggestion _sug = const WorkTypeSuggestion();
  final Set<String> _dismissed = {};

  String get _sources =>
      '${widget.notebook.id}:${widget.notebook.workType.name}:'
      '${[for (final c in widget.notebook.context)
        if (c.enabled) c.id].join(',')}';

  @override
  void initState() {
    super.initState();
    unawaited(_ask());
  }

  @override
  void didUpdateWidget(WorkTypeHint old) {
    super.didUpdateWidget(old);
    final before =
        '${old.notebook.id}:${old.notebook.workType.name}:'
        '${[for (final c in old.notebook.context)
          if (c.enabled) c.id].join(',')}';
    if (before != _sources) unawaited(_ask());
  }

  Future<void> _ask() async {
    final key = _sources;
    try {
      final sug = await widget.state.core.workTypeSuggestion(widget.notebook.id);
      if (mounted && key == _sources) setState(() => _sug = sug);
    } on CoreException {
      if (mounted) setState(() => _sug = const WorkTypeSuggestion());
    }
  }

  @override
  Widget build(BuildContext context) {
    final wt = WorkTypeId.values.where((w) => w.name == _sug.workType).firstOrNull;
    if (wt == null || _dismissed.contains(_sources)) return const SizedBox.shrink();
    final label = wt.name[0].toUpperCase() + wt.name.substring(1);
    return Padding(
      padding: const EdgeInsets.only(bottom: TvSpace.s16),
      child: TvInlineNotice(
        key: const Key('worktype-hint'),
        kind: TvNoticeKind.info,
        title: 'These sources look like $label work',
        message: 'Tessivo suggests $label because ${_sug.reason}. You can switch until the first output.',
        action: Wrap(
          spacing: TvSpace.s8,
          children: [
            TvButton(
              'Switch to $label',
              key: const Key('worktype-switch'),
              dense: true,
              onPressed: () => unawaited(widget.state.setWorkType(widget.notebook.id, wt)),
            ),
            TvButton(
              'Dismiss',
              key: const Key('worktype-dismiss'),
              kind: TvButtonKind.ghost,
              dense: true,
              onPressed: () => setState(() => _dismissed.add(_sources)),
            ),
          ],
        ),
      ),
    );
  }
}

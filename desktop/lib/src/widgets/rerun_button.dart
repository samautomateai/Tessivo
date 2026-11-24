import 'dart:async';

import 'package:material_ui/material_ui.dart';

import '../app_state.dart';
import '../core/models.dart';
import '../design/components/buttons.dart';
import '../design/icons.dart';

/// "Run again": repeats the run behind the open output, optionally on another model.
class RerunButton extends StatelessWidget {
  const RerunButton({
    super.key,
    required this.state,
    required this.outputId,
    required this.enabled,
    this.compact = false,
  });

  final AppState state;
  final String outputId;
  final bool enabled;
  final bool compact;

  Future<void> _open(BuildContext context) async {
    final info = await state.runOf(outputId);
    if (info == null || !context.mounted) return;
    final model = await showDialog<String>(
      context: context,
      builder: (_) => _RerunDialog(info: info),
    );
    if (model != null) await state.rerun(info, model: model.trim().isEmpty ? null : model.trim());
  }

  @override
  Widget build(BuildContext context) => TvToolButton(
    'Run again',
    key: const Key('rerun'),
    icon: TvIcon.runAgain,
    compact: compact,
    onPressed: enabled ? () => unawaited(_open(context)) : null,
  );
}

class _RerunDialog extends StatefulWidget {
  const _RerunDialog({required this.info});

  final RunInfo info;

  @override
  State<_RerunDialog> createState() => _RerunDialogState();
}

class _RerunDialogState extends State<_RerunDialog> {
  late final _model = TextEditingController(text: widget.info.model);

  @override
  void dispose() {
    _model.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) => AlertDialog(
    title: const Text('Run again'),
    content: SizedBox(
      width: 520,
      child: Column(
        mainAxisSize: MainAxisSize.min,
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            widget.info.promptName == null ? widget.info.prompt : 'Saved prompt: ${widget.info.promptName}',
            maxLines: 4,
            overflow: TextOverflow.ellipsis,
          ),
          const SizedBox(height: 12),
          TextField(
            key: const Key('rerun-model'),
            controller: _model,
            decoration: const InputDecoration(
              labelText: 'Model',
              helperText: 'An alias or provider/model. Empty: the settings that apply now.',
            ),
          ),
        ],
      ),
    ),
    actions: [
      TextButton(onPressed: () => Navigator.of(context).pop(), child: const Text('Cancel')),
      FilledButton(
        key: const Key('rerun-go'),
        onPressed: () => Navigator.of(context).pop(_model.text),
        child: const Text('Run'),
      ),
    ],
  );
}

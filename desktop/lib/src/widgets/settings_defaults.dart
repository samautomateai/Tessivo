import 'package:material_ui/material_ui.dart';

import '../core/models.dart';
import '../core/settings_draft.dart';
import '../design/colors.dart';
import '../design/type.dart';

/// Run defaults and token limits, edited in place and saved together.
class DefaultsEditor extends StatefulWidget {
  const DefaultsEditor({super.key, required this.settings, required this.onSave});

  final SettingsInfo settings;

  /// Saves the edited draft; returns an error message or null.
  final Future<String?> Function(SettingsDraft draft) onSave;

  @override
  State<DefaultsEditor> createState() => _DefaultsEditorState();
}

class _DefaultsEditorState extends State<DefaultsEditor> {
  late final _model = TextEditingController(text: widget.settings.defaultModel ?? '');
  late final _maxOut = TextEditingController(text: _n(widget.settings.maxOutputTokens));
  late final _perRun = TextEditingController(text: _n(widget.settings.maxTokensPerRun));
  late final _perDay = TextEditingController(text: _n(widget.settings.maxTokensPerDay));
  late String _thinking = widget.settings.thinkingEffort ?? 'off';
  String? _numberError;
  String? _status;

  static String _n(int? v) => v == null ? '' : '$v';

  @override
  void dispose() {
    for (final c in [_model, _maxOut, _perRun, _perDay]) {
      c.dispose();
    }
    super.dispose();
  }

  /// '' is "not set"; anything else must be a whole number ≥ 0.
  int? _parse(TextEditingController c) => c.text.trim().isEmpty ? 0 : int.tryParse(c.text.trim());

  Future<void> _save() async {
    final nums = [_maxOut, _perRun, _perDay].map(_parse).toList();
    if (nums.any((n) => n == null || n < 0)) {
      setState(() => _numberError = 'Enter a whole number');
      return;
    }
    final draft = SettingsDraft.fromJson(widget.settings.raw)
      ..setDefault('model', _model.text.trim())
      ..setDefault('thinking_effort', _thinking == 'off' ? null : _thinking)
      ..setDefault('max_output_tokens', nums[0] == 0 ? null : nums[0])
      ..setLimit('max_tokens_per_run', nums[1])
      ..setLimit('max_tokens_per_day', nums[2]);
    final err = await widget.onSave(draft);
    if (!mounted) return;
    setState(() {
      _numberError = null;
      _status = err ?? 'Saved';
    });
  }

  Widget _num(String key, String label, TextEditingController c) => SizedBox(
    width: 200,
    child: TextField(
      key: Key(key),
      controller: c,
      decoration: InputDecoration(labelText: label, hintText: 'not set'),
    ),
  );

  @override
  Widget build(BuildContext context) {
    final aliases = widget.settings.aliases.map((a) => a.name).join(', ');
    return Card(
      key: const Key('defaults'),
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text('Run defaults and limits', style: Theme.of(context).textTheme.titleSmall),
            const SizedBox(height: 8),
            Wrap(
              spacing: 16,
              runSpacing: 8,
              crossAxisAlignment: WrapCrossAlignment.end,
              children: [
                SizedBox(
                  width: 260,
                  child: TextField(
                    key: const Key('default-model'),
                    controller: _model,
                    decoration: InputDecoration(
                      labelText: 'Default model',
                      helperText: aliases.isEmpty ? 'provider/model' : 'an alias ($aliases) or provider/model',
                    ),
                  ),
                ),
                DropdownButton<String>(
                  key: const Key('default-thinking'),
                  value: _thinking,
                  items: [
                    for (final t in const ['off', 'low', 'medium', 'high'])
                      DropdownMenuItem(value: t, child: Text('Thinking: $t')),
                  ],
                  onChanged: (v) => setState(() => _thinking = v ?? 'off'),
                ),
                _num('max-output', 'Max output tokens', _maxOut),
                _num('limit-run', 'Tokens per run', _perRun),
                _num('limit-day', 'Tokens per day', _perDay),
              ],
            ),
            if (_numberError != null)
              Padding(
                padding: const EdgeInsets.only(top: 8),
                child: Text(_numberError!, style: TextStyle(color: Theme.of(context).colorScheme.error)),
              ),
            const SizedBox(height: 12),
            Row(
              children: [
                FilledButton(key: const Key('save-defaults'), onPressed: _save, child: const Text('Save')),
                const SizedBox(width: 12),
                if (_status != null)
                  Text(_status!, style: TvType.helper.copyWith(color: TvColors.of(context).textHelper)),
              ],
            ),
            const SizedBox(height: 8),
            Text(
              'Overridden by: notebook › work type › prompt › single run. A limit of 0 or empty means none; '
              'a warning comes at 80% of the daily limit.',
              style: TvType.helper.copyWith(color: TvColors.of(context).textHelper),
            ),
          ],
        ),
      ),
    );
  }
}

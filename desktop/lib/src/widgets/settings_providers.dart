import 'dart:async';

import 'package:material_ui/material_ui.dart';

import '../core/core_client.dart';
import '../core/models.dart';
import '../core/settings_draft.dart';
import '../design/colors.dart';
import '../design/components/buttons.dart';
import '../design/components/notices.dart';
import '../design/components/table.dart';
import '../design/sizes.dart';
import '../design/type.dart';
import 'model_check_dialog.dart';

/// A provider: its URL, whether its API key is set (never the key), a way to
/// set the key, and its live model list.
class ProviderCard extends StatefulWidget {
  const ProviderCard({super.key, required this.core, required this.provider, this.onKeyChanged});

  final CoreClient core;
  final ProviderInfo provider;
  final VoidCallback? onKeyChanged;

  @override
  State<ProviderCard> createState() => _ProviderCardState();
}

class _ProviderCardState extends State<ProviderCard> {
  late Future<bool> _keySet = _check();
  Future<List<String>>? _models;

  Future<bool> _check() async {
    final ref = widget.provider.keyRef;
    return ref == null || await widget.core.secretSet(ref);
  }

  Future<void> _setKey() async {
    final ref = widget.provider.keyRef;
    if (ref == null) return;
    final value = await showDialog<String>(
      context: context,
      builder: (_) => _KeyDialog(name: widget.provider.name),
    );
    if (value == null || value.isEmpty) return;
    try {
      await widget.core.setSecret(ref, value);
    } on CoreException catch (e) {
      if (mounted) ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(e.message)));
      return;
    }
    setState(() {
      _keySet = _check();
    });
    widget.onKeyChanged?.call();
  }

  void _loadModels() {
    final name = widget.provider.name;
    setState(() {
      _models = widget.core.listModels(provider: name).then((m) => m[name] ?? const []);
    });
  }

  @override
  Widget build(BuildContext context) {
    final p = widget.provider;
    final models = _models;
    final c = TvColors.of(context);
    return Container(
      key: Key('provider-${p.name}'),
      margin: const EdgeInsets.only(bottom: TvSpace.s8),
      decoration: BoxDecoration(
        color: c.layer01,
        border: Border.all(color: c.borderSubtle),
        borderRadius: BorderRadius.circular(TvRadius.small),
      ),
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                Expanded(
                  child: Text(p.name, style: TvType.heading01.copyWith(color: c.textPrimary)),
                ),
                if (p.keyRef != null)
                  TextButton(key: Key('set-key-${p.name}'), onPressed: _setKey, child: const Text('Set API key…')),
                TextButton(
                  key: Key('load-models-${p.name}'),
                  onPressed: _loadModels,
                  child: Text(models == null ? 'Show models' : 'Refresh'),
                ),
              ],
            ),
            Text(p.baseUrl.isEmpty ? p.kindLabel : '${p.kindLabel} · ${p.baseUrl}'),
            FutureBuilder<bool>(future: _keySet, builder: (context, snap) => _keyStatus(context, p, snap)),
            if (p.keyRef != null)
              Text('Stored in your OS keychain as ${p.keyRef}', style: TvType.helper.copyWith(color: c.textHelper)),
            if (models != null) _ModelList(models),
          ],
        ),
      ),
    );
  }

  Widget _keyStatus(BuildContext context, ProviderInfo p, AsyncSnapshot<bool> snap) {
    final missing = p.keyRef != null && snap.data == false;
    final text = p.keyRef == null
        ? 'No API key'
        : !snap.hasData
        ? 'Checking key…'
        : (snap.data! ? 'Key set' : 'Key not set');
    final c = TvColors.of(context);
    return Text(text, style: TvType.helper.copyWith(color: missing ? c.error : c.textHelper));
  }
}

class _ModelList extends StatelessWidget {
  const _ModelList(this.models);

  final Future<List<String>> models;

  @override
  Widget build(BuildContext context) => FutureBuilder<List<String>>(
    future: models,
    builder: (context, snap) {
      if (snap.hasError) {
        return Text(
          'Could not list models: ${snap.error}',
          style: TvType.helper.copyWith(color: TvColors.of(context).error),
        );
      }
      if (!snap.hasData) return const LinearProgressIndicator();
      if (snap.data!.isEmpty) return const Text('The provider reported no models.');
      return Padding(
        padding: const EdgeInsets.only(top: 8),
        child: Wrap(spacing: 6, runSpacing: 6, children: [for (final m in snap.data!) Chip(label: Text(m))]),
      );
    },
  );
}

/// Asks for an API key in an obscured field; returns it or null.
class _KeyDialog extends StatefulWidget {
  const _KeyDialog({required this.name});

  final String name;

  @override
  State<_KeyDialog> createState() => _KeyDialogState();
}

class _KeyDialogState extends State<_KeyDialog> {
  final _value = TextEditingController();

  @override
  void dispose() {
    _value.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) => AlertDialog(
    title: Text('API key for ${widget.name}'),
    content: SizedBox(
      width: 460,
      child: TextField(
        key: const Key('key-value'),
        controller: _value,
        obscureText: true,
        autocorrect: false,
        enableSuggestions: false,
        decoration: const InputDecoration(
          labelText: 'API key',
          helperText: 'Saved to your OS keychain by the core. It is never shown again.',
        ),
      ),
    ),
    actions: [
      TextButton(onPressed: () => Navigator.of(context).pop(), child: const Text('Cancel')),
      FilledButton(
        key: const Key('save-key'),
        onPressed: () => Navigator.of(context).pop(_value.text.trim()),
        child: const Text('Save'),
      ),
    ],
  );
}

/// A known provider: its API kind, base URL and whether it needs a key.
class ProviderPreset {
  const ProviderPreset(this.label, this.name, this.kind, this.url, {this.needsKey = true});
  final String label;
  final String name;
  final String kind;
  final String url;
  final bool needsKey;
}

/// Providers the Add provider dialog can fill in. The OpenAI-compatible ones
/// differ only in URL; Anthropic and Gemini use their native APIs.
const providerPresets = [
  ProviderPreset('Custom (OpenAI-compatible)', '', 'openai', ''),
  ProviderPreset('LiteLLM proxy', 'litellm', 'openai', 'http://localhost:4000/v1'),
  ProviderPreset('OpenAI', 'openai', 'openai', 'https://api.openai.com/v1'),
  ProviderPreset('Anthropic (native API)', 'anthropic', 'anthropic', ''),
  ProviderPreset('Google Gemini (native API)', 'gemini', 'gemini', ''),
  ProviderPreset('Ollama (local)', 'ollama', 'openai', 'http://localhost:11434/v1', needsKey: false),
  ProviderPreset('LM Studio (local)', 'lmstudio', 'openai', 'http://localhost:1234/v1', needsKey: false),
  ProviderPreset('vLLM (local)', 'vllm', 'openai', 'http://localhost:8000/v1', needsKey: false),
  ProviderPreset('OpenRouter', 'openrouter', 'openai', 'https://openrouter.ai/api/v1'),
  ProviderPreset('Groq', 'groq', 'openai', 'https://api.groq.com/openai/v1'),
  ProviderPreset('DeepSeek', 'deepseek', 'openai', 'https://api.deepseek.com/v1'),
  ProviderPreset('Mistral', 'mistral', 'openai', 'https://api.mistral.ai/v1'),
  ProviderPreset('xAI', 'xai', 'openai', 'https://api.x.ai/v1'),
  ProviderPreset('Together AI', 'together', 'openai', 'https://api.together.xyz/v1'),
];

/// A provider to add: name, URL ("" for a native API's default), kind and whether it needs a key.
typedef NewProvider = ({String name, String url, String kind, bool needsKey});

/// Adds a provider from a preset or by hand; its key reference is NAME_KEY.
class AddProviderDialog extends StatefulWidget {
  const AddProviderDialog({super.key});

  @override
  State<AddProviderDialog> createState() => _AddProviderDialogState();
}

class _AddProviderDialogState extends State<AddProviderDialog> {
  final _name = TextEditingController();
  final _url = TextEditingController();
  ProviderPreset _preset = providerPresets.first;

  @override
  void dispose() {
    _name.dispose();
    _url.dispose();
    super.dispose();
  }

  void _pick(ProviderPreset? p) {
    if (p == null) return;
    setState(() {
      _preset = p;
      _name.text = p.name;
      _url.text = p.url;
    });
  }

  @override
  Widget build(BuildContext context) {
    final native = _preset.kind != 'openai';
    return AlertDialog(
      title: const Text('Add a model provider'),
      content: SizedBox(
        width: 460,
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            DropdownButtonFormField<ProviderPreset>(
              key: const Key('provider-preset'),
              isExpanded: true,
              initialValue: _preset,
              decoration: const InputDecoration(labelText: 'Provider'),
              items: [for (final p in providerPresets) DropdownMenuItem(value: p, child: Text(p.label))],
              onChanged: _pick,
            ),
            TextField(
              key: const Key('provider-name'),
              controller: _name,
              decoration: const InputDecoration(labelText: 'Name', helperText: 'Used in model names: name/model'),
            ),
            TextField(
              key: const Key('provider-url'),
              controller: _url,
              decoration: InputDecoration(
                labelText: 'Base URL',
                helperText: native ? 'Leave empty for the official API' : 'OpenAI-compatible, ending in /v1',
              ),
            ),
            if (native)
              Padding(
                padding: const EdgeInsets.only(top: 8),
                child: Text(
                  'Requests go straight to this cloud provider, not through your own proxy.',
                  style: TvType.helper.copyWith(color: TvColors.of(context).textHelper),
                ),
              ),
          ],
        ),
      ),
      actions: [
        TextButton(onPressed: () => Navigator.of(context).pop(), child: const Text('Cancel')),
        FilledButton(
          key: const Key('save-provider'),
          onPressed: () => Navigator.of(context).pop<NewProvider>((
            name: _name.text.trim(),
            url: _url.text.trim(),
            kind: _preset.kind,
            needsKey: _preset.needsKey,
          )),
          child: const Text('Add'),
        ),
      ],
    );
  }
}

/// The key reference a new provider's API key is stored under.
String keyRefFor(String provider) => '${provider.toUpperCase().replaceAll(RegExp('[^A-Z0-9]+'), '_')}_KEY';

/// An alias's fallback chain, editable as one "provider/model" per line.
/// The aliases as a data table: name, fallback chain, when it falls back, and
/// Test and Edit… for each.
class AliasesTable extends StatelessWidget {
  const AliasesTable({super.key, required this.settings, required this.onSave, this.onTest});

  final SettingsInfo settings;
  final Future<String?> Function(SettingsDraft draft) onSave;

  /// Checks each model in the chain; the Test button shows when it is set.
  final Future<List<ModelCheck>> Function(String model)? onTest;

  Future<void> _edit(BuildContext context, AliasInfo alias) async {
    final lines = await showDialog<List<String>>(
      context: context,
      builder: (_) => _ChainDialog(alias: alias),
    );
    if (lines == null || lines.isEmpty) return;
    final err = await onSave(SettingsDraft.fromJson(settings.raw)..setAliasChain(alias.name, lines));
    if (err != null && context.mounted) showTvToast(context, err, kind: TvNoticeKind.error);
  }

  @override
  Widget build(BuildContext context) {
    final test = onTest;
    return TvDataTable(
      key: const Key('aliases-table'),
      columns: const ['Alias', 'Fallback chain', 'Falls back on', ''],
      flex: const [2, 5, 3, 3],
      rows: [
        for (final a in settings.aliases)
          [
            Text(a.name, key: Key('alias-${a.name}')),
            Tooltip(message: a.chain.join('\n'), child: Text(a.chain.join('  →  '))),
            Text(a.fallbackOn.isEmpty ? '—' : a.fallbackOn.join(', ')),
            Row(
              mainAxisAlignment: MainAxisAlignment.end,
              children: [
                if (test != null)
                  Flexible(
                    child: TvButton(
                      'Test',
                      key: Key('test-alias-${a.name}'),
                      kind: TvButtonKind.ghost,
                      dense: true,
                      onPressed: () => unawaited(showModelCheck(context, a.name, test)),
                    ),
                  ),
                Flexible(
                  child: TvButton(
                    'Edit…',
                    key: Key('edit-alias-${a.name}'),
                    kind: TvButtonKind.ghost,
                    dense: true,
                    onPressed: () => unawaited(_edit(context, a)),
                  ),
                ),
              ],
            ),
          ],
      ],
    );
  }
}

class _ChainDialog extends StatefulWidget {
  const _ChainDialog({required this.alias});

  final AliasInfo alias;

  @override
  State<_ChainDialog> createState() => _ChainDialogState();
}

class _ChainDialogState extends State<_ChainDialog> {
  late final _chain = TextEditingController(text: widget.alias.chain.join('\n'));

  @override
  void dispose() {
    _chain.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) => AlertDialog(
    title: Text('Fallback chain for "${widget.alias.name}"'),
    content: SizedBox(
      width: 460,
      child: TextField(
        key: const Key('alias-chain'),
        controller: _chain,
        minLines: 3,
        maxLines: 8,
        decoration: const InputDecoration(
          helperText: 'One provider/model per line, tried in order',
          border: OutlineInputBorder(),
        ),
      ),
    ),
    actions: [
      TextButton(onPressed: () => Navigator.of(context).pop(), child: const Text('Cancel')),
      FilledButton(
        key: const Key('save-alias'),
        onPressed: () =>
            Navigator.of(context).pop(_chain.text.split('\n').map((l) => l.trim()).where((l) => l.isNotEmpty).toList()),
        child: const Text('Save'),
      ),
    ],
  );
}

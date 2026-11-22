import 'dart:async';

import 'package:material_ui/material_ui.dart';

import '../core/core_client.dart';
import '../core/models.dart';
import '../core/settings_draft.dart';
import '../theme.dart';

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
    return Card(
      key: Key('provider-${p.name}'),
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                Expanded(child: Text(p.name, style: Theme.of(context).textTheme.titleSmall)),
                if (p.keyRef != null)
                  TextButton(key: Key('set-key-${p.name}'), onPressed: _setKey, child: const Text('Set API key…')),
                TextButton(
                  key: Key('load-models-${p.name}'),
                  onPressed: _loadModels,
                  child: Text(models == null ? 'Show models' : 'Refresh'),
                ),
              ],
            ),
            Text(p.baseUrl),
            FutureBuilder<bool>(future: _keySet, builder: (context, snap) => _keyStatus(context, p, snap)),
            if (p.keyRef != null)
              Text('Stored in your OS keychain as ${p.keyRef}', style: const TextStyle(color: Tokens.textMuted)),
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
    return Text(text, style: TextStyle(color: missing ? Theme.of(context).colorScheme.error : Tokens.textMuted));
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
        return Text('Could not list models: ${snap.error}', style: const TextStyle(color: Colors.red));
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

/// Adds a provider (name and base URL; its key reference is NAME_KEY).
class AddProviderDialog extends StatefulWidget {
  const AddProviderDialog({super.key});

  @override
  State<AddProviderDialog> createState() => _AddProviderDialogState();
}

class _AddProviderDialogState extends State<AddProviderDialog> {
  final _name = TextEditingController();
  final _url = TextEditingController();

  @override
  void dispose() {
    _name.dispose();
    _url.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) => AlertDialog(
    title: const Text('Add a model provider'),
    content: SizedBox(
      width: 460,
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          TextField(
            key: const Key('provider-name'),
            controller: _name,
            decoration: const InputDecoration(labelText: 'Name', helperText: 'e.g. litellm, ollama'),
          ),
          TextField(
            key: const Key('provider-url'),
            controller: _url,
            decoration: const InputDecoration(labelText: 'Base URL', helperText: 'OpenAI-compatible, ending in /v1'),
          ),
        ],
      ),
    ),
    actions: [
      TextButton(onPressed: () => Navigator.of(context).pop(), child: const Text('Cancel')),
      FilledButton(
        key: const Key('save-provider'),
        onPressed: () => Navigator.of(context).pop((_name.text.trim(), _url.text.trim())),
        child: const Text('Add'),
      ),
    ],
  );
}

/// The key reference a new provider's API key is stored under.
String keyRefFor(String provider) => '${provider.toUpperCase().replaceAll(RegExp('[^A-Z0-9]+'), '_')}_KEY';

/// An alias's fallback chain, editable as one "provider/model" per line.
class AliasCard extends StatelessWidget {
  const AliasCard({super.key, required this.alias, required this.settings, required this.onSave});

  final AliasInfo alias;
  final SettingsInfo settings;
  final Future<String?> Function(SettingsDraft draft) onSave;

  Future<void> _edit(BuildContext context) async {
    final lines = await showDialog<List<String>>(
      context: context,
      builder: (_) => _ChainDialog(alias: alias),
    );
    if (lines == null || lines.isEmpty) return;
    final err = await onSave(SettingsDraft.fromJson(settings.raw)..setAliasChain(alias.name, lines));
    if (err != null && context.mounted) ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(err)));
  }

  @override
  Widget build(BuildContext context) => Card(
    key: Key('alias-${alias.name}'),
    child: Padding(
      padding: const EdgeInsets.all(16),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Expanded(
                child: Text('Fallback chain for "${alias.name}"', style: Theme.of(context).textTheme.titleSmall),
              ),
              TextButton(
                key: Key('edit-alias-${alias.name}'),
                onPressed: () => unawaited(_edit(context)),
                child: const Text('Edit…'),
              ),
            ],
          ),
          for (final (i, m) in alias.chain.indexed)
            ListTile(
              dense: true,
              leading: CircleAvatar(radius: 11, child: Text('${i + 1}')),
              title: Text(m),
            ),
          if (alias.fallbackOn.isNotEmpty) Text('Falls back on: ${alias.fallbackOn.join(', ')}'),
        ],
      ),
    ),
  );
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

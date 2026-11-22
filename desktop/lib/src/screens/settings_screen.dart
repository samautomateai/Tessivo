import 'package:material_ui/material_ui.dart';

import '../core/core_client.dart';
import '../core/models.dart';
import '../theme.dart';
import 'mcp_settings.dart';

/// Settings as the core sees them: providers, aliases with fallback chains,
/// run defaults and token limits, plus each provider's live model list.
/// Read-only here; edit with `tess config` (keys go to the OS keychain).
class SettingsScreen extends StatefulWidget {
  const SettingsScreen({super.key, required this.core});

  final CoreClient core;

  @override
  State<SettingsScreen> createState() => _SettingsScreenState();
}

class _SettingsScreenState extends State<SettingsScreen> {
  late final Future<SettingsInfo> _settings = widget.core.getSettings();
  final Map<String, Future<List<String>>> _models = {};

  void _loadModels(String provider) {
    setState(() {
      _models[provider] = widget.core.listModels(provider: provider).then((m) => m[provider] ?? const []);
    });
  }

  @override
  Widget build(BuildContext context) {
    final text = Theme.of(context).textTheme;
    return FutureBuilder<SettingsInfo>(
      future: _settings,
      builder: (context, snap) {
        if (snap.hasError) return Center(child: Text('Could not load settings: ${snap.error}'));
        if (!snap.hasData) return const Center(child: CircularProgressIndicator());
        final s = snap.data!;
        return ListView(
          padding: const EdgeInsets.all(28),
          children: [
            Text('Models and aliases', style: text.headlineSmall?.copyWith(fontWeight: FontWeight.w600)),
            const SizedBox(height: 4),
            const Text(
              'Notebooks and prompts use aliases, so you can swap models without editing them. '
              "Change these with 'tess config'; API keys stay in your OS keychain.",
              style: TextStyle(color: Tokens.textMuted),
            ),
            const SizedBox(height: 20),
            for (final a in s.aliases) _aliasCard(text, a),
            _defaultsCard(text, s),
            const SizedBox(height: 16),
            Text('Providers', style: text.titleMedium),
            const SizedBox(height: 8),
            for (final p in s.providers) _providerCard(text, p),
            const SizedBox(height: 24),
            McpServersSection(core: widget.core),
          ],
        );
      },
    );
  }

  Widget _aliasCard(TextTheme text, AliasInfo a) => Card(
    key: Key('alias-${a.name}'),
    child: Padding(
      padding: const EdgeInsets.all(16),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text('Fallback chain for "${a.name}"', style: text.titleSmall),
          const SizedBox(height: 8),
          for (final (i, m) in a.chain.indexed)
            ListTile(
              dense: true,
              leading: CircleAvatar(radius: 11, child: Text('${i + 1}')),
              title: Text(m),
            ),
          if (a.fallbackOn.isNotEmpty) Text('Falls back on: ${a.fallbackOn.join(', ')}'),
        ],
      ),
    ),
  );

  Widget _defaultsCard(TextTheme text, SettingsInfo s) {
    String n(int? v) => v == null ? 'not set' : '$v';
    return Card(
      key: const Key('defaults'),
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text('Run defaults and limits', style: text.titleSmall),
            const SizedBox(height: 8),
            Text('Model: ${s.defaultModel ?? 'not set'} · Thinking: ${s.thinkingEffort ?? 'off'}'),
            Text('Max output tokens: ${n(s.maxOutputTokens)}'),
            Text('Tokens per run: ${n(s.maxTokensPerRun)} · per day: ${n(s.maxTokensPerDay)}'),
            const SizedBox(height: 8),
            const Text(
              'Overridden by: notebook › work type › prompt › single run.',
              style: TextStyle(color: Tokens.textMuted),
            ),
          ],
        ),
      ),
    );
  }

  Widget _providerCard(TextTheme text, ProviderInfo p) {
    final models = _models[p.name];
    return Card(
      key: Key('provider-${p.name}'),
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                Expanded(child: Text(p.name, style: text.titleSmall)),
                TextButton(
                  key: Key('load-models-${p.name}'),
                  onPressed: () => _loadModels(p.name),
                  child: Text(models == null ? 'Show models' : 'Refresh'),
                ),
              ],
            ),
            Text(p.baseUrl),
            Text(
              p.keyRef == null ? 'No API key' : 'Key from ${p.keyRef}',
              style: const TextStyle(color: Tokens.textMuted),
            ),
            if (models != null) _modelList(models),
          ],
        ),
      ),
    );
  }

  Widget _modelList(Future<List<String>> models) => FutureBuilder<List<String>>(
    future: models,
    builder: (context, snap) {
      if (snap.hasError) return Text('Could not list models: ${snap.error}', style: const TextStyle(color: Colors.red));
      if (!snap.hasData) return const LinearProgressIndicator();
      if (snap.data!.isEmpty) return const Text('The provider reported no models.');
      return Padding(
        padding: const EdgeInsets.only(top: 8),
        child: Wrap(spacing: 6, runSpacing: 6, children: [for (final m in snap.data!) Chip(label: Text(m))]),
      );
    },
  );
}

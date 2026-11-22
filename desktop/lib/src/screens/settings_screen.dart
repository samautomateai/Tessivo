import 'dart:async';

import 'package:material_ui/material_ui.dart';

import '../core/core_client.dart';
import '../core/models.dart';
import '../core/settings_draft.dart';
import '../theme.dart';
import '../widgets/settings_defaults.dart';
import '../widgets/settings_providers.dart';
import 'mcp_settings.dart';

/// Settings: providers and their API keys, aliases with fallback chains, run
/// defaults and token limits, and MCP servers. Changes are saved to the
/// core's config file; API keys go to the OS keychain and are never shown.
class SettingsScreen extends StatefulWidget {
  const SettingsScreen({super.key, required this.core, this.onChanged});

  final CoreClient core;

  /// Called after a save or a key change (the app re-checks its setup).
  final VoidCallback? onChanged;

  @override
  State<SettingsScreen> createState() => _SettingsScreenState();
}

class _SettingsScreenState extends State<SettingsScreen> {
  late Future<SettingsInfo> _settings = widget.core.getSettings();

  /// Saves a draft and reloads; returns an error message or null.
  Future<String?> _save(SettingsDraft draft) async {
    try {
      final saved = await widget.core.updateSettings(draft.toBody());
      setState(() {
        _settings = Future.value(saved);
      });
      widget.onChanged?.call();
      return null;
    } on CoreException catch (e) {
      return e.message;
    }
  }

  Future<void> _addProvider(SettingsInfo s) async {
    final got = await showDialog<(String, String)>(context: context, builder: (_) => const AddProviderDialog());
    if (got == null || got.$1.isEmpty || got.$2.isEmpty) return;
    final err = await _save(SettingsDraft.fromJson(s.raw)..addProvider(got.$1, got.$2, keyRefFor(got.$1)));
    if (err != null && mounted) ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(err)));
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
              'API keys stay in your OS keychain.',
              style: TextStyle(color: Tokens.textMuted),
            ),
            const SizedBox(height: 20),
            for (final a in s.aliases) AliasCard(alias: a, settings: s, onSave: _save),
            DefaultsEditor(key: const Key('defaults-editor'), settings: s, onSave: _save),
            const SizedBox(height: 16),
            Row(
              children: [
                Expanded(child: Text('Providers', style: text.titleMedium)),
                TextButton.icon(
                  key: const Key('add-provider'),
                  onPressed: () => unawaited(_addProvider(s)),
                  icon: const Icon(Icons.add, size: 16),
                  label: const Text('Add provider'),
                ),
              ],
            ),
            const SizedBox(height: 8),
            for (final p in s.providers)
              ProviderCard(
                key: ValueKey('p-${p.name}'),
                core: widget.core,
                provider: p,
                onKeyChanged: widget.onChanged,
              ),
            const SizedBox(height: 24),
            McpServersSection(core: widget.core),
          ],
        );
      },
    );
  }
}

import 'dart:async';
import 'dart:io';

import 'package:material_ui/material_ui.dart';

import '../../app_state.dart';
import '../../core/core_client.dart';
import '../../core/models.dart';
import '../../core/settings_draft.dart';
import '../../design/colors.dart';
import '../../design/components/buttons.dart';
import '../../design/components/fields.dart';
import '../../design/components/headers.dart';
import '../../design/components/notices.dart';
import '../../design/icons.dart';
import '../../design/sizes.dart';
import '../../design/type.dart';
import '../../widgets/export_menu.dart';
import '../../widgets/settings_agent.dart';
import '../../widgets/settings_appearance.dart';
import '../../widgets/settings_defaults.dart';
import '../../widgets/settings_providers.dart';
import '../../widgets/text_prompt_dialog.dart';
import '../mcp_settings.dart';
import 'integrations_section.dart';
import 'packs_section.dart';
import 'settings_side_bar.dart';

/// One line under each section's title.
String _helper(SettingsSection s) => switch (s) {
  SettingsSection.models =>
    'Notebooks and prompts use aliases, so you can swap models without editing them. '
        'API keys stay in your OS keychain and are never shown.',
  SettingsSection.defaults => 'What runs use unless a notebook or prompt says otherwise, and the token limits.',
  SettingsSection.agent => 'How far an agent run may go, and which tools run without asking.',
  SettingsSection.mcp => 'Servers whose tools and resources notebooks and chats may use.',
  SettingsSection.integrations => 'Trackers, wikis and folders outputs are sent to. Tokens stay in your OS keychain.',
  SettingsSection.packs => 'Templates, prompts and MCP servers your team shares from a folder, a .zip or a git repo.',
  SettingsSection.appearance => 'Theme and zoom. Text also follows the size set in your OS.',
  SettingsSection.desktop => 'How the app behaves on this computer.',
  SettingsSection.diagnostics => 'A zip to attach to a bug report.',
};

/// Settings, one section at a time (picked in the side bar). Changes are
/// saved to the core's config file; API keys go to the OS keychain.
class SettingsPage extends StatefulWidget {
  const SettingsPage({super.key, required this.state, this.onChanged});

  final AppState state;

  /// Called after a save or a key change (the app re-checks its setup).
  final VoidCallback? onChanged;

  @override
  State<SettingsPage> createState() => _SettingsPageState();
}

class _SettingsPageState extends State<SettingsPage> {
  CoreClient get core => widget.state.core;
  late Future<SettingsInfo> _settings = core.getSettings();

  /// Saves a draft and reloads; returns an error message or null.
  Future<String?> _save(SettingsDraft draft) async {
    try {
      final saved = await core.updateSettings(draft.toBody());
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
    final got = await showDialog<NewProvider>(context: context, builder: (_) => const AddProviderDialog());
    if (got == null || got.name.isEmpty || (got.url.isEmpty && got.kind == 'openai')) return;
    final draft = SettingsDraft.fromJson(s.raw)
      ..addProvider(got.name, got.url, got.needsKey ? keyRefFor(got.name) : '', kind: got.kind);
    final err = await _save(draft);
    if (err != null && mounted) showTvToast(context, err, kind: TvNoticeKind.error);
  }

  Future<void> _saveDiagnostics() async {
    final path = await askText(
      context,
      title: 'Save diagnostics',
      label: 'File path',
      initial: downloadsPath('tessivo-diagnostics.zip'),
      maxLength: 4096,
    );
    if (path == null || path.trim().isEmpty || !mounted) return;
    try {
      File(path.trim()).writeAsBytesSync(await core.diagnostics(), flush: true);
      if (mounted) showTvToast(context, 'Saved ${path.trim()}', kind: TvNoticeKind.success);
    } on Object catch (e) {
      if (mounted) {
        showTvToast(context, e is CoreException ? e.message : 'Could not save: $e', kind: TvNoticeKind.error);
      }
    }
  }

  List<Widget> _section(SettingsSection section, SettingsInfo s) {
    final c = TvColors.of(context);
    final note = TvType.body.copyWith(color: c.textSecondary);
    return switch (section) {
      SettingsSection.models => [
        const _Heading('Aliases'),
        AliasesTable(settings: s, onSave: _save, onTest: core.testModel),
        const SizedBox(height: TvSpace.s32),
        _Heading(
          'Providers',
          action: TvButton(
            'Add provider',
            key: const Key('add-provider'),
            icon: TvIcon.add,
            dense: true,
            onPressed: () => unawaited(_addProvider(s)),
          ),
        ),
        for (final p in s.providers)
          ProviderCard(key: ValueKey('p-${p.name}'), core: core, provider: p, onKeyChanged: widget.onChanged),
      ],
      SettingsSection.defaults => [DefaultsEditor(key: const Key('defaults-editor'), settings: s, onSave: _save)],
      SettingsSection.agent => [AgentEditor(key: const Key('agent-editor'), settings: s, onSave: _save)],
      SettingsSection.mcp => [
        McpServersSection(core: core, onChanged: () => unawaited(widget.state.refreshMcpServers())),
      ],
      SettingsSection.integrations => [IntegrationsSection(state: widget.state)],
      SettingsSection.packs => [PacksSection(state: widget.state)],
      SettingsSection.appearance => [AppearanceSettings(state: widget.state)],
      SettingsSection.desktop => [
        TvToggle(
          key: const Key('close-to-tray'),
          label: 'Keep running in the tray when the window is closed',
          value: SettingsDraft.fromJson(s.raw).closeToTray,
          onChanged: (on) => unawaited(_save(SettingsDraft.fromJson(s.raw)..closeToTray = on)),
        ),
        Padding(
          padding: const EdgeInsets.only(left: 44),
          child: Text('Runs and schedules go on; Quit in the tray menu ends Tessivo.', style: note),
        ),
      ],
      SettingsSection.diagnostics => [
        Text(
          'Versions, settings (key names only), whether keys are set, counts and the end of the core log. '
          'No keys, notebook names or content.',
          style: note,
        ),
        const SizedBox(height: TvSpace.s16),
        Align(
          alignment: Alignment.centerLeft,
          child: TvButton(
            'Save diagnostics…',
            key: const Key('save-diagnostics'),
            icon: TvIcon.export,
            onPressed: () => unawaited(_saveDiagnostics()),
          ),
        ),
      ],
    };
  }

  @override
  Widget build(BuildContext context) {
    final c = TvColors.of(context);
    final section = widget.state.settingsSection;
    return ColoredBox(
      color: c.background,
      child: FutureBuilder<SettingsInfo>(
        future: _settings,
        builder: (context, snap) {
          if (snap.hasError) {
            return Center(
              child: Text('Could not load settings: ${snap.error}', style: TvType.body.copyWith(color: c.error)),
            );
          }
          if (!snap.hasData) return const Center(child: CircularProgressIndicator());
          return Column(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              TvPageHeader(title: settingsLabel(section)),
              Padding(
                padding: const EdgeInsets.fromLTRB(TvSpace.s24, TvSpace.s4, TvSpace.s24, 0),
                child: Text(_helper(section), style: TvType.helper.copyWith(color: c.textHelper)),
              ),
              Expanded(
                child: ListView(
                  key: const Key('settings-list'),
                  padding: const EdgeInsets.fromLTRB(TvSpace.s24, TvSpace.s16, TvSpace.s24, TvSpace.s32),
                  children: [
                    for (final w in _section(section, snap.data!))
                      Align(
                        alignment: Alignment.topLeft,
                        child: ConstrainedBox(
                          constraints: const BoxConstraints(maxWidth: TvSize.readingWidth + 200),
                          child: w,
                        ),
                      ),
                  ],
                ),
              ),
            ],
          );
        },
      ),
    );
  }
}

class _Heading extends StatelessWidget {
  const _Heading(this.text, {this.action});

  final String text;
  final Widget? action;

  @override
  Widget build(BuildContext context) => Padding(
    padding: const EdgeInsets.only(bottom: TvSpace.s8),
    child: Row(
      children: [
        Expanded(
          child: Text(text, style: TvType.heading01.copyWith(color: TvColors.of(context).textPrimary)),
        ),
        ?action,
      ],
    ),
  );
}

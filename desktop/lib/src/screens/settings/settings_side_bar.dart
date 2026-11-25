import 'package:material_ui/material_ui.dart';

import '../../app_state.dart';
import '../../design/components/headers.dart';
import '../../design/components/rows.dart';

/// Each section's name in the side bar and as the page title.
String settingsLabel(SettingsSection s) => switch (s) {
  SettingsSection.models => 'Models',
  SettingsSection.defaults => 'Defaults and limits',
  SettingsSection.agent => 'Agent',
  SettingsSection.mcp => 'MCP servers',
  SettingsSection.packs => 'Packs',
  SettingsSection.appearance => 'Appearance',
  SettingsSection.desktop => 'Desktop',
  SettingsSection.diagnostics => 'Diagnostics',
};

/// The Settings side bar: its sections, as VS Code's settings outline.
class SettingsSideBar extends StatelessWidget {
  const SettingsSideBar({super.key, required this.state});

  final AppState state;

  @override
  Widget build(BuildContext context) => Column(
    crossAxisAlignment: CrossAxisAlignment.stretch,
    children: [
      const TvPanelHeader('Settings'),
      Expanded(
        child: ListView(
          children: [
            for (final s in SettingsSection.values)
              TvTreeRow(
                key: Key('settings-${s.name}'),
                leading: const SizedBox.shrink(),
                label: settingsLabel(s),
                selected: state.settingsSection == s,
                onTap: () => state.openSettings(s),
              ),
          ],
        ),
      ),
    ],
  );
}

import 'package:material_ui/material_ui.dart';

import '../theme.dart';

/// Settings: model aliases, fallback chain and run defaults. Read-only until
/// the SettingsService is wired to the core.
class SettingsScreen extends StatefulWidget {
  const SettingsScreen({super.key});

  @override
  State<SettingsScreen> createState() => _SettingsScreenState();
}

const thinkingLevels = ['off', 'low', 'medium', 'high'];

class _SettingsScreenState extends State<SettingsScreen> {
  String _thinking = 'medium';

  @override
  Widget build(BuildContext context) {
    final text = Theme.of(context).textTheme;
    return ListView(
      padding: const EdgeInsets.all(28),
      children: [
        Text('Models and aliases', style: text.headlineSmall?.copyWith(fontWeight: FontWeight.w600)),
        const SizedBox(height: 4),
        const Text(
          'Notebooks and prompts use aliases, so you can swap models without editing them.',
          style: TextStyle(color: Tokens.textMuted),
        ),
        const SizedBox(height: 20),
        Card(
          child: Padding(
            padding: const EdgeInsets.all(16),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text('Fallback chain for "smart"', style: text.titleSmall),
                const SizedBox(height: 8),
                for (final (i, m) in const ['primary model', 'fallback model', 'local model'].indexed)
                  ListTile(dense: true, leading: CircleAvatar(radius: 11, child: Text('${i + 1}')), title: Text(m)),
                const Text('Falls back on timeout, rate limit, 5xx or context overflow.'),
              ],
            ),
          ),
        ),
        const SizedBox(height: 16),
        Card(
          child: Padding(
            padding: const EdgeInsets.all(16),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text('Thinking effort', style: text.titleSmall),
                const SizedBox(height: 8),
                SegmentedButton<String>(
                  key: const Key('thinking'),
                  segments: [for (final l in thinkingLevels) ButtonSegment(value: l, label: Text(l))],
                  selected: {_thinking},
                  onSelectionChanged: (s) => setState(() => _thinking = s.first),
                ),
                const SizedBox(height: 12),
                const Text(
                  'Overridden by: notebook › work type › prompt › single run. API keys stay in your OS keychain.',
                  style: TextStyle(color: Tokens.textMuted),
                ),
              ],
            ),
          ),
        ),
      ],
    );
  }
}

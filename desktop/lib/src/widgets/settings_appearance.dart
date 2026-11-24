import 'dart:async';

import 'package:material_ui/material_ui.dart';

import '../app_state.dart';
import '../design/colors.dart';
import '../design/components/buttons.dart';
import '../design/icons.dart';
import '../design/scale_probe.dart';
import '../design/sizes.dart';
import '../design/theme.dart';
import '../design/type.dart';

/// Settings → Appearance: theme, zoom, and what scale the app sees from the OS.
class AppearanceSettings extends StatelessWidget {
  const AppearanceSettings({super.key, required this.state});

  final AppState state;

  static const _themes = [
    (TvThemeMode.system, 'System'),
    (TvThemeMode.light, 'Light'),
    (TvThemeMode.dark, 'Dark'),
    (TvThemeMode.highContrast, 'High contrast'),
  ];

  @override
  Widget build(BuildContext context) {
    final c = TvColors.of(context);
    final label = TvType.helper.copyWith(color: c.textSecondary);
    return Card(
      key: const Key('appearance'),
      child: Padding(
        padding: const EdgeInsets.all(TvSpace.s16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text('Appearance', style: TvType.heading01.copyWith(color: c.textPrimary)),
            const SizedBox(height: TvSpace.s12),
            Wrap(
              spacing: TvSpace.s24,
              runSpacing: TvSpace.s12,
              crossAxisAlignment: WrapCrossAlignment.center,
              children: [
                Row(
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    Text('Theme', style: label),
                    const SizedBox(width: TvSpace.s8),
                    DropdownButton<TvThemeMode>(
                      key: const Key('appearance-theme'),
                      value: state.themeMode,
                      items: [for (final (m, l) in _themes) DropdownMenuItem(value: m, child: Text(l))],
                      onChanged: (m) => unawaited(state.setThemeMode(m ?? TvThemeMode.system)),
                    ),
                  ],
                ),
                Row(
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    Text('Zoom', style: label),
                    const SizedBox(width: TvSpace.s8),
                    TvIconButton(TvIcon.subtract, tooltip: 'Zoom out (Ctrl −)', onPressed: () => state.stepZoom(-1)),
                    SizedBox(
                      width: 48,
                      child: Text(
                        '${(state.zoom * 100).round()} %',
                        key: const Key('appearance-zoom'),
                        textAlign: TextAlign.center,
                        style: TvType.body.copyWith(color: c.textPrimary),
                      ),
                    ),
                    TvIconButton(TvIcon.add, tooltip: 'Zoom in (Ctrl =)', onPressed: () => state.stepZoom(1)),
                    const SizedBox(width: TvSpace.s4),
                    TvButton('Reset', dense: true, kind: TvButtonKind.ghost, onPressed: () => state.stepZoom(0)),
                  ],
                ),
              ],
            ),
            const SizedBox(height: TvSpace.s12),
            Text(
              'The app follows Windows display scaling and text size; zoom adds to them. '
              'Seen from the system: ${describeScale(MediaQuery.of(context))}.',
              key: const Key('appearance-scale'),
              style: TvType.helper.copyWith(color: c.textHelper),
            ),
          ],
        ),
      ),
    );
  }
}

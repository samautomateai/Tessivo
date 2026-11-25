import 'dart:async';

import 'package:material_ui/material_ui.dart';

import '../../app_state.dart';
import '../../core/core_client.dart';
import '../../core/models.dart';
import '../../design/colors.dart';
import '../../design/components/buttons.dart';
import '../../design/components/notices.dart';
import '../../design/components/table.dart';
import '../../design/components/tags.dart';
import '../../design/icons.dart';
import '../../design/sizes.dart';
import '../../design/type.dart';
import '../../widgets/text_prompt_dialog.dart';

/// Settings → Profiles: save the current models and limits as a profile, and
/// switch between profiles in one step (SET-10).
class ProfilesSection extends StatefulWidget {
  const ProfilesSection({super.key, required this.state, this.onChanged});

  final AppState state;

  /// Called after a switch, so the settings shown are reloaded.
  final VoidCallback? onChanged;

  @override
  State<ProfilesSection> createState() => _ProfilesSectionState();
}

class _ProfilesSectionState extends State<ProfilesSection> {
  CoreClient get core => widget.state.core;
  ProfilesInfo? _info;
  bool _busy = false;

  @override
  void initState() {
    super.initState();
    unawaited(_run(core.profiles));
  }

  /// Runs a call, keeps the list, refreshes the app after a change and shows [done] (or a refusal).
  Future<void> _run(Future<ProfilesInfo> Function() call, {String? done}) async {
    setState(() => _busy = true);
    try {
      final info = await call();
      if (!mounted) return;
      setState(() => _info = info);
      if (done != null) {
        await widget.state.refreshProfile();
        widget.onChanged?.call();
        if (mounted) showTvToast(context, done, kind: TvNoticeKind.success);
      }
    } on CoreException catch (e) {
      if (mounted) showTvToast(context, e.message, kind: TvNoticeKind.error);
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  Future<void> _save() async {
    final name = await askText(context, title: 'Save as profile', label: 'Profile name (e.g. work)', maxLength: 32);
    if (name == null || name.trim().isEmpty) return;
    await _run(() => core.saveProfile(name.trim()), done: 'Saved profile ${name.trim()}');
  }

  @override
  Widget build(BuildContext context) {
    final c = TvColors.of(context);
    final info = _info;
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        Row(
          children: [
            Expanded(
              child: Text(
                'A profile keeps providers, aliases, run defaults and token limits. Keys stay in your OS keychain; '
                'switching keeps your edits in the profile you leave.',
                style: TvType.body.copyWith(color: c.textSecondary),
              ),
            ),
            const SizedBox(width: TvSpace.s8),
            TvButton(
              'Save current as…',
              key: const Key('profile-save'),
              icon: TvIcon.add,
              dense: true,
              onPressed: _busy ? null : () => unawaited(_save()),
            ),
          ],
        ),
        const SizedBox(height: TvSpace.s16),
        if (info == null)
          const Center(child: CircularProgressIndicator())
        else if (info.profiles.isEmpty)
          Text(
            'No profiles yet. Save the current settings to start one.',
            key: const Key('no-profiles'),
            style: TvType.body.copyWith(color: c.textSecondary),
          )
        else
          TvDataTable(
            columns: const ['Profile', 'Providers', 'Default model', ''],
            flex: const [3, 2, 3, 3],
            rows: [for (final p in info.profiles) _row(p, active: p.name == info.active)],
          ),
      ],
    );
  }

  List<Widget> _row(ProfileSummary p, {required bool active}) => [
    Wrap(
      spacing: TvSpace.s8,
      crossAxisAlignment: WrapCrossAlignment.center,
      children: [
        Text(p.name),
        if (active) const TvTag('Active', tone: TvTagTone.green),
      ],
    ),
    Text('${p.providers}'),
    Text(p.defaultModel.isEmpty ? '—' : p.defaultModel),
    Wrap(
      alignment: WrapAlignment.end,
      spacing: TvSpace.s4,
      children: [
        if (!active)
          TvButton(
            'Use',
            key: Key('profile-use-${p.name}'),
            dense: true,
            onPressed: _busy ? null : () => unawaited(_run(() => core.useProfile(p.name), done: 'Using ${p.name}')),
          ),
        TvIconButton(
          TvIcon.close,
          key: Key('profile-delete-${p.name}'),
          tooltip: 'Delete ${p.name} (the settings stay)',
          onPressed: _busy
              ? null
              : () => unawaited(_run(() => core.deleteProfile(p.name), done: 'Deleted profile ${p.name}')),
        ),
      ],
    ),
  ];
}

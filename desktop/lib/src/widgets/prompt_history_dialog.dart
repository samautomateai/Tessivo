import 'package:material_ui/material_ui.dart';

import '../core/models.dart';
import '../design/colors.dart';
import '../design/components/buttons.dart';
import '../design/components/modal.dart';
import '../design/sizes.dart';
import '../design/type.dart';

/// Shows a saved prompt's versions, newest first, and returns the one to restore (PR-4).
/// The first version is the current one and has no Restore.
Future<PromptVersion?> showPromptHistory(BuildContext context, String name, List<PromptVersion> versions) =>
    showTvModal<PromptVersion?>(
      context,
      title: 'History of $name',
      width: 640,
      body: _VersionList(versions: versions),
      actions: const [TvModalAction<PromptVersion?>('Close', value: null)],
    );

class _VersionList extends StatelessWidget {
  const _VersionList({required this.versions});

  final List<PromptVersion> versions;

  @override
  Widget build(BuildContext context) {
    final c = TvColors.of(context);
    if (versions.isEmpty) return Text('No versions yet.', style: TvType.body.copyWith(color: c.textSecondary));
    return ListView.separated(
      key: const Key('prompt-versions'),
      shrinkWrap: true,
      itemCount: versions.length,
      separatorBuilder: (_, _) => Divider(height: TvSpace.s16, color: c.borderSubtle),
      itemBuilder: (context, i) {
        final v = versions[i];
        final when = v.savedAt.isEmpty ? '' : ' · ${_date(v.savedAt)}';
        return Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                Expanded(
                  child: Text(
                    'Version ${v.version}${i == 0 ? ' (current)' : ''}$when',
                    style: TvType.label.copyWith(color: c.textSecondary),
                  ),
                ),
                if (i > 0)
                  TvButton(
                    'Restore',
                    key: Key('prompt-restore-${v.version}'),
                    dense: true,
                    onPressed: () => Navigator.of(context).pop(v),
                  ),
              ],
            ),
            const SizedBox(height: TvSpace.s4),
            Text(
              v.text,
              maxLines: 6,
              overflow: TextOverflow.ellipsis,
              style: TvType.body.copyWith(color: c.textPrimary),
            ),
          ],
        );
      },
    );
  }
}

/// "2026-11-25T10:00:00Z" → "2026-11-25 10:00".
String _date(String iso) {
  final t = DateTime.tryParse(iso)?.toLocal();
  if (t == null) return iso;
  String two(int n) => n.toString().padLeft(2, '0');
  return '${t.year}-${two(t.month)}-${two(t.day)} ${two(t.hour)}:${two(t.minute)}';
}

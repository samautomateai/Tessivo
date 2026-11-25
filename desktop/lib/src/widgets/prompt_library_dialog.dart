import 'package:material_ui/material_ui.dart';

import '../core/core_client.dart';
import '../design/colors.dart';
import '../design/type.dart';

/// Picks a starter prompt: the notebook's work type first, the rest below.
class PromptLibraryDialog extends StatelessWidget {
  const PromptLibraryDialog({super.key, required this.prompts, required this.workType});

  final Future<List<LibraryPrompt>> prompts;

  /// The open notebook's work type (analyze, summarize, …).
  final String workType;

  Widget _tile(BuildContext context, LibraryPrompt p) => ListTile(
    key: Key('library-${p.id}'),
    title: Text(p.pack.isEmpty ? p.name : '${p.name}  ·  from ${p.pack}${p.mark.isEmpty ? '' : ' (${p.mark})'}'),
    subtitle: Text(p.text, maxLines: 2, overflow: TextOverflow.ellipsis),
    trailing: Text(p.workType, style: TvType.helper.copyWith(color: TvColors.of(context).textHelper)),
    onTap: () => Navigator.of(context).pop(p),
  );

  @override
  Widget build(BuildContext context) => AlertDialog(
    title: const Text('Prompt library'),
    content: SizedBox(
      width: 600,
      height: 420,
      child: FutureBuilder<List<LibraryPrompt>>(
        future: prompts,
        builder: (context, snap) {
          if (snap.hasError) return Text('Could not load the library: ${snap.error}');
          if (!snap.hasData) return const Center(child: CircularProgressIndicator());
          final all = snap.data!;
          final mine = [
            for (final p in all)
              if (p.workType == workType) p,
          ];
          final rest = [
            for (final p in all)
              if (p.workType != workType) p,
          ];
          return ListView(
            children: [
              for (final p in mine) _tile(context, p),
              if (mine.isNotEmpty && rest.isNotEmpty) const Divider(),
              for (final p in rest) _tile(context, p),
            ],
          );
        },
      ),
    ),
    actions: [TextButton(onPressed: () => Navigator.of(context).pop(), child: const Text('Cancel'))],
  );
}

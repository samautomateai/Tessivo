import 'package:material_ui/material_ui.dart';

/// Asks for a repository folder and the branch, tag or commit to compare with.
/// Pops `(repo, base)` or null.
class GitDiffDialog extends StatefulWidget {
  const GitDiffDialog({super.key});

  @override
  State<GitDiffDialog> createState() => _GitDiffDialogState();
}

class _GitDiffDialogState extends State<GitDiffDialog> {
  final _repo = TextEditingController();
  final _base = TextEditingController(text: 'main');

  @override
  void dispose() {
    _repo.dispose();
    _base.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) => AlertDialog(
    title: const Text('Add git changes'),
    content: SizedBox(
      width: 460,
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          const Text('The diff against the base is read with git on every run.'),
          TextField(
            key: const Key('git-repo'),
            controller: _repo,
            maxLength: 4096,
            decoration: const InputDecoration(labelText: 'Repository folder (absolute path)'),
          ),
          TextField(
            key: const Key('git-base'),
            controller: _base,
            maxLength: 200,
            decoration: const InputDecoration(labelText: 'Compare with (branch, tag or commit)'),
          ),
        ],
      ),
    ),
    actions: [
      TextButton(onPressed: () => Navigator.of(context).pop(), child: const Text('Cancel')),
      FilledButton(
        key: const Key('git-save'),
        onPressed: () => Navigator.of(context).pop((_repo.text.trim(), _base.text.trim())),
        child: const Text('Add'),
      ),
    ],
  );
}

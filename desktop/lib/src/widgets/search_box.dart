import 'dart:async';

import 'package:material_ui/material_ui.dart';

import '../app_state.dart';
import '../core/core_client.dart';
import '../core/models.dart';
import '../theme.dart';

/// The sidebar search: Enter shows matches across notebooks, outputs,
/// prompts and research notes; choosing one opens it.
class SearchBox extends StatefulWidget {
  const SearchBox({super.key, required this.state});

  final AppState state;

  @override
  State<SearchBox> createState() => _SearchBoxState();
}

class _SearchBoxState extends State<SearchBox> {
  final _text = TextEditingController();

  @override
  void dispose() {
    _text.dispose();
    super.dispose();
  }

  Future<void> _search(String q) => runSearch(context, widget.state, q);

  @override
  Widget build(BuildContext context) {
    return Semantics(
      label: 'Search notebooks, outputs, prompts and notes',
      child: TextField(
        key: const Key('sidebar-search'),
        controller: _text,
        textInputAction: TextInputAction.search,
        onSubmitted: (q) => unawaited(_search(q)),
        style: const TextStyle(color: Colors.white, fontSize: 13),
        decoration: const InputDecoration(
          isDense: true,
          hintText: 'Search',
          hintStyle: TextStyle(color: Tokens.sidebarText),
          prefixIcon: Icon(Icons.search, size: 18, color: Tokens.sidebarText),
          border: OutlineInputBorder(borderSide: BorderSide(color: Tokens.sidebarSelected)),
          enabledBorder: OutlineInputBorder(borderSide: BorderSide(color: Tokens.sidebarSelected)),
        ),
      ),
    );
  }
}

class _Results extends StatelessWidget {
  const _Results({required this.query, required this.hits});

  final String query;
  final List<SearchHit> hits;

  static const _icons = {
    'notebook': Icons.book_outlined,
    'output': Icons.description_outlined,
    'prompt': Icons.bookmark_outline,
    'research': Icons.travel_explore,
    'note': Icons.sticky_note_2_outlined,
  };

  @override
  Widget build(BuildContext context) => AlertDialog(
    key: const Key('search-results'),
    title: Text('Results for "$query"'),
    content: SizedBox(
      width: 640,
      height: 420,
      child: hits.isEmpty
          ? const Center(child: Text('Nothing found.'))
          : ListView(
              children: [
                for (final (i, h) in hits.indexed)
                  ListTile(
                    key: Key('hit-$i'),
                    leading: Icon(_icons[h.kind] ?? Icons.search),
                    title: Text(h.title),
                    subtitle: Text('${h.kind} · ${h.snippet}', maxLines: 2, overflow: TextOverflow.ellipsis),
                    onTap: () => Navigator.of(context).pop(h),
                  ),
              ],
            ),
    ),
    actions: [TextButton(onPressed: () => Navigator.of(context).pop(), child: const Text('Close'))],
  );
}

/// Searches for [q], shows the matches and opens the one chosen.
Future<void> runSearch(BuildContext context, AppState state, String q) async {
  if (q.trim().isEmpty) return;
  final messenger = ScaffoldMessenger.of(context);
  final List<SearchHit> hits;
  try {
    hits = await state.core.search(q.trim());
  } on CoreException catch (e) {
    messenger.showSnackBar(SnackBar(content: Text(e.message)));
    return;
  }
  if (!context.mounted) return;
  final chosen = await showDialog<SearchHit>(
    context: context,
    builder: (ctx) => _Results(query: q.trim(), hits: hits),
  );
  if (chosen != null) await state.openHit(chosen);
}

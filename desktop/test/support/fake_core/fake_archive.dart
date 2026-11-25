import 'dart:io';

import 'package:tessivo_desktop/src/core/core_client.dart';
import 'package:tessivo_desktop/src/core/models.dart';

import 'fake_research.dart';

/// Archives in the fake core: export writes a tiny zip-looking file, import
/// adds a copy named "(imported)" with one source that needs relinking.
mixin DemoArchive on DemoResearch {
  /// The notebook list of the fake core.
  List<Notebook> get archiveNotebooks;

  /// Every export: (notebook ID, outputs included).
  final exports = <(String, bool)>[];

  int _imports = 0;

  Future<void> exportNotebook(String notebookId, String toPath, {bool outputs = false}) async {
    if (!archiveNotebooks.any((n) => n.id == notebookId)) throw CoreException(404, 'notebook $notebookId not found');
    exports.add((notebookId, outputs));
    File(toPath).writeAsStringSync('PK fake archive of $notebookId');
  }

  Future<Notebook> importNotebook(String fromPath) async {
    if (!File(fromPath).readAsStringSync().startsWith('PK')) throw const CoreException(400, 'not a Tessivo archive');
    final from = archiveNotebooks.first;
    final nb = Notebook(
      id: 'imported-${++_imports}',
      name: '${from.name} (imported)',
      workType: from.workType,
      context: const [
        ContextItem(
          id: 'r1',
          label: 'payments-service',
          kind: 'Folder',
          enabled: false,
          estimatedTokens: 0,
          needsRelink: true,
        ),
        ContextItem(id: 'r2', label: 'notes.md', kind: 'MD', enabled: true, estimatedTokens: 1000),
      ],
    );
    archiveNotebooks.add(nb);
    return nb;
  }

  Future<Notebook> relinkContext(String notebookId, String contextId, String path) async {
    if (!path.startsWith('/') && !RegExp(r'^[A-Za-z]:\\').hasMatch(path)) {
      throw const CoreException(400, 'path must be absolute');
    }
    final i = archiveNotebooks.indexWhere((n) => n.id == notebookId);
    final nb = archiveNotebooks[i];
    return archiveNotebooks[i] = nb.copyWith(
      context: [
        for (final c in nb.context)
          c.id == contextId
              ? ContextItem(id: c.id, label: c.label, kind: c.kind, enabled: true, estimatedTokens: c.estimatedTokens)
              : c,
      ],
    );
  }

  Future<void> exportResearch(String id, String toPath) async => File(toPath).writeAsStringSync('PK fake research');

  Future<ResearchNotebook> importResearch(String fromPath) async {
    if (!File(fromPath).readAsStringSync().startsWith('PK')) throw const CoreException(400, 'not a Tessivo archive');
    return createResearch('Imported research');
  }
}

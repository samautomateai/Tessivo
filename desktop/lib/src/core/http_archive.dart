part of 'http_core_client.dart';

/// Largest archive the app saves or sends (the core's import limit).
const maxArchiveBytes = 512 * 1024 * 1024;

/// Notebook archives (NB-4): the core builds and reads them; the app streams
/// them to and from files, never holding a whole archive in memory.
mixin _HttpArchive on _HttpChat {
  Future<HttpClientRequest> _open(String method, String path, {Map<String, String>? query});

  String _nbPath(String id) => '/v1/notebooks/${Uri.encodeComponent(id)}';

  Future<void> exportNotebook(String notebookId, String toPath, {bool outputs = false}) =>
      _download('${_nbPath(notebookId)}/archive', toPath, query: outputs ? const {'outputs': '1'} : null);

  Future<Notebook> importNotebook(String fromPath) async =>
      notebookFromJson(await _upload('/v1/notebooks/import', fromPath));

  Future<Notebook> relinkContext(String notebookId, String contextId, String path) async => notebookFromJson(
    await _json(
      'POST',
      '${_nbPath(notebookId)}/context/${Uri.encodeComponent(contextId)}/relink',
      body: {'path': path},
    ),
  );

  Future<void> exportResearch(String id, String toPath) =>
      _download('/v1/research/${Uri.encodeComponent(id)}/archive', toPath);

  Future<ResearchNotebook> importResearch(String fromPath) async =>
      ResearchNotebook.fromJson(await _upload('/v1/research/import', fromPath));

  /// Streams the response into `<toPath>.part`, then renames it into place.
  Future<void> _download(String path, String toPath, {Map<String, String>? query}) async {
    final resp = await (await _open('GET', path, query: query)).close();
    if (resp.statusCode >= 400) {
      final text = await HttpCoreClient._readLimited(resp);
      final decoded = text.isEmpty ? null : jsonDecode(text);
      throw CoreException(
        resp.statusCode,
        (decoded is Map<String, Object?> ? decoded['error'] as String? : null) ?? 'export failed',
      );
    }
    final part = File('$toPath.part');
    final sink = part.openWrite();
    var written = 0;
    try {
      await for (final chunk in resp) {
        written += chunk.length;
        if (written > maxArchiveBytes) throw const CoreException(502, 'archive too large');
        sink.add(chunk);
      }
      await sink.close();
      await part.rename(toPath);
    } on Object {
      await sink.close();
      if (part.existsSync()) part.deleteSync();
      rethrow;
    }
  }

  /// Sends a file as an application/zip body and returns the JSON reply.
  Future<Map<String, Object?>> _upload(String path, String fromPath) async {
    final file = File(fromPath);
    final size = await file.length();
    if (size > maxArchiveBytes) throw const CoreException(400, 'archive is larger than ${maxArchiveBytes >> 20} MB');
    final req = await _open('POST', path);
    req.headers.contentType = ContentType('application', 'zip');
    req.contentLength = size;
    await req.addStream(file.openRead());
    final resp = await req.close();
    final text = await HttpCoreClient._readLimited(resp);
    final decoded = text.isEmpty ? <String, Object?>{} : jsonDecode(text);
    final map = decoded is Map<String, Object?> ? decoded : <String, Object?>{};
    if (resp.statusCode >= 400) throw CoreException(resp.statusCode, (map['error'] as String?) ?? 'import failed');
    return map;
  }
}

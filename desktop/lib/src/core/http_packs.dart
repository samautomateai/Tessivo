part of 'http_core_client.dart';

/// Team pack endpoints of the local API (docs/api.md, "Packs").
mixin _HttpPacks on _HttpChat {
  Future<List<PackInfo>> listPacks() async => packsFromJson(await _json('GET', '/v1/packs'));

  Future<PackInfo> addPack(String source, {String branch = ''}) async => PackInfo.fromJson(
    await _json('POST', '/v1/packs', body: {'source': source, if (branch.isNotEmpty) 'branch': branch}),
  );

  Future<List<PackInfo>> refreshPacks() async => packsFromJson(await _json('POST', '/v1/packs/refresh'));

  Future<void> removePack(String id) async {
    await _json('DELETE', '/v1/packs/${Uri.encodeComponent(id)}');
  }

  /// The name `<pack>/<server>` goes URL-encoded, so its '/' stays in one segment.
  Future<void> trustMcpServer(String name, {required bool trust}) async {
    await _json(trust ? 'POST' : 'DELETE', '/v1/mcp/servers/${Uri.encodeComponent(name)}/trust');
  }
}

import 'dart:convert';
import 'dart:io';

import 'package:flutter_test/flutter_test.dart';
import 'package:tessivo_desktop/src/core/http_core_client.dart';

import 'support/real_http.dart';

void main() {
  test('archives stream to and from files over HTTP', () async {
    final server = await HttpServer.bind(InternetAddress.loopbackIPv4, 0);
    final seen = <String>[];
    server.listen((req) async {
      final body = await req.fold<List<int>>([], (a, b) => a..addAll(b));
      seen.add('${req.method} ${req.uri} ${req.headers.contentType?.mimeType} ${body.length}');
      final res = req.response;
      if (req.uri.path == '/v1/notebooks/ops/archive') {
        res.headers.contentType = ContentType('application', 'zip');
        res.add(utf8.encode('PK archive bytes'));
      } else if (req.uri.path == '/v1/notebooks/gone/archive') {
        res.statusCode = 404;
        res.write(jsonEncode({'error': 'notebook gone not found'}));
      } else if (req.uri.path == '/v1/notebooks/import') {
        res.statusCode = 201;
        res.write(
          jsonEncode({'id': 'ops-2', 'name': 'Ops (imported)', 'workType': 'summarize', 'context': <Object?>[]}),
        );
      } else {
        res.statusCode = 201;
        res.write(jsonEncode({'id': 'r-2', 'name': 'Vendors (imported)'}));
      }
      await res.close();
    });
    addTearDown(() => server.close(force: true));
    final client = HttpCoreClient(
      Endpoint(network: 'tcp', address: '127.0.0.1:${server.port}', token: 't'),
      client: realHttpClient(),
    );
    final dir = Directory.systemTemp.createTempSync('tess-archive');
    addTearDown(() => dir.deleteSync(recursive: true));
    final path = '${dir.path}${Platform.pathSeparator}ops.tessivo.zip';

    await client.exportNotebook('ops', path, outputs: true);
    expect(File(path).readAsStringSync(), 'PK archive bytes');
    expect(File('$path.part').existsSync(), isFalse);
    expect(seen.last, startsWith('GET /v1/notebooks/ops/archive?outputs=1'));

    final missing = '${dir.path}${Platform.pathSeparator}gone.zip';
    await expectLater(
      client.exportNotebook('gone', missing),
      throwsA(isA<CoreException>().having((e) => e.message, 'message', 'notebook gone not found')),
    );
    expect(File(missing).existsSync() || File('$missing.part').existsSync(), isFalse);

    final nb = await client.importNotebook(path);
    expect(nb.name, 'Ops (imported)');
    expect(seen.last, 'POST /v1/notebooks/import application/zip 16');

    final r = await client.importResearch(path);
    expect(r.id, 'r-2');
  });
}

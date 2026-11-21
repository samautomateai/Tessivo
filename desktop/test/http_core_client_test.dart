import 'dart:convert';
import 'dart:io';

import 'package:flutter_test/flutter_test.dart';
import 'package:tessivo_desktop/main.dart';
import 'package:tessivo_desktop/src/core/core_client.dart';
import 'package:tessivo_desktop/src/core/http_core_client.dart';
import 'package:tessivo_desktop/src/core/models.dart';
import 'package:tessivo_desktop/src/core/supervisor.dart';

import 'support/real_http.dart';

const token = 'secret-token';

const notebookJson = {
  'id': 'board',
  'name': 'Board',
  'workType': 'summarize',
  'context': [
    {'id': 'c1', 'label': 'minutes.md', 'enabled': true, 'inPlace': false, 'estimatedTokens': 1200},
    {'id': 'c2', 'label': 'repo', 'enabled': false, 'inPlace': true, 'estimatedTokens': 9000},
  ],
};

/// A tiny fake of the Go core's API.
Future<HttpServer> fakeCore(HttpServer server) async {
  server.listen((req) async {
    final res = req.response;
    if (req.headers.value(HttpHeaders.authorizationHeader) != 'Bearer $token') {
      res.statusCode = 401;
      res.write(jsonEncode({'error': 'missing or invalid token'}));
      await res.close();
      return;
    }
    final body = await utf8.decoder.bind(req).join();
    final path = req.uri.path;
    res.headers.contentType = ContentType.json;
    if (path == '/v1/health') {
      res.write(jsonEncode({'ok': true}));
    } else if (req.method == 'GET' && path == '/v1/notebooks') {
      res.write(
        jsonEncode({
          'notebooks': [notebookJson],
        }),
      );
    } else if (req.method == 'POST' && path == '/v1/notebooks') {
      final m = jsonDecode(body) as Map<String, Object?>;
      res.statusCode = 201;
      res.write(jsonEncode({'id': 'new', 'name': m['name'], 'workType': m['workType'], 'context': <Object>[]}));
    } else if (req.method == 'PATCH' && path == '/v1/notebooks/board/context/c2') {
      res.write(jsonEncode(notebookJson));
    } else if (path == '/v1/outputs') {
      res.write(
        jsonEncode({
          'outputs': [
            {
              'id': 'o1',
              'title': 'Digest',
              'workType': req.uri.queryParameters['workType'] ?? 'summarize',
              'notebookName': 'Board',
              'createdAt': '2026-11-20',
            },
          ],
        }),
      );
    } else if (path == '/v1/notebooks/board/runs') {
      res.headers.contentType = ContentType('text', 'event-stream');
      res.write('event: status\ndata: {"text":"generating"}\n\n');
      res.write('event: delta\ndata: {"text":"# Dig"}\n\n');
      res.write(
        'event: finished\ndata: {"valid":true,"modelUsed":"litellm/b","totalTokens":42,"markdown":"# Digest",'
        '"attempts":[{"target":"litellm/a","trigger":"rate_limit"},{"target":"litellm/b"}]}\n\n',
      );
    } else if (path == '/v1/notebooks/bad/runs') {
      res.headers.contentType = ContentType('text', 'event-stream');
      res.write('event: error\ndata: {"status":422,"error":"output failed schema validation"}\n\n');
    } else {
      res.statusCode = 404;
      res.write(jsonEncode({'error': 'not found'}));
    }
    await res.close();
  });
  return server;
}

void main() {
  late HttpServer server;
  late HttpCoreClient client;

  setUp(() async {
    server = await fakeCore(await HttpServer.bind(InternetAddress.loopbackIPv4, 0));
    client = HttpCoreClient(
      Endpoint(network: 'tcp', address: '127.0.0.1:${server.port}', token: token),
      client: realHttpClient(),
    );
  });

  tearDown(() async {
    client.close();
    await server.close(force: true);
  });

  test('lists, creates and toggles notebooks', () async {
    final nbs = await client.listNotebooks();
    expect(nbs.single.name, 'Board');
    expect(nbs.single.workType, WorkTypeId.summarize);
    expect(nbs.single.context.first.kind, 'MD');
    expect(nbs.single.context.last.kind, 'Folder');
    expect(nbs.single.enabledTokens, 1200);
    final created = await client.createNotebook(name: 'Ops', workType: WorkTypeId.report);
    expect(created.workType, WorkTypeId.report);
    final toggled = await client.setContextEnabled(notebookId: 'board', contextId: 'c2', enabled: true);
    expect(toggled.id, 'board');
    expect(await client.listWorkTypes(), hasLength(4));
    expect(await client.health(), isTrue);
  });

  test('lists outputs with a filter', () async {
    final outs = await client.listOutputs(workType: WorkTypeId.report);
    expect(outs.single.workType, WorkTypeId.report);
    expect((await client.listOutputs()).single.title, 'Digest');
  });

  test('streams a run and reports fallbacks', () async {
    final events = await client.startRun(notebookId: 'board', prompt: 'p', vars: const {}).toList();
    expect(events[0], isA<RunStatus>());
    expect((events[1] as RunDelta).text, '# Dig');
    final outcome = (events[2] as RunFinished).outcome;
    expect(outcome.valid, isTrue);
    expect(outcome.modelUsed, 'litellm/b');
    expect(outcome.fallbackNote, contains('rate_limit'));
  });

  test('surfaces errors as CoreException', () async {
    await expectLater(
      client.startRun(notebookId: 'bad', prompt: 'p', vars: const {}).toList(),
      throwsA(isA<CoreException>().having((e) => e.status, 'status', 422)),
    );
    await expectLater(
      client.startRun(notebookId: 'missing', prompt: 'p', vars: const {}).toList(),
      throwsA(isA<CoreException>().having((e) => e.status, 'status', 404)),
    );
    final wrong = HttpCoreClient(
      Endpoint(network: 'tcp', address: '127.0.0.1:${server.port}', token: 'nope'),
      client: realHttpClient(),
    );
    await expectLater(wrong.listNotebooks(), throwsA(isA<CoreException>().having((e) => e.status, 'status', 401)));
    wrong.close();
  });

  test('works over a Unix socket', () async {
    if (Platform.isWindows) return;
    final dir = await Directory.systemTemp.createTemp('tsv');
    final sock = '${dir.path}/core.sock';
    final unixServer = await fakeCore(await HttpServer.bind(InternetAddress(sock, type: InternetAddressType.unix), 0));
    final unixClient = HttpCoreClient(Endpoint(network: 'unix', address: sock, token: token), client: realHttpClient());
    expect((await unixClient.listNotebooks()).single.id, 'board');
    unixClient.close();
    await unixServer.close(force: true);
    await dir.delete(recursive: true);
  });

  group('endpoint discovery', () {
    test('parses and validates the endpoint file', () async {
      final dir = await Directory.systemTemp.createTemp('tsv');
      final f = File('${dir.path}/endpoint.json');
      expect(await Endpoint.discover(path: f.path), isNull);
      await f.writeAsString(jsonEncode({'network': 'unix', 'address': '/tmp/s', 'token': 't'}));
      final ep = await Endpoint.discover(path: f.path);
      expect(ep?.network, 'unix');
      await f.writeAsString(jsonEncode({'network': 'udp', 'address': 'x', 'token': 't'}));
      await expectLater(Endpoint.discover(path: f.path), throwsFormatException);
      await f.writeAsString(jsonEncode({'network': 'tcp'}));
      await expectLater(Endpoint.discover(path: f.path), throwsFormatException);
      await f.writeAsString('[1]');
      await expectLater(Endpoint.discover(path: f.path), throwsFormatException);
      await dir.delete(recursive: true);
    });

    test('default path matches Go os.UserConfigDir', () {
      expect(defaultEndpointPath({'HOME': '/h'}, os: 'linux'), '/h/.config/tessivo/run/endpoint.json');
      final xdg = defaultEndpointPath({'HOME': '/h', 'XDG_CONFIG_HOME': '/x'}, os: 'linux');
      expect(xdg, '/x/tessivo/run/endpoint.json');
      expect(
        defaultEndpointPath({'HOME': '/h'}, os: 'macos'),
        '/h/Library/Application Support/tessivo/run/endpoint.json',
      );
      expect(defaultEndpointPath({'APPDATA': r'C:\A'}, os: 'windows'), r'C:\A\tessivo\run\endpoint.json');
    });

    test('connectCore falls back to demo data', () async {
      expect(await connectCore(() async => null), isA<DemoCoreClient>());
      expect(await connectCore(() async => throw const CoreStartException('no tess')), isA<DemoCoreClient>());
      final real = await connectCore(() async => const Endpoint(network: 'tcp', address: '127.0.0.1:1', token: 't'));
      expect(real, isA<HttpCoreClient>());
      (real as HttpCoreClient).close();
    });
  });

  test('SSE parser handles multi-line data and trailing events', () async {
    final events = await parseSse(Stream.fromIterable(['data: a\ndata: b\n\n', 'event: x\ndata: c'])).toList();
    expect(events.first.data, 'a\nb');
    expect(events.first.event, 'message');
    expect(events.last.event, 'x');
  });
}

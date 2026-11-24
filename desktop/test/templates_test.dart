import 'dart:convert';
import 'dart:io';

import 'package:flutter_test/flutter_test.dart';
import 'package:material_ui/material_ui.dart';
import 'package:tessivo_desktop/main.dart';
import 'package:tessivo_desktop/src/app_state.dart';
import 'package:tessivo_desktop/src/core/http_core_client.dart';
import 'package:tessivo_desktop/src/core/models.dart';

import 'support/fake_core/fake_core.dart';
import 'support/fake_core/fake_templates.dart';
import 'support/real_http.dart';

const token = 'secret-token';

const templateJson = {
  'id': 'weekly-ops-report',
  'name': 'Weekly ops report',
  'description': 'Weekly status',
  'workType': 'report',
  'version': 1,
  'source': 'builtin',
  'prompts': [
    {'name': 'weekly', 'text': 'Produce it', 'settings': <String, Object>{}},
  ],
  'slots': [
    {'name': 'Tickets', 'kind': 'spreadsheet', 'description': '', 'required': true},
  ],
  'schedule': {'cron': '0 8 * * MON', 'missed': 'skip'},
};

/// Fake of the core's template endpoints; records request bodies.
Future<(HttpServer, List<Map<String, Object?>>)> fakeTemplates() async {
  final server = await HttpServer.bind(InternetAddress.loopbackIPv4, 0);
  final bodies = <Map<String, Object?>>[];
  server.listen((req) async {
    final res = req.response;
    final text = await utf8.decoder.bind(req).join();
    if (text.isNotEmpty) bodies.add(jsonDecode(text) as Map<String, Object?>);
    final path = req.uri.path;
    if (req.method == 'GET' && path == '/v1/templates') {
      final wt = req.uri.queryParameters['workType'];
      res.write(
        jsonEncode({
          'templates': wt == null || wt == 'report' ? [templateJson] : <Object>[],
        }),
      );
    } else if (req.method == 'POST' && path == '/v1/notebooks') {
      res.statusCode = 201;
      res.write(jsonEncode({'id': 'n1', 'name': 'Ops', 'workType': 'report', 'context': <Object>[]}));
    } else if (req.method == 'POST' && path == '/v1/templates') {
      res.statusCode = 201;
      res.write(
        jsonEncode({
          'id': 'mine',
          'name': 'Mine',
          'workType': 'analyze',
          'version': 2,
          'source': 'user',
          'parent': 'change-risk-review@1',
          'prompts': <Object>[],
          'slots': <Object>[],
        }),
      );
    } else if (req.method == 'DELETE' && path == '/v1/templates/mine') {
      res.statusCode = 204;
    } else {
      res.statusCode = 404;
      res.write(jsonEncode({'error': 'only user templates can be deleted'}));
    }
    await res.close();
  });
  return (server, bodies);
}

Future<AppState> pumpTemplates(WidgetTester tester, {FakeCoreClient? core}) async {
  tester.view.physicalSize = const Size(1600, 1200);
  tester.view.devicePixelRatio = 1;
  addTearDown(tester.view.reset);
  final state = AppState(core ?? FakeCoreClient(tick: Duration.zero));
  await tester.pumpWidget(TessivoApp(state: state));
  await state.load();
  state.go(Section.templates);
  await tester.pumpAndSettle();
  return state;
}

Future<void> nameDialog(WidgetTester tester, String name) async {
  await tester.enterText(find.byKey(const Key('text-prompt')), name);
  await tester.tap(find.byKey(const Key('text-prompt-ok')));
  await tester.pumpAndSettle();
}

void main() {
  emptyTemplatesTests();
  group('http client', () {
    late HttpServer server;
    late List<Map<String, Object?>> bodies;
    late HttpCoreClient client;

    setUp(() async {
      (server, bodies) = await fakeTemplates();
      client = HttpCoreClient(
        Endpoint(network: 'tcp', address: '127.0.0.1:${server.port}', token: token),
        client: realHttpClient(),
      );
    });
    tearDown(() async {
      client.close();
      await server.close(force: true);
    });

    test('lists templates with slots, prompts and schedule', () async {
      final t = (await client.listTemplates()).single;
      expect(t.workType, WorkTypeId.report);
      expect(t.prompts, ['weekly']);
      expect(t.slots.single.required, isTrue);
      expect(t.scheduleCron, '0 8 * * MON');
      expect(await client.listTemplates(workType: WorkTypeId.analyze), isEmpty);
    });

    test('creates from template, saves and deletes', () async {
      expect((await client.createFromTemplate('weekly-ops-report', name: 'Ops')).id, 'n1');
      expect(bodies.last, {'templateId': 'weekly-ops-report', 'name': 'Ops'});
      await client.createFromTemplate('weekly-ops-report');
      expect(bodies.last, {'templateId': 'weekly-ops-report'});

      final saved = await client.saveAsTemplate('board', name: 'Mine');
      expect(saved.isUser, isTrue);
      expect(saved.parent, 'change-risk-review@1');
      expect(bodies.last['fromNotebook'], 'board');

      await client.deleteTemplate('mine');
      expect(() => client.deleteTemplate('weekly-ops-report'), throwsA(isA<CoreException>()));
    });
  });

  test('template ids follow the core slug rule', () {
    expect(templateIdFor('  My Weekly Report! '), 'my-weekly-report');
    expect(templateIdFor('!!!'), isEmpty);
    expect(templateIdFor('a' * 70), hasLength(63));
  });

  test('demo core guards template operations', () async {
    final core = FakeCoreClient(tick: Duration.zero);
    expect(() => core.createFromTemplate('missing'), throwsStateError);
    expect(() => core.saveAsTemplate('missing', name: 'x'), throwsStateError);
    expect(() => core.saveAsTemplate('nb-1', name: '!!!'), throwsFormatException);
    expect(() => core.deleteTemplate('weekly-ops-report'), throwsStateError);
    expect((await core.createFromTemplate('contract-digest')).name, 'Contract digest');
  });

  testWidgets('gallery filters by work type', (tester) async {
    await pumpTemplates(tester);
    expect(find.byKey(const Key('template-card-weekly-ops-report')), findsOneWidget);
    await tester.tap(find.byKey(const Key('template-card-weekly-ops-report')));
    await tester.pumpAndSettle();
    final detail = find.byKey(const Key('template-detail'));
    expect(find.descendant(of: detail, matching: find.textContaining('Tickets (required)')), findsOneWidget);
    expect(find.descendant(of: detail, matching: find.textContaining('Previous report')), findsOneWidget);

    await tester.tap(find.byKey(const Key('template-filter-summarize')));
    await tester.pumpAndSettle();
    expect(find.byKey(const Key('template-card-contract-digest')), findsOneWidget);
    expect(find.byKey(const Key('template-card-weekly-ops-report')), findsNothing);
  });

  testWidgets('using a template creates and opens a notebook', (tester) async {
    final state = await pumpTemplates(tester);
    await tester.tap(find.byKey(const Key('template-use-weekly-ops-report')));
    await tester.pumpAndSettle();
    await nameDialog(tester, 'Ops for team B');
    expect(state.section, Section.notebooks);
    expect(state.selected?.name, 'Ops for team B');
    expect(state.selected?.workType, WorkTypeId.report);
  });

  testWidgets('cancelling the name dialog does nothing', (tester) async {
    final state = await pumpTemplates(tester);
    final before = state.notebooks.length;
    await tester.tap(find.byKey(const Key('template-use-contract-digest')));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Cancel'));
    await tester.pumpAndSettle();
    expect(state.notebooks, hasLength(before));
    expect(state.section, Section.templates);
  });

  testWidgets('saves the selected notebook as a template and removes it', (tester) async {
    await pumpTemplates(tester);
    await tester.tap(find.byKey(const Key('save-as-template')));
    await tester.pumpAndSettle();
    await nameDialog(tester, 'My risk review');
    expect(find.byKey(const Key('template-card-my-risk-review')), findsOneWidget);
    expect(find.text('Yours'), findsOneWidget);

    await tester.tap(find.byKey(const Key('template-delete-my-risk-review')));
    await tester.pumpAndSettle();
    expect(find.byKey(const Key('template-card-my-risk-review')), findsNothing);
  });

  testWidgets('a bad template name shows an error', (tester) async {
    await pumpTemplates(tester);
    await tester.tap(find.byKey(const Key('save-as-template')));
    await tester.pumpAndSettle();
    await nameDialog(tester, '***');
    expect(find.text('Enter a template name'), findsOneWidget);
  });

  testWidgets('new notebook dialog links to the gallery', (tester) async {
    final state = await pumpTemplates(tester);
    await tester.tap(find.byKey(const Key('activity-notebooks')));
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const Key('new-notebook')));
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const Key('browse-templates')));
    await tester.pumpAndSettle();
    expect(state.section, Section.templates);
    expect(find.byKey(const Key('template-card-change-risk-review')), findsOneWidget);
  });
}

void emptyTemplatesTests() {
  testWidgets('with no templates the gallery says how to make one', (tester) async {
    await pumpTemplates(tester, core: _NoTemplates());
    expect(find.byKey(const Key('no-templates')), findsOneWidget);
  });
}

class _NoTemplates extends FakeCoreClient {
  _NoTemplates() : super(tick: Duration.zero);

  @override
  Future<List<TemplateInfo>> listTemplates({WorkTypeId? workType}) async => const [];
}

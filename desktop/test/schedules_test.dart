import 'dart:async';
import 'dart:convert';
import 'dart:io';

import 'package:flutter_test/flutter_test.dart';
import 'package:material_ui/material_ui.dart';
import 'package:tessivo_desktop/main.dart';
import 'package:tessivo_desktop/src/app_state.dart';
import 'package:tessivo_desktop/src/core/core_client.dart';
import 'package:tessivo_desktop/src/core/http_core_client.dart';
import 'package:tessivo_desktop/src/core/models.dart';

const token = 'secret-token';

const scheduleJson = {
  'id': 's1',
  'cron': '0 8 * * MON',
  'missed': 'run_once',
  'enabled': true,
  'prompt': 'Weekly',
  'nextRun': '2026-11-23T08:00:00Z',
  'history': [
    {'status': 'failed', 'startedAt': '2026-11-16T08:00:00Z', 'error': 'model unavailable'},
  ],
};

/// Fake of the core's schedule and event endpoints.
Future<HttpServer> fakeSchedules() async {
  final server = await HttpServer.bind(InternetAddress.loopbackIPv4, 0);
  server.listen((req) async {
    final res = req.response;
    final body = await utf8.decoder.bind(req).join();
    final path = req.uri.path;
    const base = '/v1/notebooks/board/schedules';
    if (req.headers.value(HttpHeaders.authorizationHeader) != 'Bearer $token') {
      res.statusCode = 401;
    } else if (req.method == 'GET' && path == base) {
      res.write(
        jsonEncode({
          'schedules': [scheduleJson],
        }),
      );
    } else if (req.method == 'POST' && path == base) {
      final m = jsonDecode(body) as Map<String, Object?>;
      res.statusCode = 201;
      res.write(
        jsonEncode({'id': 's2', 'cron': m['cron'], 'missed': m['missed'], 'enabled': true, 'history': <Object>[]}),
      );
    } else if (req.method == 'PATCH' && path == '$base/s1') {
      final m = jsonDecode(body) as Map<String, Object?>;
      res.write(jsonEncode({...scheduleJson, 'enabled': m['enabled']}));
    } else if (req.method == 'DELETE' && path == '$base/s1') {
      res.statusCode = 204;
    } else if (path == '/v1/events') {
      res.headers.contentType = ContentType('text', 'event-stream');
      res.write(': keepalive\n\n');
      res.write('event: other\ndata: {"kind":"schedule.finished","notebookName":"x"}\n\n');
      res.write('event: event\ndata: not json\n\n');
      res.write('event: event\ndata: {"kind":"schedule.finished","notebookName":"Board","title":"Digest"}\n\n');
    } else {
      res.statusCode = 404;
      res.write(jsonEncode({'error': 'not found'}));
    }
    await res.close();
  });
  return server;
}

Future<AppState> pumpSchedules(WidgetTester tester) async {
  tester.view.physicalSize = const Size(1600, 1000);
  tester.view.devicePixelRatio = 1;
  addTearDown(tester.view.reset);
  final state = AppState(DemoCoreClient(tick: Duration.zero));
  await tester.pumpWidget(TessivoApp(state: state));
  await state.load();
  state.select('nb-4');
  state.go(Section.schedules);
  await tester.pumpAndSettle();
  return state;
}

void main() {
  group('http client', () {
    late HttpServer server;
    late HttpCoreClient client;

    setUp(() async {
      server = await fakeSchedules();
      client = HttpCoreClient(Endpoint(network: 'tcp', address: '127.0.0.1:${server.port}', token: token));
    });
    tearDown(() async {
      client.close();
      await server.close(force: true);
    });

    test('lists, adds, toggles and deletes schedules', () async {
      final list = await client.listSchedules('board');
      expect(list.single.cron, '0 8 * * MON');
      expect(list.single.history.single.error, 'model unavailable');
      final added = await client.addSchedule('board', cron: '@daily', missed: 'skip');
      expect(added.id, 's2');
      expect((await client.setScheduleEnabled('board', 's1', enabled: false)).enabled, isFalse);
      await client.deleteSchedule('board', 's1');
      expect(() => client.deleteSchedule('board', 'nope'), throwsA(isA<CoreException>()));
    });

    test('events keep only well-formed event frames', () async {
      final events = await client.events().toList();
      expect(events.single.message, 'Board: Digest is ready');
    });

    test('events with a bad token fail', () async {
      final bad = HttpCoreClient(Endpoint(network: 'tcp', address: '127.0.0.1:${server.port}', token: 'x'));
      addTearDown(bad.close);
      expect(bad.events().toList(), throwsA(isA<CoreException>()));
    });
  });

  test('event messages cover every kind', () {
    String msg(String kind, {String? error}) => CoreEvent(kind: kind, notebookName: 'N', error: error).message;
    expect(msg('schedule.finished'), 'N: scheduled run is ready');
    expect(msg('schedule.failed', error: 'boom'), 'N: scheduled run failed (boom)');
    expect(msg('schedule.skipped'), 'N: missed run skipped');
    expect(msg('schedule.started'), 'N: scheduled run started');
  });

  testWidgets('schedules screen lists, adds, toggles and removes', (tester) async {
    await pumpSchedules(tester);
    expect(find.text('0 8 * * MON'), findsOneWidget);

    await tester.enterText(find.byKey(const Key('schedule-cron')), 'nonsense');
    await tester.tap(find.byKey(const Key('schedule-add')));
    await tester.pumpAndSettle();
    expect(find.byKey(const Key('schedule-error')), findsOneWidget);

    await tester.enterText(find.byKey(const Key('schedule-cron')), '@daily');
    await tester.tap(find.byKey(const Key('schedule-add')));
    await tester.pumpAndSettle();
    expect(find.text('@daily'), findsOneWidget);
    expect(find.byKey(const Key('schedule-error')), findsNothing);

    final toggle = find.byKey(const Key('schedule-enabled-s1'));
    await tester.tap(toggle);
    await tester.pumpAndSettle();
    expect(tester.widget<Switch>(toggle).value, isFalse);

    await tester.tap(find.byKey(const Key('schedule-delete-s1')));
    await tester.pumpAndSettle();
    expect(find.text('0 8 * * MON'), findsNothing);
  });

  testWidgets('core events show a dismissible banner', (tester) async {
    final state = await pumpSchedules(tester);
    final core = state.core as DemoCoreClient;
    state.listenForEvents();
    core.emit(const CoreEvent(kind: 'schedule.started', notebookName: 'Ops'));
    core.emit(const CoreEvent(kind: 'schedule.finished', notebookName: 'Ops', title: 'Weekly'));
    core.emit(const CoreEvent(kind: 'schedule.failed', notebookName: 'Ops'));
    await tester.pumpAndSettle();
    expect(find.byKey(const Key('notice-banner')), findsOneWidget);
    expect(find.text('Ops: scheduled run failed  (+1 more)'), findsOneWidget);

    await tester.tap(find.byKey(const Key('notice-dismiss')));
    await tester.pumpAndSettle();
    expect(find.text('Ops: Weekly is ready'), findsOneWidget);
    await tester.tap(find.byKey(const Key('notice-dismiss')));
    await tester.pumpAndSettle();
    expect(find.byKey(const Key('notice-banner')), findsNothing);
    state.stopEvents();
  });

  test('an ended event stream schedules a reconnect that stopping cancels', () async {
    final core = _EndingCore();
    final state = AppState(core)..listenForEvents();
    await Future<void>.delayed(Duration.zero);
    expect(core.subscriptions, 1);
    state.dispose();
  });
}

class _EndingCore extends DemoCoreClient {
  int subscriptions = 0;

  @override
  Stream<CoreEvent> events() {
    subscriptions++;
    return const Stream.empty();
  }
}

import 'dart:convert';
import 'dart:io';

import 'package:flutter_test/flutter_test.dart';
import 'package:material_ui/material_ui.dart';
import 'package:tessivo_desktop/main.dart';
import 'package:tessivo_desktop/src/app_state.dart';
import 'package:tessivo_desktop/src/core/http_core_client.dart';
import 'package:tessivo_desktop/src/core/settings_draft.dart';

import 'support/fake_core/fake_core.dart';
import 'support/real_http.dart';

Future<(AppState, FakeCoreClient)> _pump(WidgetTester tester, {bool keySet = true}) async {
  tester.view.physicalSize = const Size(1600, 1200);
  tester.view.devicePixelRatio = 1;
  addTearDown(tester.view.reset);
  final core = FakeCoreClient(tick: Duration.zero);
  if (!keySet) await core.deleteSecret('TESSIVO_LITELLM_KEY');
  final state = AppState(core);
  await tester.pumpWidget(TessivoApp(state: state));
  await state.load();
  await tester.pumpAndSettle();
  return (state, core);
}

Future<void> _openSettings(WidgetTester tester) async {
  await tester.tap(find.text('Settings'));
  await tester.pumpAndSettle();
}

void main() {
  trayToggleTests();
  testWidgets('edit run defaults and limits, then save', (tester) async {
    final (_, core) = await _pump(tester);
    await _openSettings(tester);
    await tester.enterText(find.byKey(const Key('default-model')), 'litellm/qwen');
    await tester.enterText(find.byKey(const Key('limit-day')), '50000');
    await tester.tap(find.byKey(const Key('save-defaults')));
    await tester.pumpAndSettle();
    final s = await core.getSettings();
    expect((s.defaultModel, s.maxTokensPerDay), ('litellm/qwen', 50000));
    expect(find.text('Saved'), findsOneWidget);

    await tester.enterText(find.byKey(const Key('limit-day')), 'lots');
    await tester.tap(find.byKey(const Key('save-defaults')));
    await tester.pumpAndSettle();
    expect(find.text('Enter a whole number'), findsOneWidget);
  });

  testWidgets('set an API key without ever showing it', (tester) async {
    final (_, core) = await _pump(tester, keySet: false);
    await _openSettings(tester);
    expect(find.text('Key not set'), findsOneWidget);
    await tester.tap(find.byKey(const Key('set-key-litellm')));
    await tester.pumpAndSettle();
    final field = find.byKey(const Key('key-value'));
    expect(tester.widget<TextField>(field).obscureText, isTrue);
    await tester.enterText(field, 'sk-secret-123');
    await tester.tap(find.byKey(const Key('save-key')));
    await tester.pumpAndSettle();
    expect(await core.secretSet('TESSIVO_LITELLM_KEY'), isTrue);
    expect(find.text('Key set'), findsOneWidget);
    expect(find.textContaining('sk-secret'), findsNothing);
  });

  testWidgets('add a provider and edit an alias chain', (tester) async {
    final (_, core) = await _pump(tester);
    await _openSettings(tester);
    await tester.tap(find.byKey(const Key('add-provider')));
    await tester.pumpAndSettle();
    await tester.enterText(find.byKey(const Key('provider-name')), 'ollama');
    await tester.enterText(find.byKey(const Key('provider-url')), 'http://localhost:11434/v1');
    await tester.tap(find.byKey(const Key('save-provider')));
    await tester.pumpAndSettle();
    expect((await core.getSettings()).providers.map((p) => p.name), contains('ollama'));

    await tester.tap(find.byKey(const Key('edit-alias-smart')));
    await tester.pumpAndSettle();
    await tester.enterText(find.byKey(const Key('alias-chain')), 'ollama/llama3\nlitellm/fallback-model');
    await tester.tap(find.byKey(const Key('save-alias')));
    await tester.pumpAndSettle();
    expect((await core.getSettings()).aliases.single.chain, ['ollama/llama3', 'litellm/fallback-model']);
  });

  testWidgets('a missing key shows a setup banner that leads to settings', (tester) async {
    final (state, core) = await _pump(tester, keySet: false);
    expect(find.byKey(const Key('setup-banner')), findsOneWidget);
    expect(find.textContaining('API key'), findsWidgets);
    await tester.tap(find.byKey(const Key('setup-open')));
    await tester.pumpAndSettle();
    expect(state.section, Section.settings);
    await core.setSecret('TESSIVO_LITELLM_KEY', 'k');
    await state.checkSetup();
    await tester.pumpAndSettle();
    expect(find.byKey(const Key('setup-banner')), findsNothing);
  });

  test('the draft keeps fields the desktop does not edit', () {
    final draft = SettingsDraft.fromJson({
      'providers': [
        {
          'name': 'litellm',
          'base_url': 'http://x/v1',
          'api_key_ref': 'K',
          'timeout_seconds': 90,
          'headers': {'X-Team': 'ops'},
        },
      ],
      'aliases': [
        {
          'name': 'smart',
          'chain': [
            {'provider': 'litellm', 'model': 'a'},
          ],
          'max_retries': 3,
        },
      ],
      'defaults': {'model': 'smart', 'context_budget': 64000},
      'limits': <String, Object?>{},
    });
    draft
      ..setProviderUrl('litellm', 'http://y/v1')
      ..setAliasChain('smart', ['litellm/b'])
      ..setDefault('model', 'litellm/b')
      ..setLimit('max_tokens_per_day', 10);
    final body = draft.toBody();
    final p = (body['providers']! as List).single as Map<String, Object?>;
    expect((p['base_url'], p['timeout_seconds']), ('http://y/v1', 90));
    expect(p['headers'], {'X-Team': 'ops'});
    final a = (body['aliases']! as List).single as Map<String, Object?>;
    expect(a['max_retries'], 3);
    expect(a['chain'], [
      {'provider': 'litellm', 'model': 'b'},
    ]);
    expect((body['defaults']! as Map)['context_budget'], 64000);
    expect((body['limits']! as Map)['max_tokens_per_day'], 10);
  });

  test('http client saves settings and keys', () async {
    final server = await HttpServer.bind(InternetAddress.loopbackIPv4, 0);
    final calls = <String>[];
    server.listen((req) async {
      calls.add('${req.method} ${req.uri.path} ${await utf8.decoder.bind(req).join()}');
      final res = req.response;
      if (req.uri.path.startsWith('/v1/secrets') && req.method == 'GET') {
        res.write(jsonEncode({'set': true}));
      } else if (req.method == 'PUT' && req.uri.path == '/v1/settings') {
        res.write(jsonEncode({'providers': <Object>[], 'aliases': <Object>[], 'defaults': <String, Object?>{}}));
      } else {
        res.statusCode = 204;
      }
      await res.close();
    });
    final client = HttpCoreClient(
      Endpoint(network: 'tcp', address: '127.0.0.1:${server.port}', token: 't'),
      client: realHttpClient(),
    );
    expect(await client.secretSet('K'), isTrue);
    await client.setSecret('K', 'v');
    expect(calls.last, 'PUT /v1/secrets/K {"value":"v"}');
    await client.deleteSecret('K');
    expect(calls.last, startsWith('DELETE /v1/secrets/K'));
    await client.updateSettings({'providers': <Object>[], 'aliases': <Object>[], 'defaults': <String, Object?>{}});
    expect(calls.last, startsWith('PUT /v1/settings'));
    client.close();
    await server.close(force: true);
  });
}

void trayToggleTests() {
  testWidgets('the tray toggle saves desktop.close_to_tray', (tester) async {
    final (_, core) = await _pump(tester);
    await _openSettings(tester);
    await tester.scrollUntilVisible(
      find.byKey(const Key('close-to-tray')),
      200,
      scrollable: find.descendant(of: find.byKey(const Key('settings-list')), matching: find.byType(Scrollable)).first,
    );
    await tester.tap(find.byKey(const Key('close-to-tray')));
    await tester.pumpAndSettle();
    expect(SettingsDraft.fromJson((await core.getSettings()).raw).closeToTray, isTrue);
  });
}

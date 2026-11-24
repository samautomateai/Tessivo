import 'package:flutter_test/flutter_test.dart';
import 'package:tessivo_desktop/src/thread_state.dart';

import 'support/fake_core/fake_core.dart';

void main() {
  late FakeCoreClient core;
  late List<String> errors;
  late int applied;
  late ThreadState t;

  setUp(() async {
    core = FakeCoreClient(tick: Duration.zero);
    errors = [];
    applied = 0;
    t = ThreadState(core, 'nb-1', 'o1', onError: errors.add, onApplied: () async => applied++);
    await t.load();
  });
  tearDown(() => t.dispose());

  test('load reads the thread, suggestions and the output', () {
    expect(t.session?.messages, hasLength(2));
    expect(t.suggestions.single.id, 'sg1');
    expect(t.detail?.title, 'PR #482 risk report');
    expect(t.openSuggestions, hasLength(1));
  });

  test('send streams the reply and stores both turns', () async {
    expect(await t.send('Where is that check?'), isTrue);
    expect(t.streaming, isFalse);
    expect(t.session!.messages.map((m) => m.role).toList(), ['user', 'assistant', 'user', 'assistant']);
    expect(await t.send('   '), isFalse);
  });

  test('apply refreshes the notebook; applying twice reports the error', () async {
    await t.apply('sg1');
    expect(t.suggestions.single.status, 'applied');
    expect(applied, 1);
    await t.apply('sg1');
    expect(errors.single, contains('already applied'));
  });

  test('suggest adds suggestions and dismiss resolves one', () async {
    await t.suggest();
    final fresh = t.openSuggestions.firstWhere((s) => s.kind == 'memory');
    await t.dismiss(fresh.id);
    expect(t.openSuggestions.map((s) => s.id), isNot(contains(fresh.id)));
    expect(t.busy, isFalse);
  });

  test('newVersion returns the revision and notes it in the thread', () async {
    final v = await t.newVersion();
    expect(v?.parentId, 'o1');
    expect(t.session!.messages.last.role, 'note');
  });

  test('applying tells the user which prompt it created', () async {
    final notes = <String>[];
    final created = ThreadState(_CreatingCore(), 'nb-1', 'o1', onError: errors.add, onNotice: notes.add);
    await created.load();
    await created.apply('sg1');
    expect(notes.single, contains('refined-1'));
    created.dispose();
  });

  test('errors from the core become one message', () async {
    final missing = ThreadState(core, 'nb-1', 'nope', onError: errors.add);
    await missing.load();
    expect(errors.single, contains('not found'));
    missing.dispose();
  });
}

/// Applying creates a new saved prompt, as the core does for ad-hoc runs.
class _CreatingCore extends FakeCoreClient {
  _CreatingCore() : super(tick: Duration.zero);

  @override
  Future<({Suggestion suggestion, String? promptName})> applySuggestion(
    String notebookId,
    String outputId,
    String suggestionId,
  ) async {
    final r = await super.applySuggestion(notebookId, outputId, suggestionId);
    return (suggestion: r.suggestion, promptName: 'refined-1');
  }
}

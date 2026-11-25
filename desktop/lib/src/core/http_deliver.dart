part of 'http_core_client.dart';

/// Issues, publishing and integrations (docs/api.md, "Deliver results").
mixin _HttpDeliver on _HttpChat {
  String _outPath(String notebookId, String outputId) =>
      '/v1/notebooks/${Uri.encodeComponent(notebookId)}/outputs/${Uri.encodeComponent(outputId)}';

  Future<List<IssueInfo>> outputIssues(String notebookId, String outputId) async => jsonList(
    (await _json('GET', '${_outPath(notebookId, outputId)}/issues'))['issues'],
  ).map(IssueInfo.fromJson).toList();

  Future<PushResult> pushIssues(
    String notebookId,
    String outputId,
    String integration, {
    List<int> findings = const [],
    bool again = false,
  }) async => PushResult.fromJson(
    await _json(
      'POST',
      '${_outPath(notebookId, outputId)}/issues/push',
      body: {'integration': integration, if (findings.isNotEmpty) 'findings': findings, if (again) 'again': true},
    ),
  );

  Future<Delivery> publishOutput(String notebookId, String outputId, String integration) async => Delivery.fromJson(
    await _json('POST', '${_outPath(notebookId, outputId)}/publish', body: {'integration': integration}),
  );

  Future<List<Delivery>> deliveries(String notebookId, String outputId) async => jsonList(
    (await _json('GET', '${_outPath(notebookId, outputId)}/deliveries'))['deliveries'],
  ).map(Delivery.fromJson).toList();

  Future<List<IntegrationInfo>> listIntegrations() async =>
      jsonList((await _json('GET', '/v1/integrations'))['integrations']).map(IntegrationInfo.fromJson).toList();

  Future<IntegrationInfo> addIntegration(IntegrationInfo integration) async =>
      IntegrationInfo.fromJson(await _json('POST', '/v1/integrations', body: integration.toBody()));

  Future<void> removeIntegration(String name) async {
    await _json('DELETE', '/v1/integrations/${Uri.encodeComponent(name)}');
  }

  Future<void> testIntegration(String name) async {
    await _json('POST', '/v1/integrations/${Uri.encodeComponent(name)}/test');
  }
}

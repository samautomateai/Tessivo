/// The result of one tiny request to one model (`POST /v1/models/test`).
class ModelCheck {
  const ModelCheck({required this.model, required this.ok, this.millis = 0, this.reply = '', this.error = ''});

  factory ModelCheck.fromJson(Map<String, Object?> j) => ModelCheck(
    model: j['model'] as String? ?? '',
    ok: j['ok'] == true,
    millis: (j['millis'] as num?)?.toInt() ?? 0,
    reply: j['reply'] as String? ?? '',
    error: j['error'] as String? ?? '',
  );

  final String model;
  final bool ok;
  final int millis;
  final String reply;
  final String error;
}

/// The checks in a `{checks: [...]}` reply.
List<ModelCheck> modelChecksFromJson(Object? j) => [
  if (j is Map && j['checks'] is List)
    for (final c in j['checks'] as List)
      if (c is Map<String, Object?>) ModelCheck.fromJson(c),
];

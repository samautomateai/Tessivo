/// Today's token use and the limits (`GET /v1/usage`); 0 means no limit.
class UsageInfo {
  const UsageInfo({required this.tokens, this.dayLimit = 0, this.runLimit = 0});

  factory UsageInfo.fromJson(Map<String, Object?> j) => UsageInfo(
    tokens: (j['tokens'] as num?)?.toInt() ?? 0,
    dayLimit: (j['dayLimit'] as num?)?.toInt() ?? 0,
    runLimit: (j['runLimit'] as num?)?.toInt() ?? 0,
  );

  final int tokens;
  final int dayLimit;
  final int runLimit;
}

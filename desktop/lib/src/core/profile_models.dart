/// One settings profile in a list (SET-10).
class ProfileSummary {
  const ProfileSummary({required this.name, this.providers = 0, this.defaultModel = ''});

  factory ProfileSummary.fromJson(Map<String, Object?> j) => ProfileSummary(
    name: j['name'] as String? ?? '',
    providers: (j['providers'] as num?)?.toInt() ?? 0,
    defaultModel: j['defaultModel'] as String? ?? '',
  );

  final String name;
  final int providers;
  final String defaultModel;
}

/// The settings profiles and the active one ('' when none is).
class ProfilesInfo {
  const ProfilesInfo({this.active = '', this.profiles = const []});

  factory ProfilesInfo.fromJson(Map<String, Object?> j) => ProfilesInfo(
    active: j['active'] as String? ?? '',
    profiles: [
      for (final p in (j['profiles'] as List<Object?>? ?? const []))
        if (p is Map<String, Object?>) ProfileSummary.fromJson(p),
    ],
  );

  final String active;
  final List<ProfileSummary> profiles;
}

/// A work type the sources point to, and why (WT-7); [workType] is '' when there is none.
class WorkTypeSuggestion {
  const WorkTypeSuggestion({this.workType = '', this.reason = ''});

  factory WorkTypeSuggestion.fromJson(Map<String, Object?> j) =>
      WorkTypeSuggestion(workType: j['workType'] as String? ?? '', reason: j['reason'] as String? ?? '');

  final String workType;
  final String reason;
}

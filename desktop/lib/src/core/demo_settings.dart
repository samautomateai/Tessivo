import 'errors.dart';
import 'json_mapping.dart';
import 'models.dart';

/// Demo settings as the core's JSON, editable, plus which API keys are set.
mixin DemoSettings {
  Map<String, Object?> _settings = {
    'providers': [
      {'name': 'litellm', 'base_url': 'http://localhost:4000/v1', 'api_key_ref': 'TESSIVO_LITELLM_KEY'},
    ],
    'aliases': [
      {
        'name': 'smart',
        'chain': [
          {'provider': 'litellm', 'model': 'primary-model'},
          {'provider': 'litellm', 'model': 'fallback-model'},
        ],
        'fallback_on': ['timeout', 'rate_limit', 'server_error'],
      },
    ],
    'defaults': {'model': 'smart', 'thinking_effort': 'medium', 'max_output_tokens': 8000},
    'limits': {'max_tokens_per_day': 2000000},
  };
  final Set<String> _keys = {'TESSIVO_LITELLM_KEY'};

  Future<SettingsInfo> getSettings() async => settingsFromJson(_settings);

  Future<SettingsInfo> updateSettings(Map<String, Object?> body) async {
    final defaults = body['defaults'];
    final effort = defaults is Map ? defaults['thinking_effort'] : null;
    if (effort != null && !const ['off', 'low', 'medium', 'high'].contains(effort)) {
      throw CoreException(400, 'thinking_effort "$effort": want off, low, medium or high');
    }
    final names = {
      for (final p in body['providers'] as List? ?? const [])
        if (p is Map) p['name'],
    };
    for (final a in body['aliases'] as List? ?? const []) {
      for (final m in (a is Map ? a['chain'] as List? : null) ?? const []) {
        if (m is Map && !names.contains(m['provider'])) {
          throw CoreException(400, 'alias uses unknown provider "${m['provider']}"');
        }
      }
    }
    _settings = {..._settings, ...body};
    return getSettings();
  }

  Future<bool> secretSet(String ref) async => _keys.contains(ref);

  Future<void> setSecret(String ref, String value) async {
    if (value.isEmpty) throw const CoreException(400, 'the key must not be empty');
    _keys.add(ref);
  }

  Future<void> deleteSecret(String ref) async => _keys.remove(ref);
}

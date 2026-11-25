/// A notebook's saved prompt with its own model settings (docs/api.md, prompts).
class SavedPrompt {
  const SavedPrompt({
    required this.name,
    required this.text,
    this.model = '',
    this.temperature,
    this.maxOutputTokens,
    this.chain = '',
    this.version = 0,
  });

  factory SavedPrompt.fromJson(Map<String, Object?> j) {
    final s = j['settings'] is Map<String, Object?>
        ? j['settings']! as Map<String, Object?>
        : const <String, Object?>{};
    return SavedPrompt(
      name: j['name'] as String? ?? '',
      text: j['text'] as String? ?? '',
      model: s['model'] as String? ?? '',
      temperature: (s['temperature'] as num?)?.toDouble(),
      maxOutputTokens: (s['max_output_tokens'] as num?)?.toInt(),
      chain: s['chain'] as String? ?? '',
      version: (j['version'] as num?)?.toInt() ?? 0,
    );
  }

  final String name;
  final String text;
  final String model;
  final double? temperature;
  final int? maxOutputTokens;

  /// '' or `per_source`: read each source on its own first (RUN-4).
  final String chain;

  /// 1 for the first save; earlier versions are in the history (0: not known).
  final int version;

  /// The PUT body: the text and only the settings that are set.
  Map<String, Object?> toBody() {
    final settings = <String, Object?>{
      if (model.isNotEmpty) 'model': model,
      'temperature': ?temperature,
      'max_output_tokens': ?maxOutputTokens,
      if (chain.isNotEmpty) 'chain': chain,
    };
    return {'text': text, if (settings.isNotEmpty) 'settings': settings};
  }
}

/// The run setting that reads each source on its own first.
const chainPerSource = 'per_source';

/// One saved version of a prompt (PR-4); the first in a list is the current one.
class PromptVersion {
  const PromptVersion({required this.version, required this.text, this.savedAt = ''});

  factory PromptVersion.fromJson(Map<String, Object?> j) => PromptVersion(
    version: (j['version'] as num?)?.toInt() ?? 0,
    text: j['text'] as String? ?? '',
    savedAt: j['savedAt'] as String? ?? '',
  );

  final int version;
  final String text;
  final String savedAt;
}

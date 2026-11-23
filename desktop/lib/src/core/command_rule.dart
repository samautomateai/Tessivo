/// One entry of a notebook's run_command allow-list: a program and its
/// arguments, where "*" stands for exactly one free argument.
class CommandRule {
  const CommandRule({required this.program, this.args = const []});

  factory CommandRule.fromJson(Map<String, Object?> j) => CommandRule(
    program: j['program'] as String? ?? '',
    args: [
      for (final a in j['args'] as List? ?? const [])
        if (a is String) a,
    ],
  );

  /// Parses "go test *" (split on spaces) into a rule; null when empty.
  static CommandRule? parse(String line) {
    final words = line.trim().split(RegExp(r'\s+')).where((w) => w.isNotEmpty).toList();
    return words.isEmpty ? null : CommandRule(program: words.first, args: words.sublist(1));
  }

  final String program;
  final List<String> args;

  Map<String, Object?> toJson() => {'program': program, if (args.isNotEmpty) 'args': args};

  @override
  String toString() => [program, ...args].join(' ');
}

/// The rules in a JSON list.
List<CommandRule> commandRulesFromJson(Object? j) => [
  if (j is List)
    for (final r in j)
      if (r is Map<String, Object?>) CommandRule.fromJson(r),
];

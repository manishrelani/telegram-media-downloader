class TelegramChat {
  const TelegramChat({
    required this.id,
    required this.type,
    required this.name,
    this.username,
    this.topics = const <TelegramTopic>[],
  });

  final int id;
  final String type;
  final String name;
  final String? username;
  final List<TelegramTopic> topics;

  factory TelegramChat.fromJson(Map<String, dynamic> json) {
    final dynamic rawId = json['id'];
    final int? id = rawId is int ? rawId : int.tryParse('$rawId');
    if (id == null || id == 0) {
      throw const FormatException('Chat entry has no valid id.');
    }
    final String name =
        (json['name'] ?? json['visible_name'] ?? json['title'] ?? '')
            .toString()
            .trim();
    return TelegramChat(
      id: id,
      type: (json['type'] ?? 'unknown').toString().toLowerCase(),
      name: name.isEmpty ? 'Chat $id' : name,
      username: (json['username'] as String?)?.trim().isNotEmpty == true
          ? (json['username'] as String).trim()
          : null,
      topics: (json['topics'] as List<dynamic>? ?? <dynamic>[])
          .whereType<Map<String, dynamic>>()
          .map(TelegramTopic.fromJson)
          .where((TelegramTopic topic) => topic.id > 0)
          .toList(),
    );
  }

  Map<String, dynamic> toJson() => <String, dynamic>{
    'id': id,
    'type': type,
    'name': name,
    if (username != null) 'username': username,
    if (topics.isNotEmpty)
      'topics': topics.map((TelegramTopic topic) => topic.toJson()).toList(),
  };

  bool matches(String query) {
    final String needle = query.trim().toLowerCase();
    if (needle.isEmpty) return true;
    return name.toLowerCase().contains(needle) ||
        topics.any(
          (TelegramTopic topic) => topic.title.toLowerCase().contains(needle),
        ) ||
        id.toString().contains(needle) ||
        (username?.toLowerCase().contains(needle) ?? false);
  }

  String get displayType {
    if (type.contains('channel')) return 'Channel';
    if (type.contains('group')) return 'Group';
    if (type.contains('user') || type.contains('private')) {
      return 'Private chat';
    }
    return type.isEmpty ? 'Chat' : type[0].toUpperCase() + type.substring(1);
  }
}

class TelegramTopic {
  const TelegramTopic({required this.id, required this.title});
  final int id;
  final String title;
  factory TelegramTopic.fromJson(Map<String, dynamic> json) => TelegramTopic(
    id: (json['id'] as num?)?.toInt() ?? 0,
    title: (json['title'] ?? 'Untitled topic').toString(),
  );
  Map<String, dynamic> toJson() => <String, dynamic>{'id': id, 'title': title};
}

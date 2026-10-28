class BackendMessage {
  const BackendMessage({
    required this.id,
    required this.event,
    this.data = const <String, dynamic>{},
    this.errorCode,
    this.errorMessage,
  });

  final String id;
  final String event;
  final Map<String, dynamic> data;
  final String? errorCode;
  final String? errorMessage;

  bool get isError => event == 'error';

  factory BackendMessage.fromJson(Map<String, dynamic> json) {
    final dynamic rawData = json['data'];
    final dynamic rawError = json['error'];
    final Map<String, dynamic> error = rawError is Map<String, dynamic>
        ? rawError
        : const <String, dynamic>{};
    return BackendMessage(
      id: (json['id'] ?? '').toString(),
      event: (json['event'] ?? 'unknown').toString(),
      data: rawData is Map<String, dynamic>
          ? rawData
          : const <String, dynamic>{},
      errorCode: error['code']?.toString(),
      errorMessage: error['message']?.toString(),
    );
  }
}

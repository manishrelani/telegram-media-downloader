import 'chat.dart';
import 'media_item.dart';

class DownloadProgress {
  const DownloadProgress({
    this.fileName,
    this.percent,
    this.bytesDownloaded,
    this.totalBytes,
    this.speedBytesPerSecond,
    this.etaSeconds,
    this.itemKey,
    this.updatedAt,
  });

  final String? fileName;
  final double? percent;
  final int? bytesDownloaded;
  final int? totalBytes;
  final int? speedBytesPerSecond;
  final int? etaSeconds;
  final String? itemKey;
  final int? updatedAt;

  factory DownloadProgress.fromJson(Map<String, dynamic> json) {
    double? asDouble(dynamic value) => value is num ? value.toDouble() : null;
    int? asInt(dynamic value) => value is num ? value.toInt() : null;
    return DownloadProgress(
      fileName: json['fileName']?.toString(),
      percent: asDouble(json['percent']),
      bytesDownloaded: asInt(json['bytesDownloaded']),
      totalBytes: asInt(json['totalBytes']),
      speedBytesPerSecond: asInt(json['speedBytesPerSecond']),
      etaSeconds: asInt(json['etaSeconds']),
      itemKey: json['itemKey']?.toString(),
      updatedAt: asInt(json['updatedAt']),
    );
  }

  DownloadProgress merge(DownloadProgress? previous) {
    if (previous == null ||
        (itemKey != null && itemKey != previous.itemKey) ||
        (itemKey == null &&
            fileName != null &&
            previous.fileName != null &&
            fileName != previous.fileName)) {
      return this;
    }
    return DownloadProgress(
      fileName: fileName ?? previous.fileName,
      itemKey: itemKey ?? previous.itemKey,
      percent: percent ?? previous.percent,
      bytesDownloaded: bytesDownloaded ?? previous.bytesDownloaded,
      totalBytes: totalBytes ?? previous.totalBytes,
      speedBytesPerSecond: speedBytesPerSecond,
      etaSeconds: etaSeconds,
      updatedAt: updatedAt ?? previous.updatedAt,
    );
  }
}

class DownloadJob {
  const DownloadJob({
    required this.id,
    required this.chat,
    required this.state,
    required this.destination,
    required this.filter,
    this.errorCode,
    this.errorMessage,
    this.progress,
    this.message,
    this.takeout = false,
    this.groupMedia = false,
    this.skipSame = true,
    this.library = false,
    this.destinationRoot,
    this.topic,
    this.fileTypes = const <String>[],
    this.files = const <FileTask>[],
  });

  final String id;
  final TelegramChat chat;
  final String state;
  final String destination;
  final String filter;
  final String? errorCode;
  final String? errorMessage;
  final DownloadProgress? progress;
  final String? message;
  final bool takeout;
  final bool groupMedia;
  final bool skipSame;
  final bool library;
  final String? destinationRoot;
  final TelegramTopic? topic;
  final List<String> fileTypes;
  final List<FileTask> files;
  int get completedFiles =>
      files.where((FileTask file) => file.state == 'completed').length;
  int get failedFiles =>
      files.where((FileTask file) => file.state == 'failed').length;
  double? get overallPercent {
    if (state == 'completed') return 100;
    if (files.isEmpty) return null;
    double total = 0;
    for (final FileTask file in files) {
      if (file.state == 'completed') {
        total += 100;
      } else if (<String>{'downloading', 'finalizing'}.contains(file.state)) {
        if (file.progress?.percent == null) return null;
        total += file.progress!.percent!.clamp(0, 100);
      }
    }
    return total / files.length;
  }

  bool get isActive => const {
    'queued',
    'preparing',
    'downloading',
    'pausing',
    'cancelling',
  }.contains(state);
  bool get canPause => state == 'downloading' || state == 'preparing';
  bool get canResume => state == 'paused';
  bool get canCancel => isActive || state == 'paused';
  bool get isTerminal =>
      const {'completed', 'failed', 'cancelled'}.contains(state);

  factory DownloadJob.fromJson(Map<String, dynamic> json) {
    final dynamic chatValue = json['chat'];
    final TelegramChat chat = chatValue is Map<String, dynamic>
        ? TelegramChat.fromJson(chatValue)
        : TelegramChat(id: 0, type: 'unknown', name: 'Unknown chat');
    return DownloadJob(
      id: (json['id'] ?? '').toString(),
      chat: chat,
      state: (json['state'] ?? 'unknown').toString(),
      destination: (json['destination'] ?? '').toString(),
      filter: (json['filter'] ?? 'videos_and_documents').toString(),
      errorCode: json['errorCode']?.toString(),
      errorMessage: json['errorMessage']?.toString(),
      takeout: json['takeout'] == true,
      groupMedia: json['groupMedia'] == true,
      skipSame: json['skipSame'] != false,
      library: json['library'] == true,
      destinationRoot: json['destinationRoot']?.toString(),
      topic: json['topic'] is Map<String, dynamic>
          ? TelegramTopic.fromJson(json['topic'] as Map<String, dynamic>)
          : null,
      fileTypes: (json['fileTypes'] as List<dynamic>? ?? <dynamic>[])
          .map((dynamic value) => value.toString())
          .toList(),
      files:
          (json['files'] as List<dynamic>? ??
                  (json['items'] as List<dynamic>? ?? <dynamic>[])
                      .whereType<Map<String, dynamic>>()
                      .map(
                        (Map<String, dynamic> item) => <String, dynamic>{
                          'item': item,
                          'state': 'legacy',
                        },
                      )
                      .toList())
              .whereType<Map<String, dynamic>>()
              .map(FileTask.fromJson)
              .toList(),
    );
  }

  DownloadJob copyWith({
    String? state,
    String? errorCode,
    String? errorMessage,
    DownloadProgress? progress,
    String? message,
    bool clearError = false,
    bool? takeout,
    bool? groupMedia,
    bool? skipSame,
    List<FileTask>? files,
  }) => DownloadJob(
    id: id,
    chat: chat,
    state: state ?? this.state,
    destination: destination,
    filter: filter,
    errorCode: clearError ? null : (errorCode ?? this.errorCode),
    errorMessage: clearError ? null : (errorMessage ?? this.errorMessage),
    progress: progress ?? this.progress,
    message: message ?? this.message,
    takeout: takeout ?? this.takeout,
    groupMedia: groupMedia ?? this.groupMedia,
    skipSame: skipSame ?? this.skipSame,
    library: library,
    destinationRoot: destinationRoot,
    topic: topic,
    fileTypes: fileTypes,
    files: files ?? this.files,
  );
}

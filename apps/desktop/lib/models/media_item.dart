import 'download_job.dart';

const Map<String, String> fileTypeLabels = <String, String>{
  'video': 'Videos',
  'pdf': 'PDFs',
  'image': 'Images',
  'audio': 'Audio',
  'office': 'Office',
  'text': 'Text',
  'ebook': 'eBooks',
  'archive': 'Archives',
  'code': 'Code',
  'other': 'Other',
};

class MediaItem {
  const MediaItem({
    required this.chatId,
    required this.messageId,
    required this.originalName,
    this.caption = '',
    this.sizeBytes = 0,
    this.date = 0,
    this.durationSeconds = 0,
    this.topicId = 0,
    this.thumbnail,
    this.thumbnailVersion = '',
    this.hasSpoiler = false,
  });
  final int chatId;
  final int messageId;
  final String originalName;
  final String caption;
  final int sizeBytes;
  final int date;
  final double durationSeconds;
  final int topicId;
  final String? thumbnail;
  final String thumbnailVersion;
  final bool hasSpoiler;
  String get key => '$chatId:$messageId:$originalName';
  String get title => caption.trim().isEmpty
      ? originalName
      : caption.replaceAll(RegExp(r'\s+'), ' ').trim();
  String get extension => originalName.contains('.')
      ? originalName.split('.').last.toLowerCase()
      : '';
  String get fileType {
    const Map<String, String> groups = <String, String>{
      'code':
          'go py js ts tsx jsx dart java c h cpp cs rs swift kt html css scss json yaml yml xml sql sh ipynb tex',
      'video': 'mp4 mov mkv m4v webm avi 3gp mpeg mpg m2v mts m2ts ogv wmv flv',
      'image': 'jpg jpeg png webp gif heic heif avif bmp tif tiff',
      'audio': 'mp3 m4a aac ogg opus wav flac alac aiff amr wma',
      'pdf': 'pdf',
      'office': 'doc docx xls xlsx ppt pptx odt ods odp pages numbers key rtf',
      'text': 'txt md csv tsv log',
      'ebook': 'epub mobi azw azw3 djvu',
      'archive': 'zip rar 7z tar gz bz2 xz tgz zst',
    };
    for (final MapEntry<String, String> group in groups.entries) {
      if (group.value.split(' ').contains(extension)) return group.key;
    }
    return 'other';
  }

  MediaItem withThumbnail(String image) => MediaItem(
    chatId: chatId,
    messageId: messageId,
    originalName: originalName,
    caption: caption,
    sizeBytes: sizeBytes,
    date: date,
    durationSeconds: durationSeconds,
    topicId: topicId,
    thumbnail: image,
    hasSpoiler: hasSpoiler,
    thumbnailVersion: thumbnailVersion,
  );
  factory MediaItem.fromJson(Map<String, dynamic> json) => MediaItem(
    chatId: (json['chatId'] as num?)?.toInt() ?? 0,
    messageId: (json['messageId'] as num?)?.toInt() ?? 0,
    originalName: (json['originalName'] ?? '').toString(),
    caption: (json['caption'] ?? '').toString(),
    sizeBytes: (json['sizeBytes'] as num?)?.toInt() ?? 0,
    date: (json['date'] as num?)?.toInt() ?? 0,
    durationSeconds: (json['durationSeconds'] as num?)?.toDouble() ?? 0,
    topicId: (json['topicId'] as num?)?.toInt() ?? 0,
    thumbnail: json['thumbnail'] as String?,
    thumbnailVersion: (json['thumbnailVersion'] ?? '').toString(),
    hasSpoiler: json['hasSpoiler'] == true,
  );
}

class FileTask {
  const FileTask({
    required this.item,
    required this.state,
    this.progress,
    this.savedPath,
    this.error,
    this.availability,
  });
  final MediaItem item;
  final String state;
  final DownloadProgress? progress;
  final String? savedPath;
  final String? error;
  final String? availability;
  factory FileTask.fromJson(Map<String, dynamic> json) => FileTask(
    item: MediaItem.fromJson(json['item'] as Map<String, dynamic>),
    state: (json['state'] ?? 'queued').toString(),
    progress: json['progress'] is Map<String, dynamic>
        ? DownloadProgress.fromJson(json['progress'] as Map<String, dynamic>)
        : null,
    savedPath: json['savedPath']?.toString(),
    error: json['error']?.toString(),
    availability: json['availability']?.toString(),
  );
  FileTask withThumbnail(String image) => FileTask(
    item: item.withThumbnail(image),
    state: state,
    progress: progress,
    savedPath: savedPath,
    error: error,
    availability: availability,
  );
  FileTask withProgress(DownloadProgress update) => FileTask(
    item: item,
    state: 'downloading',
    progress: update.merge(progress),
    savedPath: savedPath,
    error: error,
    availability: availability,
  );
}

class MediaPage {
  const MediaPage({
    required this.items,
    required this.snapshotId,
    required this.hasMore,
    this.cached = false,
    this.loadedAt = 0,
    this.limit = 100,
  });
  final List<MediaItem> items;
  final String snapshotId;
  final bool hasMore;
  final bool cached;
  final int loadedAt;
  final int limit;
  factory MediaPage.fromJson(Map<String, dynamic> json, int limit) => MediaPage(
    items: (json['items'] as List<dynamic>? ?? <dynamic>[])
        .whereType<Map<String, dynamic>>()
        .map(MediaItem.fromJson)
        .toList(),
    snapshotId: (json['snapshotId'] ?? '').toString(),
    hasMore: json['hasMore'] == true,
    cached: json['cached'] == true,
    loadedAt: (json['loadedAt'] as num?)?.toInt() ?? 0,
    limit: limit,
  );
}

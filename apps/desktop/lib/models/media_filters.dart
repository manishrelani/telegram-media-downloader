import 'package:flutter/foundation.dart';

import 'chat.dart';
import 'download_job.dart';
import 'media_item.dart';

const statusFilterLabels = <String, String>{
  'all': 'All files',
  'in_progress': 'In progress',
  'not_downloaded': 'Not downloaded yet',
  'queued': 'Queued',
  'downloading': 'Downloading',
  'paused': 'Paused',
  'completed': 'Downloaded',
  'attention': 'Needs attention',
  'failed': 'Failed',
  'cancelled': 'Cancelled',
  'missing': 'Missing file',
  'unavailable': 'Location unavailable',
  'unknown': 'Unknown',
};
const sizeFilterLabels = <String, String>{
  'any': 'Any size',
  'under_10': 'Under 10 MB',
  '10_100': '10–100 MB',
  '100_1000': '100 MB–1 GB',
  'over_1000': '1 GB and above',
  'unknown': 'Unknown size',
};
const dateFilterLabels = <String, String>{
  'any': 'Any time',
  'today': 'Today',
  'week': 'Last 7 days',
  'month': 'Last 30 days',
  'custom': 'Custom range',
};
const sortLabels = <String, String>{
  'active': 'Active first',
  'downloaded': 'Downloaded first',
  'not_downloaded': 'Not downloaded first',
  'newest': 'Newest posted',
  'oldest': 'Oldest posted',
  'name': 'Name A–Z',
  'name_desc': 'Name Z–A',
  'size': 'Largest first',
  'smallest': 'Smallest first',
};
const groupLabels = <String, String>{
  'chat': 'Chat → Topic',
  'type': 'File type',
  'status': 'Download status',
  'none': 'No grouping',
};

class MediaEntry {
  const MediaEntry({required this.item, this.task, this.job, this.chat});
  final MediaItem item;
  final FileTask? task;
  final DownloadJob? job;
  final TelegramChat? chat;
  String get chatName => job?.chat.name ?? chat?.name ?? 'Chat ${item.chatId}';
  int get topicId => job?.topic?.id ?? item.topicId;
  String get topicKey => '${item.chatId}:$topicId';
  String get topicName {
    if (job?.topic != null) return job!.topic!.title;
    for (final topic in chat?.topics ?? <TelegramTopic>[]) {
      if (topic.id == topicId) return topic.title;
    }
    return topicId == 0 ? 'All media' : 'Topic $topicId';
  }

  String get status {
    final file = task;
    if (file == null) return 'not_downloaded';
    if (file.state == 'completed') {
      if (file.availability == 'missing') return 'missing';
      if (file.availability == 'unavailable') return 'unavailable';
      return file.savedPath?.isNotEmpty == true ? 'completed' : 'unknown';
    }
    if (file.state == 'legacy' || file.state == 'unknown') return 'unknown';
    if (const {
      'queued',
      'downloading',
      'finalizing',
      'paused',
      'preparing',
    }.contains(file.state)) {
      if (const {
        'paused',
        'pausing',
        'failed',
        'cancelled',
        'cancelling',
      }.contains(job?.state)) {
        return switch (job!.state) {
          'pausing' => 'paused',
          'cancelling' => 'cancelled',
          final s => s,
        };
      }
    }
    return switch (file.state) {
      'finalizing' => 'downloading',
      'preparing' => 'queued',
      final s => statusFilterLabels.containsKey(s) ? s : 'unknown',
    };
  }

  bool get downloaded => status == 'completed';
}

class MediaGroup {
  MediaGroup(this.key, this.label, this.entries);
  final String key, label;
  final List<MediaEntry> entries;
}

class MediaFilters extends ChangeNotifier {
  MediaFilters({this.sort = 'newest', this.groupBy = 'none'});
  String status = 'all', size = 'any', date = 'any', query = '';
  String sort, groupBy;
  Set<String> types = fileTypeLabels.keys.toSet();
  Set<int> chatIds = <int>{};
  Set<String> topicKeys = <String>{};
  DateTime? from, to;
  void change(VoidCallback action) {
    action();
    notifyListeners();
  }

  void reset() {
    change(() {
      status = 'all';
      size = 'any';
      date = 'any';
      query = '';
      types = fileTypeLabels.keys.toSet();
      chatIds.clear();
      topicKeys.clear();
      from = null;
      to = null;
    });
  }

  bool get narrowed =>
      status != 'all' ||
      size != 'any' ||
      date != 'any' ||
      query.trim().isNotEmpty ||
      types.length != fileTypeLabels.length ||
      chatIds.isNotEmpty ||
      topicKeys.isNotEmpty;
  bool matchesStatus(MediaEntry e) => switch (status) {
    'all' => true,
    'in_progress' => const {'queued', 'downloading'}.contains(e.status),
    'not_downloaded' => !const {
      'completed',
      'unknown',
      'unavailable',
    }.contains(e.status),
    'attention' => const {
      'failed',
      'missing',
      'unavailable',
    }.contains(e.status),
    final s => e.status == s,
  };
  bool _matchesSize(int bytes) => switch (size) {
    'any' => true,
    'unknown' => bytes <= 0,
    'under_10' => bytes > 0 && bytes < 10000000,
    '10_100' => bytes >= 10000000 && bytes < 100000000,
    '100_1000' => bytes >= 100000000 && bytes < 1000000000,
    'over_1000' => bytes >= 1000000000,
    _ => true,
  };
  bool _matchesDate(int stamp, DateTime now) {
    if (date == 'any') return true;
    if (stamp <= 0) return false;
    final posted = DateTime.fromMillisecondsSinceEpoch(stamp * 1000);
    final today = DateTime(now.year, now.month, now.day);
    DateTime? start, end;
    if (date == 'custom') {
      if (from == null || to == null) return false;
      start = DateTime(from!.year, from!.month, from!.day);
      end = DateTime(to!.year, to!.month, to!.day + 1);
    } else {
      final days = date == 'week'
          ? 6
          : date == 'month'
          ? 29
          : 0;
      start = DateTime(today.year, today.month, today.day - days);
      end = DateTime(today.year, today.month, today.day + 1);
    }
    return !posted.isBefore(start) && posted.isBefore(end);
  }

  List<MediaEntry> apply(Iterable<MediaEntry> input, {DateTime? now}) {
    final needle = query.trim().toLowerCase();
    final at = now ?? DateTime.now();
    final result = input
        .where(
          (e) =>
              types.contains(e.item.fileType) &&
              matchesStatus(e) &&
              (chatIds.isEmpty || chatIds.contains(e.item.chatId)) &&
              (topicKeys.isEmpty || topicKeys.contains(e.topicKey)) &&
              _matchesSize(e.item.sizeBytes) &&
              _matchesDate(e.item.date, at) &&
              (needle.isEmpty ||
                  '${e.item.title} ${e.item.originalName} ${e.chatName} ${e.topicName}'
                      .toLowerCase()
                      .contains(needle)),
        )
        .toList();
    result.sort((a, b) {
      int order = 0;
      if (sort == 'name' || sort == 'name_desc') {
        order =
            a.item.title.toLowerCase().compareTo(b.item.title.toLowerCase()) *
            (sort == 'name' ? 1 : -1);
      } else if (sort == 'size' || sort == 'smallest') {
        if ((a.item.sizeBytes <= 0) != (b.item.sizeBytes <= 0)) {
          return a.item.sizeBytes <= 0 ? 1 : -1;
        }
        order =
            a.item.sizeBytes.compareTo(b.item.sizeBytes) *
            (sort == 'smallest' ? 1 : -1);
      } else if (sort == 'downloaded') {
        order = (a.downloaded ? 0 : 1).compareTo(b.downloaded ? 0 : 1);
      } else if (sort == 'not_downloaded') {
        order = (a.downloaded ? 1 : 0).compareTo(b.downloaded ? 1 : 0);
      } else if (sort == 'active') {
        int rank(MediaEntry e) => switch (e.status) {
          'downloading' => 0,
          'queued' => 1,
          'paused' => 2,
          'failed' || 'missing' || 'unavailable' => 3,
          'completed' => 5,
          _ => 4,
        };
        order = rank(a).compareTo(rank(b));
      } else {
        if ((a.item.date <= 0) != (b.item.date <= 0)) {
          return a.item.date <= 0 ? 1 : -1;
        }
        order =
            a.item.date.compareTo(b.item.date) * (sort == 'oldest' ? 1 : -1);
      }
      if (order != 0) return order;
      order = b.item.messageId.compareTo(a.item.messageId);
      if (order != 0) return order;
      return '${a.item.key}:${a.job?.id}'.compareTo(
        '${b.item.key}:${b.job?.id}',
      );
    });
    return result;
  }

  List<MediaGroup> groups(List<MediaEntry> entries) {
    final result = <String, MediaGroup>{};
    for (final e in entries) {
      final key = switch (groupBy) {
        'chat' => e.topicKey,
        'type' => e.item.fileType,
        'status' => e.status,
        _ => 'all',
      };
      final label = switch (groupBy) {
        'chat' => '${e.chatName} · ${e.topicName}',
        'type' => fileTypeLabels[e.item.fileType] ?? 'Other',
        'status' => statusFilterLabels[e.status] ?? 'Unknown',
        _ => '',
      };
      result.putIfAbsent(key, () => MediaGroup(key, label, [])).entries.add(e);
    }
    return result.values.toList();
  }
}

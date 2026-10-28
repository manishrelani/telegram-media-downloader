import 'package:flutter_test/flutter_test.dart';
import 'package:telegram_media_downloader/models/media_filters.dart';
import 'package:telegram_media_downloader/models/media_item.dart';
import 'package:telegram_media_downloader/models/download_job.dart';
import 'package:telegram_media_downloader/models/chat.dart';

MediaEntry entry(
  int id, {
  String name = 'file.mp4',
  String? state,
  String? availability,
  int size = 100,
  int date = 0,
  int chat = 1,
  int topic = 1,
}) {
  final item = MediaItem(
    chatId: chat,
    messageId: id,
    originalName: name,
    sizeBytes: size,
    date: date,
    topicId: topic,
  );
  final job = DownloadJob(
    id: 'job-$id',
    chat: TelegramChat(id: chat, type: 'group', name: 'Same name'),
    state: state ?? 'queued',
    destination: '/tmp',
    filter: 'all_attachments',
    topic: TelegramTopic(id: topic, title: 'Same topic'),
  );
  return MediaEntry(
    item: item,
    job: job,
    task: state == null
        ? null
        : FileTask(
            item: item,
            state: state,
            savedPath: state == 'completed' ? '/tmp/$id' : null,
            availability: availability,
          ),
  );
}

void main() {
  test(
    'status distinguishes never queued, saved, missing, offline, and legacy',
    () {
      final f = MediaFilters();
      final rows = [
        entry(1),
        entry(2, state: 'completed', availability: 'available'),
        entry(3, state: 'completed', availability: 'missing'),
        entry(4, state: 'completed', availability: 'unavailable'),
        entry(5, state: 'legacy'),
        entry(6, state: 'failed'),
        entry(7, state: 'downloading'),
      ];
      f.status = 'not_downloaded';
      expect(f.apply(rows).map((e) => e.item.messageId), [7, 6, 3, 1]);
      f.status = 'completed';
      expect(f.apply(rows).map((e) => e.item.messageId), [2]);
      f.status = 'attention';
      expect(f.apply(rows).map((e) => e.item.messageId), [6, 4, 3]);
      f.status = 'unknown';
      expect(f.apply(rows).single.item.messageId, 5);
    },
  );
  test(
    'types combine with chat, topic, search and non-overlapping size boundaries',
    () {
      final f = MediaFilters()
        ..types = {'pdf'}
        ..chatIds = {2}
        ..topicKeys = {'2:7'}
        ..query = 'NOTES'
        ..size = '10_100';
      final rows = [
        entry(1, name: 'notes.PDF', size: 10000000, chat: 2, topic: 7),
        entry(2, name: 'notes.pdf', size: 100000000, chat: 2, topic: 7),
        entry(3, name: 'notes.pdf', size: 10000000, chat: 3, topic: 7),
        entry(4, name: 'notes.pdf', size: 0, chat: 2, topic: 7),
      ];
      expect(f.apply(rows).single.item.messageId, 1);
      f.types = {};
      expect(f.apply(rows), isEmpty);
    },
  );
  test(
    'date ranges use local days, include custom end day, exclude unknown dates',
    () {
      final now = DateTime(2026, 10, 27, 14);
      int stamp(DateTime d) => d.millisecondsSinceEpoch ~/ 1000;
      final rows = [
        entry(1, date: stamp(DateTime(2026, 10, 27, 0))),
        entry(2, date: stamp(DateTime(2026, 10, 26, 23, 59))),
        entry(3, date: stamp(DateTime(2026, 10, 27, 23, 59))),
        entry(4, date: 0),
        entry(5, date: stamp(DateTime(2026, 10, 28))),
      ];
      final f = MediaFilters()..date = 'today';
      expect(f.apply(rows, now: now).map((e) => e.item.messageId), [3, 1]);
      f.date = 'custom';
      f.from = DateTime(2026, 10, 26);
      f.to = DateTime(2026, 10, 27);
      expect(f.apply(rows, now: now).map((e) => e.item.messageId), [3, 1, 2]);
    },
  );
  test('unknown sizes sort last in both directions', () {
    final f = MediaFilters()..sort = 'smallest';
    final rows = [entry(1, size: 0), entry(2, size: 100), entry(3, size: 50)];
    expect(f.apply(rows).map((e) => e.item.messageId), [3, 2, 1]);
    f.sort = 'size';
    expect(f.apply(rows).map((e) => e.item.messageId), [2, 3, 1]);
  });
  test('grouping keeps identical chat and topic names separate by ID', () {
    final f = MediaFilters()..groupBy = 'chat';
    final groups = f.groups([entry(1, chat: 1), entry(2, chat: 2)]);
    expect(groups, hasLength(2));
    expect(groups[0].key, isNot(groups[1].key));
  });
  test('parent state applies to unfinished files', () {
    final e = entry(1, state: 'queued');
    expect(
      MediaEntry(
        item: e.item,
        task: e.task,
        job: e.job!.copyWith(state: 'paused'),
      ).status,
      'paused',
    );
  });
  test('images and audio have separate types and code wins TS ambiguity', () {
    expect(entry(1, name: 'photo.HEIC').item.fileType, 'image');
    expect(entry(2, name: 'voice.ogg').item.fileType, 'audio');
    expect(entry(3, name: 'source.ts').item.fileType, 'code');
  });
  test(
    'every size boundary belongs to exactly one bucket and unknown is separate',
    () {
      for (final pair in <int, String>{
        0: 'unknown',
        -1: 'unknown',
        1: 'under_10',
        9999999: 'under_10',
        10000000: '10_100',
        99999999: '10_100',
        100000000: '100_1000',
        999999999: '100_1000',
        1000000000: 'over_1000',
      }.entries) {
        final row = entry(1, size: pair.key);
        final matched = <String>[];
        for (final size in sizeFilterLabels.keys.where((key) => key != 'any')) {
          final f = MediaFilters()..size = size;
          if (f.apply([row]).isNotEmpty) matched.add(size);
        }
        expect(matched, [pair.value]);
      }
    },
  );
  test('relative ranges and reversed custom ranges handle day boundaries', () {
    final now = DateTime(2026, 10, 27);
    int seconds(DateTime value) => value.millisecondsSinceEpoch ~/ 1000;
    final f = MediaFilters()..date = 'week';
    final rows = [
      entry(1, date: seconds(DateTime(2026, 10, 21))),
      entry(2, date: seconds(DateTime(2026, 10, 20, 23, 59))),
    ];
    expect(f.apply(rows, now: now).map((e) => e.item.messageId), [1]);
    f.date = 'custom';
    f.from = DateTime(2026, 10, 28);
    f.to = DateTime(2026, 10, 27);
    expect(f.apply(rows, now: now), isEmpty);
  });
  test('type, status and flat grouping preserve every matching entry', () {
    final rows = [
      entry(1),
      entry(2, name: 'a.pdf', state: 'completed', availability: 'available'),
      entry(3, state: 'failed'),
    ];
    for (final grouping in ['type', 'status', 'none']) {
      final f = MediaFilters()..groupBy = grouping;
      final groups = f.groups(rows);
      expect(groups.expand((g) => g.entries), hasLength(3));
      if (grouping == 'none') expect(groups, hasLength(1));
    }
  });
}

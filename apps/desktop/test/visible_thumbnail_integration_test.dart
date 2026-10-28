import 'dart:async';
import 'dart:convert';
import 'dart:io';
import 'package:flutter/material.dart';
import 'package:flutter/rendering.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:telegram_media_downloader/models/backend_message.dart';
import 'package:telegram_media_downloader/models/media_item.dart';
import 'package:telegram_media_downloader/ui/video_thumbnail.dart';
import 'package:telegram_media_downloader/ui/quiet_theme.dart';
import 'library_test.dart' show LibraryBackend, readyController;

class ViewportBackend extends LibraryBackend {
  final requests = <Map<String, dynamic>>[];
  final cancelled = <String>[];
  final pendingPreviews = <String, Completer<BackendMessage>>{};
  int generation = 0;
  @override
  Future<BackendMessage> request(
    String action, {
    Map<String, dynamic>? params,
    String? requestId,
    Duration timeout = const Duration(seconds: 30),
  }) async {
    if (action == 'list_media') {
      return BackendMessage(
        id: requestId!,
        event: 'result',
        data: {
          'snapshotId': 'page-${++generation}',
          'items': [
            for (int i = 0; i < 30; i++)
              {
                'chatId': 123,
                'messageId': i + 1,
                'originalName': 'video-$i.mp4',
                'thumbnailVersion': 'v1',
              },
          ],
        },
      );
    }
    if (action == 'request_thumbnails') {
      requests.add({'id': requestId, ...params!});
      final completer = Completer<BackendMessage>();
      pendingPreviews[requestId!] = completer;
      return completer.future;
    }
    if (action == 'cancel_media_request') {
      final id = params!['requestId'] as String;
      cancelled.add(id);
      pendingPreviews
          .remove(id)
          ?.complete(
            BackendMessage(id: id, event: 'result', data: {'cancelled': true}),
          );
    }
    return super.request(
      action,
      params: params,
      requestId: requestId,
      timeout: timeout,
    );
  }

  void finish() {
    for (final entry in pendingPreviews.entries) {
      if (!entry.value.isCompleted) {
        entry.value.complete(BackendMessage(id: entry.key, event: 'result'));
      }
    }
    pendingPreviews.clear();
  }
}

void main() {
  testWidgets(
    'visible videos request by screen order and streamed image renders before batch completes',
    (tester) async {
      tester.view.physicalSize = const Size(800, 300);
      tester.view.devicePixelRatio = 1;
      addTearDown(tester.view.resetPhysicalSize);
      addTearDown(tester.view.resetDevicePixelRatio);
      final backend = ViewportBackend();
      final manager = await readyController(backend);
      final scroll = ScrollController();
      await tester.pumpWidget(
        MaterialApp(
          theme: QuietTheme.light,
          home: AnimatedBuilder(
            animation: manager,
            builder: (_, _) => ListView.builder(
              controller: scroll,
              scrollCacheExtent: ScrollCacheExtent.pixels(1200),
              itemExtent: 100,
              itemCount: manager.mediaPage!.items.length,
              itemBuilder: (_, i) => Align(
                alignment: Alignment.centerLeft,
                child: VideoThumbnail(
                  key: ValueKey(i),
                  item: manager.mediaPage!.items[i],
                  cache: manager.thumbnailCache,
                  onVisibilityChanged: manager.reportThumbnailVisibility,
                ),
              ),
            ),
          ),
        ),
      );
      await tester.pump();
      await tester.pump(const Duration(milliseconds: 80));
      expect(backend.requests, hasLength(1));
      expect(backend.requests.single['keys'], [
        '123:1:video-0.mp4',
        '123:2:video-1.mp4',
        '123:3:video-2.mp4',
      ]);
      final image = base64Encode(
        File('test/fixtures/video-preview.jpg').readAsBytesSync(),
      );
      backend.messages.add(
        BackendMessage(
          id: backend.requests.single['id'] as String,
          event: 'media_thumbnails',
          data: {
            'snapshotId': manager.mediaPage!.snapshotId,
            'thumbnails': {'123:1:video-0.mp4': image},
          },
        ),
      );
      await tester.pumpAndSettle();
      expect(find.byType(Image), findsOneWidget);
      expect(backend.pendingPreviews, hasLength(1));
      expect(backend.cancelled, isEmpty);
      scroll.jumpTo(1000);
      await tester.pumpAndSettle();
      await tester.pump(const Duration(milliseconds: 80));
      expect(backend.cancelled, contains(backend.requests.first['id']));
      expect(backend.requests, hasLength(2));
      expect(backend.requests.last['keys'], [
        '123:11:video-10.mp4',
        '123:12:video-11.mp4',
        '123:13:video-12.mp4',
      ]);
      await tester.pumpWidget(const SizedBox.shrink());
      backend.finish();
      manager.dispose();
      scroll.dispose();
      await tester.pump();
    },
  );
  testWidgets(
    'scope changes cancel previews; spoilers, documents, sharp images and unknown items never request',
    (tester) async {
      final backend = ViewportBackend();
      final manager = await readyController(backend);
      final item = manager.mediaPage!.items.first;
      manager.reportThumbnailVisibility('a', item, true, 0);
      await tester.pump(const Duration(milliseconds: 80));
      expect(backend.requests, hasLength(1));
      await manager.openChat(
        manager.chats.single,
        topic: manager.chats.single.topics.last,
      );
      await tester.pump();
      expect(backend.cancelled, hasLength(1));
      final sharp = item.withThumbnail(
        base64Encode(File('test/fixtures/video-preview.jpg').readAsBytesSync()),
      );
      manager.reportThumbnailVisibility('a', sharp, true, 0);
      manager.reportThumbnailVisibility(
        'b',
        const MediaItem(
          chatId: 123,
          messageId: 2,
          originalName: 'secret.mp4',
          hasSpoiler: true,
        ),
        true,
        1,
      );
      manager.reportThumbnailVisibility(
        'c',
        const MediaItem(chatId: 123, messageId: 3, originalName: 'notes.pdf'),
        true,
        2,
      );
      manager.reportThumbnailVisibility(
        'd',
        const MediaItem(chatId: 999, messageId: 3, originalName: 'unknown.mp4'),
        true,
        3,
      );
      await tester.pump(const Duration(seconds: 3));
      expect(backend.requests, hasLength(1));
      backend.messages.add(
        const BackendMessage(
          id: 'thumb-example',
          event: 'error',
          errorCode: 'thumbnail_unavailable',
          errorMessage: 'offline',
        ),
      );
      await tester.pump();
      expect(manager.errorMessage, isNull);
      backend.finish();
      manager.dispose();
      await tester.pump();
    },
  );
}

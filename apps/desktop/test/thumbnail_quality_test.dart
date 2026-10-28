import 'dart:convert';
import 'dart:io';
import 'package:flutter/material.dart';
import 'package:telegram_media_downloader/ui/quiet_theme.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:telegram_media_downloader/models/backend_message.dart';
import 'package:telegram_media_downloader/models/media_item.dart';
import 'package:telegram_media_downloader/services/thumbnail_cache.dart';
import 'package:telegram_media_downloader/ui/video_thumbnail.dart';
import 'library_test.dart' show LibraryBackend, readyController;

void main() {
  test(
    'thumbnail upgrades respect snapshot identity and preserve selected files and progress',
    () async {
      final backend = LibraryBackend();
      final controller = await readyController(backend);
      final item = controller.mediaPage!.items.first;
      controller.toggleMedia(item, true);
      final image = base64Encode([
        ...File('test/fixtures/video-preview.jpg').readAsBytesSync(),
        0,
      ]);
      backend.messages.add(
        BackendMessage(
          id: 'preview',
          event: 'media_thumbnails',
          data: {
            'snapshotId': 'stale',
            'thumbnails': {item.key: 'bad'},
          },
        ),
      );
      await Future<void>.delayed(Duration.zero);
      expect(controller.mediaPage!.items.first.thumbnail, isNot('bad'));
      backend.messages.add(
        BackendMessage(
          id: 'preview',
          event: 'media_thumbnails',
          data: {
            'snapshotId': controller.mediaPage!.snapshotId,
            'thumbnails': {item.key: image},
          },
        ),
      );
      await Future<void>.delayed(Duration.zero);
      expect(controller.mediaPage!.items.first.thumbnail, image);
      expect(controller.selectedMediaCount, 1);
      expect(controller.jobs.first.files.first.progress!.percent, 42.8);
      backend.messages.add(
        BackendMessage(
          id: 'saved-job',
          event: 'job_state',
          data: {'job': LibraryBackend.job()},
        ),
      );
      await Future<void>.delayed(Duration.zero);
      expect(controller.jobs.first.files.first.item.thumbnail, image);
      expect(controller.jobs.first.files.first.progress!.percent, 42.8);
      controller.dispose();
    },
  );
  testWidgets(
    'retina previews decode for the screen density instead of a fixed pixel width',
    (tester) async {
      tester.view.devicePixelRatio = 3;
      addTearDown(tester.view.resetDevicePixelRatio);
      await tester.pumpWidget(
        MaterialApp(
          theme: QuietTheme.light,
          home: Scaffold(
            body: VideoThumbnail(
              item: MediaItem(
                chatId: 1,
                messageId: 1,
                originalName: 'a.mp4',
                thumbnail: base64Encode(
                  File('test/fixtures/video-preview.jpg').readAsBytesSync(),
                ),
              ),
              cache: ThumbnailCache(memoryOnly: true),
            ),
          ),
        ),
      );
      await tester.pumpAndSettle();
      final widget = tester.widget<Image>(find.byType(Image));
      expect((widget.image as ResizeImage).width, 288);
      expect(widget.filterQuality, FilterQuality.high);
      expect(tester.takeException(), isNull);
    },
  );
  testWidgets(
    'tiny Telegram placeholders are not stretched into blurry covers',
    (tester) async {
      await tester.pumpWidget(
        MaterialApp(
          theme: QuietTheme.light,
          home: Scaffold(
            body: VideoThumbnail(
              item: MediaItem(
                chatId: 1,
                messageId: 1,
                originalName: 'a.mp4',
                durationSeconds: 65,
                thumbnail: base64Encode(
                  File(
                    'test/fixtures/low-resolution-preview.jpg',
                  ).readAsBytesSync(),
                ),
              ),
              cache: ThumbnailCache(memoryOnly: true),
            ),
          ),
        ),
      );
      await tester.pumpAndSettle();
      expect(find.byType(Image), findsNothing);
      expect(find.byIcon(Icons.play_circle_outline), findsOneWidget);
      expect(find.text('1:05'), findsOneWidget);
      expect(tester.takeException(), isNull);
    },
  );
}

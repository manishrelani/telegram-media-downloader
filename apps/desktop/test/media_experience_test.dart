import 'dart:convert';
import 'dart:io';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:telegram_media_downloader/main.dart';
import 'package:telegram_media_downloader/models/media_item.dart';
import 'package:telegram_media_downloader/services/thumbnail_cache.dart';
import 'package:telegram_media_downloader/ui/video_thumbnail.dart';
import 'package:telegram_media_downloader/ui/quiet_theme.dart';
import 'library_test.dart' show LibraryBackend, readyController;
import 'support/sidebar_width_store.dart';

const pixel =
    'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+aivQAAAAASUVORK5CYII=';
void main() {
  test(
    'thumbnail cache persists, rejects oversized payloads, and recovers corrupt files',
    () async {
      final directory = await Directory.systemTemp.createTemp('tmm-thumb-');
      addTearDown(() => directory.delete(recursive: true));
      final cache = ThumbnailCache(directory: directory);
      const item = MediaItem(
        chatId: 1,
        messageId: 1,
        originalName: 'video.mp4',
        thumbnail: pixel,
      );
      final data = await cache.load(item);
      expect(data, isNotNull);
      expect(
        await cache.load(
          const MediaItem(
            chatId: 1,
            messageId: 1,
            originalName: 'video.mp4',
            thumbnail: pixel,
            hasSpoiler: true,
          ),
        ),
        isNull,
      );
      final fresh = ThumbnailCache(directory: directory);
      expect(
        await fresh.load(
          const MediaItem(chatId: 1, messageId: 1, originalName: 'video.mp4'),
        ),
        orderedEquals(data!),
      );
      expect(
        await cache.load(
          MediaItem(
            chatId: 1,
            messageId: 2,
            originalName: 'b.mp4',
            thumbnail: base64Encode(List.filled(300000, 0)),
          ),
        ),
        isNull,
      );
      for (final file in directory.listSync().whereType<File>()) {
        await file.writeAsString('corrupt');
      }
      expect(
        await ThumbnailCache(directory: directory).load(
          const MediaItem(chatId: 1, messageId: 1, originalName: 'video.mp4'),
        ),
        isNull,
      );
    },
  );
  testWidgets(
    'missing and corrupt previews preserve geometry and duration without errors',
    (tester) async {
      for (final data in <String?>[
        null,
        'broken!',
        base64Encode([1, 2, 3]),
      ]) {
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
                  thumbnail: data,
                ),
                cache: ThumbnailCache(memoryOnly: true),
              ),
            ),
          ),
        );
        await tester.pumpAndSettle();
        expect(tester.getSize(find.byType(VideoThumbnail)), const Size(96, 54));
        expect(find.text('1:05'), findsOneWidget);
        expect(tester.takeException(), isNull);
      }
    },
  );
  testWidgets('download filters work per file and clear restores results', (
    tester,
  ) async {
    tester.view.physicalSize = const Size(1440, 900);
    tester.view.devicePixelRatio = 1;
    addTearDown(tester.view.resetPhysicalSize);
    addTearDown(tester.view.resetDevicePixelRatio);
    final controller = await readyController(LibraryBackend());
    await tester.pumpWidget(
      TelegramMediaDownloaderApp(
        autoConnect: false,
        controller: controller,
        sidebarWidthStore: FakeSidebarWidthStore(),
      ),
    );
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const ValueKey<String>('queue-toggle')));
    await tester.pumpAndSettle();
    await tester.tap(
      find.byKey(const ValueKey<String>('downloads-status-completed')),
    );
    await tester.pumpAndSettle();
    expect(
      find.byKey(
        const ValueKey<String>('queue-file-saved-job-123:30:api-design.mp4'),
      ),
      findsNothing,
    );
    expect(
      find.text('02. Authentication from first principles'),
      findsOneWidget,
    );
    await tester.tap(find.byKey(const ValueKey<String>('downloads-filters')));
    await tester.pumpAndSettle();
    expect(find.text('Filter downloads'), findsOneWidget);
    await tester.sendKeyEvent(LogicalKeyboardKey.escape);
    await tester.pumpAndSettle();
    controller.downloadFilters.reset();
    await tester.pumpAndSettle();
    expect(find.text('01. Designing a better API'), findsWidgets);
    expect(tester.takeException(), isNull);
    await tester.pumpWidget(const SizedBox.shrink());
    controller.dispose();
  });
  testWidgets(
    'filter dialog fits compact large text and selection survives status changes',
    (tester) async {
      tester.view.physicalSize = const Size(800, 650);
      tester.view.devicePixelRatio = 1;
      addTearDown(tester.view.resetPhysicalSize);
      addTearDown(tester.view.resetDevicePixelRatio);
      final controller = await readyController(LibraryBackend());
      controller.toggleMedia(controller.mediaPage!.items.last, true);
      await tester.pumpWidget(
        TelegramMediaDownloaderApp(
          autoConnect: false,
          controller: controller,
          textScaler: TextScaler.linear(1.5),
          sidebarWidthStore: FakeSidebarWidthStore(),
        ),
      );
      await tester.pumpAndSettle();
      await tester.tap(find.byKey(const ValueKey<String>('library-filters')));
      await tester.pumpAndSettle();
      expect(find.text('Filter library'), findsOneWidget);
      expect(tester.takeException(), isNull);
      await tester.sendKeyEvent(LogicalKeyboardKey.escape);
      await tester.pumpAndSettle();
      controller.libraryFilters.change(
        () => controller.libraryFilters.status = 'completed',
      );
      await tester.pumpAndSettle();
      expect(controller.selectedMediaCount, 1);
      expect(find.text('1 selected · 1 hidden by filters'), findsOneWidget);
      expect(tester.takeException(), isNull);
      await tester.pumpWidget(const SizedBox.shrink());
      controller.dispose();
    },
  );
  testWidgets(
    'Escape cancels draft filters and Apply changes the visible files',
    (tester) async {
      tester.view.physicalSize = const Size(1440, 900);
      tester.view.devicePixelRatio = 1;
      addTearDown(tester.view.resetPhysicalSize);
      addTearDown(tester.view.resetDevicePixelRatio);
      final controller = await readyController(LibraryBackend());
      await tester.pumpWidget(
        TelegramMediaDownloaderApp(
          autoConnect: false,
          controller: controller,
          sidebarWidthStore: FakeSidebarWidthStore(),
        ),
      );
      await tester.pumpAndSettle();
      Future<void> choosePDF() async {
        await tester.tap(find.byKey(const ValueKey<String>('library-filters')));
        await tester.pumpAndSettle();
        await tester.tap(find.widgetWithText(FilterChip, 'All files'));
        await tester.pumpAndSettle();
        await tester.tap(find.widgetWithText(FilterChip, 'PDFs'));
        await tester.pumpAndSettle();
      }

      await choosePDF();
      await tester.sendKeyEvent(LogicalKeyboardKey.escape);
      await tester.pumpAndSettle();
      expect(controller.visibleMedia, hasLength(5));
      await choosePDF();
      await tester.tap(find.widgetWithText(FilledButton, 'Show 1 file'));
      await tester.pumpAndSettle();
      expect(controller.visibleMedia.single.fileType, 'pdf');
      expect(tester.takeException(), isNull);
      await tester.pumpWidget(const SizedBox.shrink());
      controller.dispose();
    },
  );
  testWidgets('switching grouping reveals files from a collapsed batch', (
    tester,
  ) async {
    tester.view.physicalSize = const Size(1440, 1000);
    tester.view.devicePixelRatio = 1;
    addTearDown(tester.view.resetPhysicalSize);
    addTearDown(tester.view.resetDevicePixelRatio);
    final controller = await readyController(LibraryBackend());
    await tester.pumpWidget(
      TelegramMediaDownloaderApp(
        autoConnect: false,
        controller: controller,
        sidebarWidthStore: FakeSidebarWidthStore(),
      ),
    );
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const ValueKey<String>('queue-toggle')));
    await tester.pumpAndSettle();
    await tester.tap(find.byTooltip('Collapse file list'));
    await tester.pumpAndSettle();
    controller.downloadFilters.change(
      () => controller.downloadFilters.groupBy = 'none',
    );
    await tester.pumpAndSettle();
    expect(find.text('01. Designing a better API'), findsWidgets);
    expect(find.byTooltip('Batch download actions'), findsWidgets);
    expect(tester.takeException(), isNull);
    await tester.pumpWidget(const SizedBox.shrink());
    controller.dispose();
  });
  testWidgets(
    'a cached preview disappears immediately when a video becomes a spoiler',
    (tester) async {
      final cache = ThumbnailCache(memoryOnly: true);
      Future<void> show(bool spoiler) async {
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
                  hasSpoiler: spoiler,
                ),
                cache: cache,
              ),
            ),
          ),
        );
      }

      await show(false);
      await tester.pumpAndSettle();
      expect(find.byType(Image), findsOneWidget);
      expect(tester.takeException(), isNull);
      await show(true);
      expect(find.byType(Image), findsNothing);
      await tester.pumpAndSettle();
      expect(tester.takeException(), isNull);
    },
  );
  testWidgets(
    'cancelling a custom date picker restores the previous date choice',
    (tester) async {
      tester.view.physicalSize = const Size(1440, 900);
      tester.view.devicePixelRatio = 1;
      addTearDown(tester.view.resetPhysicalSize);
      addTearDown(tester.view.resetDevicePixelRatio);
      final controller = await readyController(LibraryBackend());
      await tester.pumpWidget(
        TelegramMediaDownloaderApp(
          autoConnect: false,
          controller: controller,
          sidebarWidthStore: FakeSidebarWidthStore(),
        ),
      );
      await tester.pumpAndSettle();
      await tester.tap(find.byKey(const ValueKey<String>('library-filters')));
      await tester.pumpAndSettle();
      final field = find.byWidgetPredicate(
        (widget) =>
            widget is DropdownButtonFormField<String> &&
            (widget.key?.toString().contains('Posted date:') ?? false),
      );
      await tester.ensureVisible(field);
      await tester.pumpAndSettle();
      await tester.tap(field);
      await tester.pumpAndSettle();
      await tester.tap(find.text('Custom range').last);
      await tester.pumpAndSettle();
      await tester.sendKeyEvent(LogicalKeyboardKey.escape);
      await tester.pumpAndSettle();
      final dropdown = tester.widget<DropdownButton<String>>(
        find.descendant(
          of: field,
          matching: find.byType(DropdownButton<String>),
        ),
      );
      expect(dropdown.value, 'any');
      expect(controller.libraryFilters.date, 'any');
      expect(tester.takeException(), isNull);
      await tester.pumpWidget(const SizedBox.shrink());
      controller.dispose();
    },
  );
}

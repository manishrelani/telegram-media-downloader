import 'dart:convert';
import 'dart:async';
import 'dart:io';
import 'dart:ui' as ui;

import 'package:flutter/material.dart';
import 'package:flutter/rendering.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:telegram_media_downloader/controllers/manager_controller.dart';
import 'package:telegram_media_downloader/main.dart';
import 'package:telegram_media_downloader/models/backend_message.dart';
import 'package:telegram_media_downloader/models/chat.dart';
import 'package:telegram_media_downloader/services/backend_client.dart';
import 'support/sidebar_width_store.dart';
import 'package:telegram_media_downloader/services/thumbnail_cache.dart';

class LibraryBackend extends BackendClient {
  final StreamController<BackendMessage> messages =
      StreamController<BackendMessage>.broadcast();
  final List<Map<String, dynamic>> calls = <Map<String, dynamic>>[];
  final List<Completer<BackendMessage>> pending = <Completer<BackendMessage>>[];
  bool deferMedia = false;
  @override
  Stream<BackendMessage> get events => messages.stream;
  static final Map<String, dynamic> chat = <String, dynamic>{
    'id': 123,
    'type': 'group',
    'name': 'Development Collective',
    'topics': <Map<String, dynamic>>[
      <String, dynamic>{'id': 1, 'title': 'General'},
      <String, dynamic>{'id': 42, 'title': 'Backend'},
      <String, dynamic>{'id': 43, 'title': 'Frontend'},
    ],
  };
  static final List<Map<String, dynamic>> items = <Map<String, dynamic>>[
    <String, dynamic>{
      'chatId': 123,
      'messageId': 30,
      'originalName': 'api-design.mp4',
      'thumbnail': base64Encode(
        File('test/fixtures/video-preview.jpg').readAsBytesSync(),
      ),
      'caption': '01. Designing a better API',
      'sizeBytes': 550000000,
      'date': 1793000000,
      'durationSeconds': 2520,
      'topicId': 42,
    },
    <String, dynamic>{
      'chatId': 123,
      'messageId': 29,
      'originalName': 'architecture.pdf',
      'caption': 'System architecture · Workshop notes',
      'sizeBytes': 2400000,
      'date': 1792900000,
      'topicId': 42,
    },
    <String, dynamic>{
      'chatId': 123,
      'messageId': 28,
      'originalName': 'authentication.mp4',
      'caption': '02. Authentication from first principles',
      'sizeBytes': 320000000,
      'date': 1792800000,
      'durationSeconds': 1800,
      'topicId': 42,
    },
    <String, dynamic>{
      'chatId': 123,
      'messageId': 27,
      'originalName': 'starter-kit.zip',
      'caption': 'Your project starter kit',
      'sizeBytes': 15000000,
      'date': 1792700000,
      'topicId': 42,
    },
    <String, dynamic>{
      'chatId': 123,
      'messageId': 26,
      'originalName': 'database.pptx',
      'caption': 'Database patterns and tradeoffs',
      'sizeBytes': 4800000,
      'date': 1792600000,
      'topicId': 42,
    },
  ];
  static Map<String, dynamic> page(String snapshot) => <String, dynamic>{
    'items': items,
    'snapshotId': snapshot,
    'hasMore': true,
  };
  static Map<String, dynamic> job() => <String, dynamic>{
    'id': 'saved-job',
    'chat': chat,
    'state': 'downloading',
    'filter': 'all_attachments',
    'library': true,
    'destination': '/tmp/Telegram Media/Development Collective [123]',
    'destinationRoot': '/tmp/Telegram Media',
    'topic': <String, dynamic>{'id': 42, 'title': 'Backend'},
    'fileTypes': <String>['video', 'pdf'],
    'files': <Map<String, dynamic>>[
      <String, dynamic>{
        'item': items[0],
        'state': 'downloading',
        'progress': <String, dynamic>{
          'itemKey': '123:30:api-design.mp4',
          'percent': 42.8,
          'bytesDownloaded': 235400000,
          'totalBytes': 550000000,
          'speedBytesPerSecond': 5200000,
          'etaSeconds': 61,
          'updatedAt': DateTime.now().millisecondsSinceEpoch,
        },
      },
      <String, dynamic>{'item': items[1], 'state': 'queued'},
      <String, dynamic>{
        'item': items[2],
        'state': 'completed',
        'savedPath': '/tmp/authentication.mp4',
      },
    ],
  };
  @override
  Future<BackendMessage> request(
    String action, {
    Map<String, dynamic>? params,
    String? requestId,
    Duration timeout = const Duration(seconds: 30),
  }) async {
    calls.add(<String, dynamic>{
      'action': action,
      'params': params,
      'requestId': requestId,
    });
    switch (action) {
      case 'health':
        return const BackendMessage(
          id: 'health',
          event: 'result',
          data: <String, dynamic>{
            'ready': true,
            'supportsGracefulPause': true,
            'managedDownloadsSupported': true,
            'tdlVersion': '0.20.4',
          },
        );
      case 'list_chats':
        return BackendMessage(
          id: 'chats',
          event: 'result',
          data: <String, dynamic>{
            'chats': <Map<String, dynamic>>[chat],
          },
        );
      case 'list_jobs':
        return BackendMessage(
          id: 'jobs',
          event: 'result',
          data: <String, dynamic>{
            'jobs': <Map<String, dynamic>>[job()],
          },
        );
      case 'list_media':
        if (deferMedia) {
          final Completer<BackendMessage> completer =
              Completer<BackendMessage>();
          pending.add(completer);
          return completer.future;
        }
        return BackendMessage(
          id: requestId ?? '',
          event: 'result',
          data: page('snapshot-42'),
        );
      case 'start_media_download':
        return BackendMessage(id: requestId ?? '', event: 'job_accepted');
      default:
        return const BackendMessage(id: '', event: 'result');
    }
  }

  @override
  Future<void> dispose() => messages.close();
}

Future<ManagerController> readyController(LibraryBackend backend) async {
  final ManagerController controller = ManagerController(
    backend: backend,
    thumbnailCache: ThumbnailCache(memoryOnly: true),
  );
  await controller.initialize();
  await controller.openChat(
    controller.chats.single,
    topic: controller.chats.single.topics[1],
  );
  return controller;
}

Future<void> capturePreview(
  WidgetTester tester,
  GlobalKey repaint,
  String path,
) async {
  if (Platform.environment['TMM_CAPTURE_PREVIEW'] != '1') return;
  await tester.runAsync(
    () => Future<void>.delayed(const Duration(milliseconds: 80)),
  );
  await tester.pumpAndSettle();
  final RenderRepaintBoundary boundary =
      repaint.currentContext!.findRenderObject()! as RenderRepaintBoundary;
  await tester.runAsync(() async {
    final ui.Image image = await boundary.toImage();
    final data = await image.toByteData(format: ui.ImageByteFormat.png);
    await File(path).writeAsBytes(data!.buffer.asUint8List());
    image.dispose();
  });
}

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();
  setUpAll(() async {
    final String? fontDir = Platform.environment['TMM_PREVIEW_FONT_DIR'];
    if (fontDir == null) return;
    for (final String family in <String>['Roboto', 'MaterialIcons']) {
      final FontLoader loader = FontLoader(family);
      final List<String> files = family == 'Roboto'
          ? <String>[
              'Roboto-Regular.ttf',
              'Roboto-Medium.ttf',
              'Roboto-Bold.ttf',
              'Roboto-Black.ttf',
            ]
          : <String>['MaterialIcons-Regular.otf'];
      for (final String file in files) {
        loader.addFont(
          File('$fontDir/$file').readAsBytes().then(ByteData.sublistView),
        );
      }
      await loader.load();
    }
  });
  test(
    'topic and explicit file filters are sent without expanding selection',
    () async {
      final LibraryBackend backend = LibraryBackend();
      final ManagerController controller = await readyController(backend);
      final Directory dir = await Directory.systemTemp.createTemp(
        'tmm-selection-',
      );
      controller.setDestination(dir.path);
      controller.toggleFileType('all', false);
      controller.toggleFileType('pdf', true);
      expect(controller.visibleMedia, hasLength(1));
      controller.selectVisibleMedia(true);
      await controller.startMediaDownload();
      final Map<String, dynamic> params =
          backend.calls.last['params'] as Map<String, dynamic>;
      final Map<String, dynamic> selection =
          params['selection'] as Map<String, dynamic>;
      expect(selection['keys'], <String>['123:29:architecture.pdf']);
      expect(selection['fileTypes'], <String>['pdf']);
      expect(selection['topic'], <String, dynamic>{
        'id': 42,
        'title': 'Backend',
      });
      expect(selection['entireScope'], false);
      expect(params.containsKey('group'), false);
      controller.dispose();
      await dir.delete();
    },
  );

  test(
    'narrowing types preserves selection and sends the selected files actual types',
    () async {
      final backend = LibraryBackend();
      final controller = await readyController(backend);
      final directory = await Directory.systemTemp.createTemp(
        'tmm-filter-selection-',
      );
      addTearDown(() => directory.delete(recursive: true));
      controller.setDestination(directory.path);
      controller.toggleMedia(controller.mediaPage!.items[1], true);
      controller.toggleFileType('all', false);
      controller.toggleFileType('video', true);
      expect(controller.selectedMediaCount, 1);
      await controller.startMediaDownload();
      final params = backend.calls.last['params'] as Map<String, dynamic>;
      final selection = params['selection'] as Map<String, dynamic>;
      expect(selection['fileTypes'], ['pdf']);
      expect(selection['keys'], ['123:29:architecture.pdf']);
      controller.dispose();
    },
  );
  test('partial progress retains percentage for the same file only', () async {
    final LibraryBackend backend = LibraryBackend();
    final ManagerController controller = await readyController(backend);
    backend.messages.add(
      const BackendMessage(
        id: 'saved-job',
        event: 'download_progress',
        data: <String, dynamic>{
          'progress': <String, dynamic>{
            'jobId': 'saved-job',
            'itemKey': '123:30:api-design.mp4',
            'speedBytesPerSecond': 5000000,
          },
        },
      ),
    );
    await Future<void>.delayed(Duration.zero);
    expect(controller.jobs.single.files.first.progress?.percent, 42.8);
    backend.messages.add(
      const BackendMessage(
        id: 'saved-job',
        event: 'download_progress',
        data: <String, dynamic>{
          'progress': <String, dynamic>{
            'jobId': 'saved-job',
            'itemKey': '123:29:architecture.pdf',
            'speedBytesPerSecond': 1000,
          },
        },
      ),
    );
    await Future<void>.delayed(Duration.zero);
    expect(controller.jobs.single.files[1].progress?.percent, isNull);
    expect(controller.jobs.single.files.first.progress?.percent, 42.8);
    controller.dispose();
  });

  test('late media response cannot replace a newer topic', () async {
    final LibraryBackend backend = LibraryBackend();
    final ManagerController controller = await readyController(backend);
    backend.deferMedia = true;
    final Future<void> first = controller.openChat(
      controller.chats.single,
      topic: const TelegramTopic(id: 1, title: 'General'),
    );
    final Future<void> second = controller.openChat(
      controller.chats.single,
      topic: const TelegramTopic(id: 43, title: 'Frontend'),
    );
    backend.pending[1].complete(
      BackendMessage(
        id: '',
        event: 'result',
        data: LibraryBackend.page('newer'),
      ),
    );
    await second;
    backend.pending[0].complete(
      BackendMessage(
        id: '',
        event: 'result',
        data: LibraryBackend.page('older'),
      ),
    );
    await first;
    expect(controller.activeTopic?.id, 43);
    expect(controller.mediaPage?.snapshotId, 'newer');
    controller.dispose();
  });

  testWidgets(
    'desktop topics, selection and per-file queue render without overflow',
    (WidgetTester tester) async {
      tester.view.physicalSize = const Size(1440, 1000);
      tester.view.devicePixelRatio = 1;
      addTearDown(tester.view.resetPhysicalSize);
      addTearDown(tester.view.resetDevicePixelRatio);
      final LibraryBackend backend = LibraryBackend();
      final ManagerController controller = await readyController(backend);
      controller.toggleMedia(controller.mediaPage!.items[3], true);
      controller.toggleMedia(controller.mediaPage!.items[4], true);
      final GlobalKey repaint = GlobalKey();
      await tester.pumpWidget(
        RepaintBoundary(
          key: repaint,
          child: TelegramMediaDownloaderApp(
            sidebarWidthStore: FakeSidebarWidthStore(),
            autoConnect: false,
            controller: controller,
          ),
        ),
      );
      await tester.pumpAndSettle();
      expect(find.text('Backend'), findsWidgets);
      expect(
        find.byKey(const ValueKey<String>('file-type-menu')),
        findsOneWidget,
      );
      expect(find.text('42.8%'), findsOneWidget);
      expect(tester.takeException(), isNull);
      await capturePreview(tester, repaint, '/tmp/tmm-library-preview.png');
      if (Platform.environment['TMM_CAPTURE_PREVIEW'] == '1') {
        await tester.drag(
          find.byKey(const ValueKey<String>('sidebar-resize-handle')),
          const Offset(172, 0),
        );
        await tester.pumpAndSettle();
        await capturePreview(
          tester,
          repaint,
          '/tmp/tmm-library-resizable-preview.png',
        );
        await tester.tap(
          find.byKey(const ValueKey<String>('sidebar-resize-handle')),
        );
        await tester.pump(const Duration(milliseconds: 80));
        await tester.tap(
          find.byKey(const ValueKey<String>('sidebar-resize-handle')),
        );
        await tester.pumpAndSettle();
        await tester.pumpWidget(
          RepaintBoundary(
            key: repaint,
            child: TelegramMediaDownloaderApp(
              sidebarWidthStore: FakeSidebarWidthStore(),
              autoConnect: false,
              controller: controller,
              themeMode: ThemeMode.dark,
            ),
          ),
        );
        await tester.pumpAndSettle();
        expect(tester.takeException(), isNull);
        await capturePreview(
          tester,
          repaint,
          '/tmp/tmm-library-dark-preview.png',
        );
        await tester.pumpWidget(
          RepaintBoundary(
            key: repaint,
            child: TelegramMediaDownloaderApp(
              sidebarWidthStore: FakeSidebarWidthStore(),
              autoConnect: false,
              controller: controller,
              themeMode: ThemeMode.light,
            ),
          ),
        );
        await tester.pumpAndSettle();
      }
      tester.view.physicalSize = const Size(1440, 800);
      await tester.pump();
      await tester.tap(find.byKey(const ValueKey<String>('queue-toggle')));
      await tester.pump();
      expect(find.text('Your downloads'), findsOneWidget);
      expect(find.text('Retry unfinished'), findsNothing);
      expect(find.text('Open file'), findsNothing);
      await tester.scrollUntilVisible(
        find.byTooltip('Open file'),
        120,
        scrollable: find.descendant(
          of: find.byKey(const PageStorageKey<String>('download-list')),
          matching: find.byWidgetPredicate(
            (widget) =>
                widget is Scrollable &&
                widget.axisDirection == AxisDirection.down,
          ),
        ),
      );
      await tester.pump();
      expect(find.byTooltip('Open file'), findsOneWidget);
      expect(tester.takeException(), isNull);
      final queueScroll = tester.state<ScrollableState>(
        find.descendant(
          of: find.byKey(const PageStorageKey<String>('download-list')),
          matching: find.byWidgetPredicate(
            (widget) =>
                widget is Scrollable &&
                widget.axisDirection == AxisDirection.down,
          ),
        ),
      );
      queueScroll.position.jumpTo(0);
      await tester.pumpAndSettle();
      await capturePreview(tester, repaint, '/tmp/tmm-queue-preview.png');
      if (Platform.environment['TMM_CAPTURE_PREVIEW'] == '1') {
        await tester.tap(
          find.byKey(const ValueKey<String>('downloads-filters')),
        );
        await tester.pumpAndSettle();
        await capturePreview(tester, repaint, '/tmp/tmm-filters-preview.png');
        await tester.sendKeyEvent(LogicalKeyboardKey.escape);
        await tester.pumpAndSettle();
      }
      await tester.pumpWidget(const SizedBox.shrink());
      controller.dispose();
    },
  );

  testWidgets('compact library keeps file actions usable', (
    WidgetTester tester,
  ) async {
    tester.view.physicalSize = const Size(560, 760);
    tester.view.devicePixelRatio = 1;
    addTearDown(tester.view.resetPhysicalSize);
    addTearDown(tester.view.resetDevicePixelRatio);
    final LibraryBackend backend = LibraryBackend();
    final ManagerController controller = await readyController(backend);
    await tester.pumpWidget(
      TelegramMediaDownloaderApp(
        sidebarWidthStore: FakeSidebarWidthStore(),
        autoConnect: false,
        controller: controller,
      ),
    );
    await tester.pump();
    expect(find.text('Download selected'), findsOneWidget);
    expect(tester.takeException(), isNull);
    await tester.pumpWidget(const SizedBox.shrink());
    controller.dispose();
  });
}

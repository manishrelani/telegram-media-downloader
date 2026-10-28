import 'dart:async';
import 'dart:convert';
import 'dart:io';

import 'package:flutter/gestures.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:telegram_media_downloader/main.dart';
import 'package:telegram_media_downloader/services/sidebar_width_store.dart';

import 'library_test.dart' show LibraryBackend, readyController;
import 'support/sidebar_width_store.dart';

const handle = ValueKey<String>('sidebar-resize-handle');
const sidebar = ValueKey<String>('chat-sidebar');
double sidebarWidth(WidgetTester tester) =>
    tester.getSize(find.byKey(sidebar)).width;

class DelayedSidebarWidthStore extends FakeSidebarWidthStore {
  final Completer<double?> loaded = Completer<double?>();
  @override
  Future<double?> loadWidth() => loaded.future;
}

class UnavailableSidebarWidthStore extends FakeSidebarWidthStore {
  @override
  Future<void> saveWidth(double width) async =>
      throw const FileSystemException('Unavailable');
}

void main() {
  test(
    'sidebar preference persists across new store instances without losing other settings',
    () async {
      final Directory directory = await Directory.systemTemp.createTemp(
        'tmm-sidebar-',
      );
      addTearDown(() => directory.delete(recursive: true));
      final File file = File('${directory.path}/ui-settings.json');
      await file.writeAsString(
        jsonEncode(<String, dynamic>{'anotherSetting': true}),
      );
      final store = FileSidebarWidthStore(file: file);
      await Future.wait(<Future<void>>[
        store.saveWidth(300),
        store.saveWidth(372),
      ]);
      expect(await FileSidebarWidthStore(file: file).loadWidth(), 372);
      expect(
        (jsonDecode(await file.readAsString())
            as Map<String, dynamic>)['anotherSetting'],
        true,
      );
      await file.writeAsString('{broken');
      expect(await store.loadWidth(), isNull);
      await file.writeAsString('{"sidebarWidth":"wide"}');
      expect(await store.loadWidth(), isNull);
    },
  );

  testWidgets(
    'drag limits, double-click reset, and restoration from saved preferences',
    (WidgetTester tester) async {
      tester.view.physicalSize = const Size(1440, 900);
      tester.view.devicePixelRatio = 1;
      addTearDown(tester.view.resetPhysicalSize);
      addTearDown(tester.view.resetDevicePixelRatio);
      final store = FakeSidebarWidthStore();
      final controller = await readyController(LibraryBackend());
      await tester.pumpWidget(
        TelegramMediaDownloaderApp(
          autoConnect: false,
          controller: controller,
          sidebarWidthStore: store,
        ),
      );
      await tester.pumpAndSettle();
      expect(sidebarWidth(tester), 248);
      await tester.drag(find.byKey(handle), const Offset(400, 0));
      await tester.pumpAndSettle();
      expect(sidebarWidth(tester), 420);
      expect(store.width, 420);
      expect(store.writes, 1);
      await tester.pumpWidget(const SizedBox.shrink());
      await tester.pumpWidget(
        TelegramMediaDownloaderApp(
          autoConnect: false,
          controller: controller,
          sidebarWidthStore: store,
        ),
      );
      await tester.pumpAndSettle();
      expect(sidebarWidth(tester), 420);
      await tester.drag(find.byKey(handle), const Offset(-500, 0));
      await tester.pumpAndSettle();
      expect(sidebarWidth(tester), 220);
      await tester.tap(find.byKey(handle));
      await tester.pump(const Duration(milliseconds: 80));
      await tester.tap(find.byKey(handle));
      await tester.pumpAndSettle();
      expect(sidebarWidth(tester), 248);
      expect(store.width, 248);
      await tester.pumpWidget(const SizedBox.shrink());
      controller.dispose();
    },
  );

  testWidgets(
    'window resizing protects file space without overwriting the preferred width',
    (WidgetTester tester) async {
      tester.view.physicalSize = const Size(1440, 900);
      tester.view.devicePixelRatio = 1;
      addTearDown(tester.view.resetPhysicalSize);
      addTearDown(tester.view.resetDevicePixelRatio);
      final store = FakeSidebarWidthStore(420);
      final controller = await readyController(LibraryBackend());
      await tester.pumpWidget(
        TelegramMediaDownloaderApp(
          autoConnect: false,
          controller: controller,
          sidebarWidthStore: store,
        ),
      );
      await tester.pumpAndSettle();
      expect(sidebarWidth(tester), 420);
      tester.view.physicalSize = const Size(800, 900);
      await tester.pumpAndSettle();
      expect(sidebarWidth(tester), 272);
      expect(store.width, 420);
      expect(store.writes, 0);
      tester.view.physicalSize = const Size(1440, 900);
      await tester.pumpAndSettle();
      expect(sidebarWidth(tester), 420);
      tester.view.physicalSize = const Size(560, 760);
      await tester.pumpAndSettle();
      expect(find.byKey(handle), findsNothing);
      expect(find.byTooltip('Browse chats'), findsOneWidget);
      expect(tester.takeException(), isNull);
      await tester.pumpWidget(const SizedBox.shrink());
      controller.dispose();
    },
  );

  testWidgets(
    'keyboard resizing and reset are available when the divider has focus',
    (WidgetTester tester) async {
      tester.view.physicalSize = const Size(1440, 900);
      tester.view.devicePixelRatio = 1;
      addTearDown(tester.view.resetPhysicalSize);
      addTearDown(tester.view.resetDevicePixelRatio);
      final store = FakeSidebarWidthStore();
      final controller = await readyController(LibraryBackend());
      await tester.pumpWidget(
        TelegramMediaDownloaderApp(
          autoConnect: false,
          controller: controller,
          sidebarWidthStore: store,
        ),
      );
      await tester.pumpAndSettle();
      await tester.tap(find.byKey(handle));
      await tester.pump(const Duration(milliseconds: 350));
      await tester.sendKeyEvent(LogicalKeyboardKey.arrowRight);
      await tester.pumpAndSettle();
      expect(sidebarWidth(tester), 264);
      await tester.sendKeyEvent(LogicalKeyboardKey.end);
      await tester.pumpAndSettle();
      expect(sidebarWidth(tester), 420);
      await tester.sendKeyEvent(LogicalKeyboardKey.enter);
      await tester.pumpAndSettle();
      expect(sidebarWidth(tester), 248);
      expect(store.width, 248);
      expect(tester.takeException(), isNull);
      await tester.pumpWidget(const SizedBox.shrink());
      controller.dispose();
    },
  );
  testWidgets('a delayed preference read cannot overwrite a user resize', (
    tester,
  ) async {
    tester.view.physicalSize = const Size(1440, 900);
    tester.view.devicePixelRatio = 1;
    addTearDown(tester.view.resetPhysicalSize);
    addTearDown(tester.view.resetDevicePixelRatio);
    final store = DelayedSidebarWidthStore();
    await tester.pumpWidget(
      TelegramMediaDownloaderApp(autoConnect: false, sidebarWidthStore: store),
    );
    await tester.drag(find.byKey(handle), const Offset(80, 0));
    await tester.pumpAndSettle();
    final chosen = sidebarWidth(tester);
    store.loaded.complete(420);
    await tester.pumpAndSettle();
    expect(sidebarWidth(tester), chosen);
    expect(store.width, chosen);
    await tester.pumpWidget(const SizedBox.shrink());
  });

  testWidgets('right-click controls resize without dragging', (tester) async {
    tester.view.physicalSize = const Size(1440, 900);
    tester.view.devicePixelRatio = 1;
    addTearDown(tester.view.resetPhysicalSize);
    addTearDown(tester.view.resetDevicePixelRatio);
    final store = FakeSidebarWidthStore();
    await tester.pumpWidget(
      TelegramMediaDownloaderApp(autoConnect: false, sidebarWidthStore: store),
    );
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(handle), buttons: kSecondaryMouseButton);
    await tester.pumpAndSettle();
    await tester.tap(find.text('Make sidebar wider'));
    await tester.pumpAndSettle();
    expect(sidebarWidth(tester), 264);
    expect(store.width, 264);
    expect(tester.takeException(), isNull);
    await tester.pumpWidget(const SizedBox.shrink());
  });

  testWidgets('saving failure leaves resizing usable with a clear notice', (
    tester,
  ) async {
    tester.view.physicalSize = const Size(1440, 900);
    tester.view.devicePixelRatio = 1;
    addTearDown(tester.view.resetPhysicalSize);
    addTearDown(tester.view.resetDevicePixelRatio);
    await tester.pumpWidget(
      TelegramMediaDownloaderApp(
        autoConnect: false,
        sidebarWidthStore: UnavailableSidebarWidthStore(),
      ),
    );
    await tester.pumpAndSettle();
    await tester.drag(find.byKey(handle), const Offset(80, 0));
    await tester.pumpAndSettle();
    expect(sidebarWidth(tester), greaterThan(248));
    expect(
      find.text('Sidebar resized, but its width could not be saved.'),
      findsOneWidget,
    );
    expect(tester.takeException(), isNull);
    await tester.pumpWidget(const SizedBox.shrink());
  });
}

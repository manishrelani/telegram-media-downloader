import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:telegram_media_downloader/main.dart';
import 'package:telegram_media_downloader/ui/quiet_theme.dart';

import 'library_test.dart' show LibraryBackend, readyController;
import 'support/sidebar_width_store.dart';

double contrast(Color a, Color b) {
  final double first = a.computeLuminance();
  final double second = b.computeLuminance();
  return ((first > second ? first : second) + .05) /
      ((first < second ? first : second) + .05);
}

void main() {
  test('both themes keep secondary text and primary actions readable', () {
    for (final QuietColors colors in <QuietColors>[
      QuietColors.light,
      QuietColors.dark,
    ]) {
      expect(contrast(colors.text, colors.canvas), greaterThanOrEqualTo(4.5));
      expect(contrast(colors.muted, colors.canvas), greaterThanOrEqualTo(4.5));
      expect(contrast(colors.muted, colors.sidebar), greaterThanOrEqualTo(4.5));
      expect(
        contrast(colors.onAccent, colors.accent),
        greaterThanOrEqualTo(4.5),
      );
    }
  });

  testWidgets(
    'file type menu keeps multi-selection and Escape dismissal usable',
    (WidgetTester tester) async {
      tester.view.physicalSize = const Size(1200, 850);
      tester.view.devicePixelRatio = 1;
      addTearDown(tester.view.resetPhysicalSize);
      addTearDown(tester.view.resetDevicePixelRatio);
      final backend = LibraryBackend();
      final controller = await readyController(backend);
      await tester.pumpWidget(
        TelegramMediaDownloaderApp(
          sidebarWidthStore: FakeSidebarWidthStore(),
          autoConnect: false,
          controller: controller,
        ),
      );
      await tester.pump();
      expect(find.text('PDFs'), findsNothing);
      await tester.tap(find.byKey(const ValueKey<String>('file-type-menu')));
      await tester.pumpAndSettle();
      await tester.tap(find.widgetWithText(CheckboxMenuButton, 'All files'));
      await tester.pumpAndSettle();
      await tester.tap(find.widgetWithText(CheckboxMenuButton, 'PDFs'));
      await tester.pumpAndSettle();
      expect(controller.fileTypes, <String>{'pdf'});
      expect(controller.visibleMedia, hasLength(1));
      await tester.sendKeyEvent(LogicalKeyboardKey.escape);
      await tester.pumpAndSettle();
      expect(find.byType(CheckboxMenuButton), findsNothing);
      expect(tester.takeException(), isNull);
      await tester.pumpWidget(const SizedBox.shrink());
      controller.dispose();
    },
  );

  for (final ThemeMode mode in <ThemeMode>[ThemeMode.light, ThemeMode.dark]) {
    testWidgets('large text fits a compact ${mode.name} library and queue', (
      WidgetTester tester,
    ) async {
      tester.view.physicalSize = const Size(800, 650);
      tester.view.devicePixelRatio = 1;
      addTearDown(tester.view.resetPhysicalSize);
      addTearDown(tester.view.resetDevicePixelRatio);
      final controller = await readyController(LibraryBackend());
      await tester.pumpWidget(
        TelegramMediaDownloaderApp(
          sidebarWidthStore: FakeSidebarWidthStore(),
          autoConnect: false,
          controller: controller,
          themeMode: mode,
          textScaler: TextScaler.linear(1.5),
        ),
      );
      await tester.pump();
      expect(tester.takeException(), isNull);
      await tester.tap(find.byTooltip('View download queue'));
      await tester.pumpAndSettle();
      expect(find.text('Your downloads'), findsOneWidget);
      expect(tester.takeException(), isNull);
      await tester.tap(find.byTooltip('Back to files'));
      await tester.pumpAndSettle();
      expect(
        find.byKey(const ValueKey<String>('file-type-menu')),
        findsOneWidget,
      );
      expect(tester.takeException(), isNull);
      await tester.pumpWidget(const SizedBox.shrink());
      controller.dispose();
    });
  }
  testWidgets(
    'queue preserves browser scroll and keyboard Find returns to file search',
    (WidgetTester tester) async {
      tester.view.physicalSize = const Size(1200, 760);
      tester.view.devicePixelRatio = 1;
      addTearDown(tester.view.resetPhysicalSize);
      addTearDown(tester.view.resetDevicePixelRatio);
      final controller = await readyController(LibraryBackend());
      await tester.pumpWidget(
        TelegramMediaDownloaderApp(
          sidebarWidthStore: FakeSidebarWidthStore(),
          autoConnect: false,
          controller: controller,
        ),
      );
      await tester.pump();
      final list = find.byKey(const ValueKey<String>('media-scroll-123:42'));
      await tester.drag(list, const Offset(0, -350));
      await tester.pumpAndSettle();
      double position() => tester
          .state<ScrollableState>(
            find.descendant(of: list, matching: find.byType(Scrollable)),
          )
          .position
          .pixels;
      final double before = position();
      expect(before, greaterThan(0));
      await tester.tap(find.byKey(const ValueKey<String>('queue-toggle')));
      await tester.pumpAndSettle();
      await tester.tap(find.byTooltip('Back to files'));
      await tester.pumpAndSettle();
      expect(position(), closeTo(before, .1));
      await tester.tap(find.byKey(const ValueKey<String>('queue-toggle')));
      await tester.pumpAndSettle();
      await tester.sendKeyDownEvent(LogicalKeyboardKey.metaLeft);
      await tester.sendKeyEvent(LogicalKeyboardKey.keyF);
      await tester.sendKeyUpEvent(LogicalKeyboardKey.metaLeft);
      await tester.pumpAndSettle();
      final field = tester.widget<TextField>(
        find.byWidgetPredicate(
          (Widget w) =>
              w is TextField &&
              w.decoration?.labelText == 'Search loaded files',
        ),
      );
      expect(field.focusNode!.hasFocus, isTrue);
      expect(tester.takeException(), isNull);
      await tester.pumpWidget(const SizedBox.shrink());
      controller.dispose();
    },
  );
}

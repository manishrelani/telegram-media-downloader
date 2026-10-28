import 'package:flutter/material.dart';
import 'package:flutter/rendering.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:telegram_media_downloader/ui/thumbnail_viewport_observer.dart';

void main() {
  testWidgets(
    'sliver cache rows do not count as visible and scrolling updates demand',
    (tester) async {
      tester.view.physicalSize = const Size(800, 500);
      tester.view.devicePixelRatio = 1;
      addTearDown(tester.view.resetPhysicalSize);
      addTearDown(tester.view.resetDevicePixelRatio);
      final visible = <int, bool>{};
      final scroll = ScrollController();
      addTearDown(scroll.dispose);
      await tester.pumpWidget(
        MaterialApp(
          home: ListView.builder(
            controller: scroll,
            scrollCacheExtent: ScrollCacheExtent.pixels(1500),
            itemExtent: 100,
            itemCount: 30,
            itemBuilder: (_, index) => ThumbnailViewportObserver(
              key: ValueKey(index),
              onVisibilityChanged: (value) => visible[index] = value.visible,
              child: const SizedBox(width: 96, height: 54),
            ),
          ),
        ),
      );
      await tester.pumpAndSettle();
      expect(
        find.byKey(const ValueKey(10), skipOffstage: false),
        findsOneWidget,
      );
      expect(visible[0], true);
      expect(visible[10], isNot(true));
      scroll.jumpTo(1000);
      await tester.pumpAndSettle();
      expect(visible[0], false);
      expect(visible[10], true);
      await tester.pumpWidget(const SizedBox.shrink());
      expect(visible[10], false);
    },
  );
  testWidgets('offstage panes report no visible thumbnails until shown', (
    tester,
  ) async {
    bool hidden = true;
    late StateSetter rebuild;
    final seen = <bool>[];
    await tester.pumpWidget(
      MaterialApp(
        home: StatefulBuilder(
          builder: (_, setState) {
            rebuild = setState;
            return Offstage(
              offstage: hidden,
              child: ThumbnailViewportObserver(
                onVisibilityChanged: (value) => seen.add(value.visible),
                child: const SizedBox(width: 96, height: 54),
              ),
            );
          },
        ),
      ),
    );
    await tester.pumpAndSettle();
    expect(seen.where((value) => value), isEmpty);
    rebuild(() => hidden = false);
    await tester.pumpAndSettle();
    expect(seen, isNotEmpty);
    expect(seen.last, true);
    rebuild(() => hidden = true);
    await tester.pumpAndSettle();
    expect(seen.last, false);
  });
  testWidgets('viewport resizing rechecks rows without a scroll event', (
    tester,
  ) async {
    tester.view.physicalSize = const Size(800, 300);
    tester.view.devicePixelRatio = 1;
    addTearDown(tester.view.resetPhysicalSize);
    addTearDown(tester.view.resetDevicePixelRatio);
    final seen = <int, bool>{};
    await tester.pumpWidget(
      MaterialApp(
        home: ListView.builder(
          scrollCacheExtent: ScrollCacheExtent.pixels(1200),
          itemExtent: 100,
          itemCount: 12,
          itemBuilder: (_, index) => ThumbnailViewportObserver(
            onVisibilityChanged: (value) => seen[index] = value.visible,
            child: const SizedBox(width: 96, height: 54),
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();
    expect(seen[5], isNot(true));
    tester.view.physicalSize = const Size(800, 700);
    await tester.pumpAndSettle();
    expect(seen[5], true);
  });
}

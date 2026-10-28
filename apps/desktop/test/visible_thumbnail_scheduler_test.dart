import 'dart:async';
import 'package:flutter_test/flutter_test.dart';
import 'package:telegram_media_downloader/services/visible_thumbnail_scheduler.dart';

ThumbnailDemand demand(String key, {String snapshot = 'page'}) =>
    ThumbnailDemand(snapshotId: snapshot, key: key, version: '1');

class RequestGate {
  final calls = <(String, List<ThumbnailDemand>)>[];
  final pending = <Completer<bool>>[];
  final cancelled = <String>[];
  Future<bool> send(String id, List<ThumbnailDemand> items) {
    calls.add((id, items));
    final c = Completer<bool>();
    pending.add(c);
    return c.future;
  }

  Future<void> cancel(String id) async {
    cancelled.add(id);
  }

  VisibleThumbnailScheduler scheduler() =>
      VisibleThumbnailScheduler(send: send, cancel: cancel);
}

void main() {
  testWidgets(
    'coalesces visibility and deduplicates visible owners in screen order',
    (tester) async {
      final gate = RequestGate();
      final s = gate.scheduler();
      addTearDown(s.dispose);
      s.setVisible('old', demand('offscreen'), position: 0);
      s.setVisible('old', null);
      s.setVisible('a', demand('a'), position: 30);
      s.setVisible('b', demand('b'), position: 10);
      s.setVisible('duplicate', demand('b'), position: 20);
      await tester.pump(const Duration(milliseconds: 79));
      expect(gate.calls, isEmpty);
      await tester.pump(const Duration(milliseconds: 1));
      expect(gate.calls, hasLength(1));
      expect(gate.calls.single.$2.map((e) => e.key), ['b', 'a']);
    },
  );
  testWidgets(
    'scroll cancellation waits for the old request before dispatching the new viewport',
    (tester) async {
      final gate = RequestGate();
      final s = gate.scheduler();
      addTearDown(s.dispose);
      s.setVisible('a', demand('a'));
      s.setVisible('b', demand('b'), position: 10);
      await tester.pump(const Duration(milliseconds: 80));
      s.setVisible('a', null);
      s.setVisible('c', demand('c'), position: -1);
      await tester.pump(const Duration(milliseconds: 80));
      expect(gate.calls, hasLength(1));
      expect(gate.cancelled, [gate.calls.first.$1]);
      expect(gate.calls, hasLength(1));
      expect(gate.calls, hasLength(1));
      gate.pending.first.complete(false);
      await tester.pump();
      await tester.pump(const Duration(milliseconds: 80));
      expect(gate.calls.last.$2.map((e) => e.key), ['c', 'b']);
      expect(gate.calls, hasLength(2));
    },
  );
  testWidgets(
    'a completed image leaving the viewport does not cancel its pending sibling',
    (tester) async {
      final gate = RequestGate();
      final s = gate.scheduler();
      addTearDown(s.dispose);
      s.setVisible('a', demand('a'));
      s.setVisible('b', demand('b'));
      await tester.pump(const Duration(milliseconds: 80));
      expect(gate.calls, hasLength(1));
      s.markReady(demand('a'));
      s.setVisible('a', null);
      await tester.pump(const Duration(milliseconds: 80));
      expect(gate.cancelled, isEmpty);
    },
  );
  testWidgets(
    'busy transfer retry waits two seconds and stops after disposal',
    (tester) async {
      final gate = RequestGate();
      final s = gate.scheduler();
      s.setVisible('a', demand('a'));
      await tester.pump(const Duration(milliseconds: 80));
      expect(gate.calls, hasLength(1));
      gate.pending.first.complete(true);
      await tester.pump();
      await tester.pump(const Duration(milliseconds: 1999));
      expect(gate.calls, hasLength(1));
      await tester.pump(const Duration(milliseconds: 1));
      expect(gate.calls, hasLength(2));
      gate.pending.last.complete(true);
      await tester.pump();
      s.dispose();
      await tester.pump(const Duration(seconds: 5));
      expect(gate.calls, hasLength(2));
    },
  );
  testWidgets(
    'failed requests do not spin and can retry after leaving and reentering',
    (tester) async {
      final gate = RequestGate();
      final s = gate.scheduler();
      addTearDown(s.dispose);
      s.setVisible('a', demand('a'));
      await tester.pump(const Duration(milliseconds: 80));
      expect(gate.calls, hasLength(1));
      gate.pending.first.completeError(StateError('offline'));
      await tester.pump();
      await tester.pump(const Duration(seconds: 5));
      expect(gate.calls, hasLength(1));
      s.setVisible('a', null);
      s.setVisible('a', demand('a'));
      await tester.pump(const Duration(milliseconds: 80));
      expect(gate.calls, hasLength(2));
    },
  );
  testWidgets(
    'limits each request to twelve visible keys and keeps snapshots separate',
    (tester) async {
      final gate = RequestGate();
      final s = gate.scheduler();
      addTearDown(s.dispose);
      for (int i = 0; i < 15; i++) {
        s.setVisible(i, demand('$i'), position: i.toDouble());
      }
      s.setVisible(
        'other',
        demand('other', snapshot: 'other-page'),
        position: 20,
      );
      await tester.pump(const Duration(milliseconds: 80));
      expect(gate.calls, hasLength(1));
      expect(
        gate.calls.single.$2.map((e) => e.key),
        List.generate(12, (i) => '$i'),
      );
      expect(gate.calls, hasLength(1));
      gate.pending.first.complete(false);
      await tester.pump();
      await tester.pump(const Duration(milliseconds: 80));
      expect(gate.calls.last.$2.map((e) => e.key), ['12', '13', '14']);
    },
  );
  testWidgets('disposal cancels active requests and prevents queued work', (
    tester,
  ) async {
    final gate = RequestGate();
    final s = gate.scheduler();
    s.setVisible('a', demand('a'));
    await tester.pump(const Duration(milliseconds: 80));
    s.dispose();
    expect(gate.calls, hasLength(1));
    expect(gate.cancelled, [gate.calls.first.$1]);
    expect(gate.calls, hasLength(1));
    gate.pending.first.complete(false);
    await tester.pump(const Duration(seconds: 5));
    expect(gate.calls, hasLength(1));
  });
}

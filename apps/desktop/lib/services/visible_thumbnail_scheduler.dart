import 'dart:async';

class ThumbnailDemand {
  const ThumbnailDemand({
    required this.snapshotId,
    required this.key,
    required this.version,
  });
  final String snapshotId, key, version;
  String get identity => '$snapshotId:$key:$version';
}

class _VisibleDemand {
  const _VisibleDemand(this.item, this.position);
  final ThumbnailDemand item;
  final double position;
}

class _ThumbnailBatch {
  _ThumbnailBatch(this.id, this.items)
    : pending = items.map((e) => e.identity).toSet();
  final String id;
  final List<ThumbnailDemand> items;
  final Set<String> pending;
  bool cancelled = false;
}

/// One low-priority request for the current viewport, never the whole page.
class VisibleThumbnailScheduler {
  VisibleThumbnailScheduler({required this.send, required this.cancel});
  final Future<bool> Function(String, List<ThumbnailDemand>) send;
  final Future<void> Function(String) cancel;
  final Map<Object, _VisibleDemand> _owners = {};
  final Set<String> _attempted = {};
  Timer? _debounce, _retry;
  _ThumbnailBatch? _active;
  bool _disposed = false;
  int _sequence = 0;

  void setVisible(Object owner, ThumbnailDemand? item, {double position = 0}) {
    if (_disposed) return;
    final previous = _owners[owner];
    if (item == null) {
      _owners.remove(owner);
    } else {
      if (previous?.item.identity == item.identity &&
          previous?.position == position) {
        return;
      }
      _owners[owner] = _VisibleDemand(item, position);
    }
    if (previous != null &&
        !_owners.values.any((e) => e.item.identity == previous.item.identity)) {
      // A failed item can retry when the user returns to it. Successful items
      // are already represented by a sharp image in the controller/cache.
      _attempted.remove(previous.item.identity);
    }
    _cancelObsolete();
    _schedule();
  }

  void markReady(ThumbnailDemand item) {
    if (_disposed) return;
    _attempted.add(item.identity);
    _active?.pending.remove(item.identity);
  }

  void _cancelObsolete() {
    final active = _active;
    if (active == null || active.cancelled) return;
    final wanted = _owners.values.map((e) => e.item.identity).toSet();
    if (active.pending.any((key) => !wanted.contains(key))) _cancelActive();
  }

  void _cancelActive() {
    final active = _active;
    if (active == null || active.cancelled) return;
    active.cancelled = true;
    unawaited(cancel(active.id).catchError((Object _) {}));
  }

  void _schedule() {
    if (_disposed || _retry != null) return;
    _debounce?.cancel();
    _debounce = Timer(const Duration(milliseconds: 80), _pump);
  }

  void _pump() {
    _debounce = null;
    if (_disposed || _active != null || _retry != null) return;
    final visible = _owners.values.toList()
      ..sort((a, b) => a.position.compareTo(b.position));
    final unique = <String>{};
    final selected = <ThumbnailDemand>[];
    for (final entry in visible) {
      final item = entry.item;
      if (_attempted.contains(item.identity) || !unique.add(item.identity)) {
        continue;
      }
      if (selected.isNotEmpty && selected.first.snapshotId != item.snapshotId) {
        continue;
      }
      selected.add(item);
      if (selected.length == 12) break;
    }
    if (selected.isEmpty) return;
    final batch = _ThumbnailBatch(
      'thumb-${DateTime.now().microsecondsSinceEpoch}-${++_sequence}',
      selected,
    );
    _active = batch;
    unawaited(_run(batch));
  }

  Future<void> _run(_ThumbnailBatch batch) async {
    try {
      final busy = await send(batch.id, batch.items);
      if (_disposed || batch.cancelled) return;
      if (busy) {
        _retry = Timer(const Duration(seconds: 2), () {
          _retry = null;
          _pump();
        });
      } else {
        _attempted.addAll(batch.pending);
      }
    } catch (_) {
      if (!_disposed && !batch.cancelled) _attempted.addAll(batch.pending);
    } finally {
      if (identical(_active, batch)) _active = null;
      if (!_disposed && _retry == null) _schedule();
    }
  }

  void reset() {
    if (_disposed) return;
    _debounce?.cancel();
    _retry?.cancel();
    _debounce = null;
    _retry = null;
    _owners.clear();
    _attempted.clear();
    _cancelActive();
  }

  void dispose() {
    if (_disposed) return;
    _disposed = true;
    _debounce?.cancel();
    _retry?.cancel();
    _owners.clear();
    _attempted.clear();
    _cancelActive();
  }
}

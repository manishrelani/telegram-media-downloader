import 'dart:convert';
import 'dart:io';
import 'dart:typed_data';

import '../models/media_item.dart';

/// Bounded, best-effort local preview cache. Never fetches a video or a web URL.
class ThumbnailCache {
  ThumbnailCache({Directory? directory, this.memoryOnly = false})
    : _directory = directory ?? _defaultDirectory();
  final Directory? _directory;
  final bool memoryOnly;
  final _memory = <String, Uint8List>{};
  final _pending = <String, Future<Uint8List?>>{};
  int _writes = 0;
  static Directory? _defaultDirectory() {
    final env = Platform.environment;
    final home = env['HOME'];
    if (env['TMM_STATE_DIR']?.isNotEmpty == true) {
      return Directory('${env['TMM_STATE_DIR']}/thumbnails');
    }
    if (Platform.isMacOS && home != null) {
      return Directory(
        '$home/Library/Caches/telegram-media-manager/thumbnails',
      );
    }
    final root =
        env['LOCALAPPDATA'] ??
        env['XDG_CACHE_HOME'] ??
        (home == null ? null : '$home/.cache');
    return root == null
        ? null
        : Directory('$root/telegram-media-manager/thumbnails');
  }

  String _key(MediaItem item) {
    int hash = 2166136261;
    for (final value in utf8.encode(item.key)) {
      hash = ((hash ^ value) * 16777619) & 0xffffffff;
    }
    return '${item.chatId}_${item.messageId}_${item.date}_${hash.toRadixString(16)}_${item.thumbnailVersion.replaceAll(RegExp(r'[^0-9-]'), '')}';
  }

  static (int, int)? pixelSize(Uint8List bytes) {
    if (bytes.length < 24 || bytes.length > 262144) return null;
    (int, int)? bounded(int width, int height) =>
        width > 0 && height > 0 && width <= 1280 && height <= 1280
        ? (width, height)
        : null;
    if (bytes[0] == 137 && bytes[1] == 80 && bytes[2] == 78 && bytes[3] == 71) {
      final data = ByteData.sublistView(bytes);
      return bounded(data.getUint32(16), data.getUint32(20));
    }
    if (bytes[0] != 0xff || bytes[1] != 0xd8) return null;
    int offset = 2;
    while (offset + 8 < bytes.length) {
      if (bytes[offset] != 0xff) return null;
      final marker = bytes[offset + 1];
      if (marker == 0xff) {
        offset++;
        continue;
      }
      final length = (bytes[offset + 2] << 8) | bytes[offset + 3];
      if (length < 2 || offset + 2 + length > bytes.length) return null;
      if (const {0xc0, 0xc1, 0xc2}.contains(marker)) {
        return bounded(
          (bytes[offset + 7] << 8) | bytes[offset + 8],
          (bytes[offset + 5] << 8) | bytes[offset + 6],
        );
      }
      offset += 2 + length;
    }
    return null;
  }

  bool _valid(Uint8List bytes) => pixelSize(bytes) != null;
  Future<Uint8List?> load(MediaItem item) {
    if (item.hasSpoiler) return Future.value(null);
    final key = _key(item);
    final embedded = item.thumbnail;
    // Include the payload in memory identity so edits cannot reuse a stale preview.
    final memoryKey = '$key:${embedded ?? ''}';
    final cached = _memory.remove(memoryKey);
    if (cached != null) {
      _memory[memoryKey] = cached;
      return Future.value(cached);
    }
    return _pending.putIfAbsent(
      memoryKey,
      () => _load(item, key, memoryKey).whenComplete(() {
        _pending.remove(memoryKey);
      }),
    );
  }

  Future<Uint8List?> _load(MediaItem item, String key, String memoryKey) async {
    Uint8List? bytes;
    try {
      if (item.thumbnail?.isNotEmpty == true) {
        if (item.thumbnail!.length > 350000) return null;
        bytes = base64Decode(item.thumbnail!);
        if (!_valid(bytes)) return null;
      } else if (!memoryOnly && _directory != null) {
        final file = File('${_directory.path}/$key.thumb');
        if (await file.length() > 262144) return null;
        bytes = await file.readAsBytes();
        if (!_valid(bytes)) return null;
      }
      if (bytes == null) return null;
      _memory[memoryKey] = bytes;
      while (_memory.length > 128) {
        _memory.remove(_memory.keys.first);
      }
      if (!memoryOnly &&
          _directory != null &&
          item.thumbnail?.isNotEmpty == true) {
        try {
          await _directory.create(recursive: true);
          final file = File('${_directory.path}/$key.thumb');
          await file.writeAsBytes(bytes, flush: true);
          if (++_writes % 16 == 0) await _trim();
        } catch (_) {
          /* Cache failures do not prevent displaying an embedded preview. */
        }
      }
      return bytes;
    } catch (_) {
      return null;
    }
  }

  Future<void> _trim() async {
    final files = await _directory!
        .list(followLinks: false)
        .where((e) => e is File && e.path.endsWith('.thumb'))
        .cast<File>()
        .toList();
    if (files.length <= 256) return;
    final dated = <File, DateTime>{};
    for (final file in files) {
      dated[file] = await file.lastModified();
    }
    files.sort((a, b) => dated[a]!.compareTo(dated[b]!));
    for (final file in files.take(files.length - 256)) {
      await file.delete();
    }
  }
}

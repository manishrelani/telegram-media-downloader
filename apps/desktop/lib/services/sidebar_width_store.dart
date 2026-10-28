import 'dart:convert';
import 'dart:io';

abstract class SidebarWidthStore {
  Future<double?> loadWidth();
  Future<void> saveWidth(double width);
}

/// Small, local UI preferences kept separately from Telegram account data.
class FileSidebarWidthStore implements SidebarWidthStore {
  FileSidebarWidthStore({File? file}) : _file = file ?? _defaultFile();

  final File? _file;
  Future<void> _pending = Future<void>.value();
  int _sequence = 0;

  static File? _defaultFile() {
    final env = Platform.environment;
    String? directory = env['TMM_STATE_DIR'];
    if (directory == null || directory.isEmpty) {
      final home = env['HOME'];
      if (Platform.isMacOS && home != null) {
        directory = '$home/Library/Application Support/telegram-media-manager';
      } else if (Platform.isWindows && env['APPDATA'] != null) {
        directory = '${env['APPDATA']}/telegram-media-manager';
      } else if (Platform.isLinux) {
        final config = env['XDG_CONFIG_HOME'];
        if (config != null && config.isNotEmpty) {
          directory = '$config/telegram-media-manager';
        } else if (home != null) {
          directory = '$home/.config/telegram-media-manager';
        }
      }
    }
    return directory == null || directory.isEmpty
        ? null
        : File('$directory/ui-settings.json');
  }

  Future<Map<String, dynamic>> _read() async {
    final file = _file;
    if (file == null) return <String, dynamic>{};
    try {
      final decoded = jsonDecode(await file.readAsString());
      return decoded is Map<String, dynamic> ? decoded : <String, dynamic>{};
    } on FileSystemException catch (error) {
      if (error.osError?.errorCode == 2 || !await file.exists()) {
        return <String, dynamic>{};
      }
      rethrow;
    } on FormatException {
      return <String, dynamic>{};
    }
  }

  @override
  Future<double?> loadWidth() async {
    final value = (await _read())['sidebarWidth'];
    return value is num && value.isFinite ? value.toDouble() : null;
  }

  @override
  Future<void> saveWidth(double width) {
    if (!width.isFinite) return Future<void>.value();
    final write = _pending.then((_) => _write(width));
    // A failed write must not prevent a later resize from being saved.
    _pending = write.catchError((Object _) {});
    return write;
  }

  Future<void> _write(double width) async {
    final file = _file;
    if (file == null) {
      throw const FileSystemException('UI preferences unavailable');
    }
    final settings = await _read();
    settings['sidebarWidth'] = width;
    await file.parent.create(recursive: true);
    final temporary = File(
      '${file.path}.$pid.${DateTime.now().microsecondsSinceEpoch}.${_sequence++}.tmp',
    );
    try {
      await temporary.writeAsString(jsonEncode(settings), flush: true);
      await temporary.rename(file.path);
    } finally {
      if (await temporary.exists()) await temporary.delete();
    }
  }
}

import 'dart:io';
import 'package:flutter_test/flutter_test.dart';
import 'package:telegram_media_downloader/services/backend_client.dart';

void main() {
  test('bundled patched runtime is chosen ahead of a PATH installation', () {
    final root = Directory.systemTemp.createTempSync('tmm-runtime-test-');
    addTearDown(() => root.deleteSync(recursive: true));
    final name = Platform.isWindows ? 'tdl-tmm.exe' : 'tdl-tmm';
    final bundled = File('${root.path}${Platform.pathSeparator}$name')
      ..writeAsStringSync('fixture');
    expect(
      findTdlExecutable(
        {},
        '${root.path}${Platform.pathSeparator}telegram-media-downloader-backend',
      ),
      bundled.path,
    );
  });
  test('explicit trusted runtime override is preserved', () {
    expect(
      findTdlExecutable({'TDL_PATH': '/controlled/fake-tdl'}, '/app/backend'),
      '/controlled/fake-tdl',
    );
  });
  test('packaged runtime is required when no explicit override exists', () {
    expect(
      () => findTdlExecutable({}, '/missing/backend'),
      throwsA(isA<BackendException>()),
    );
  });
  test('release ignores a developer runtime override', () {
    final root = Directory.systemTemp.createTempSync('tmd-release-runtime-');
    addTearDown(() => root.deleteSync(recursive: true));
    final name = Platform.isWindows ? 'tdl-tmm.exe' : 'tdl-tmm';
    final bundled = File('${root.path}${Platform.pathSeparator}$name')
      ..writeAsStringSync('fixture');
    expect(
      findTdlExecutable(
        {'TDL_PATH': '/controlled/fake-tdl'},
        '${root.path}${Platform.pathSeparator}backend',
        releaseMode: true,
      ),
      bundled.path,
    );
  });
  test('release refuses an override when bundled runtime is missing', () {
    final root = Directory.systemTemp.createTempSync('tmd-missing-runtime-');
    addTearDown(() => root.deleteSync(recursive: true));
    expect(
      () => findTdlExecutable(
        {'TDL_PATH': '/controlled/fake-tdl'},
        '${root.path}${Platform.pathSeparator}backend',
        releaseMode: true,
      ),
      throwsA(isA<BackendException>()),
    );
  });

  test('release backend ignores explicit and environment overrides', () async {
    final root = Directory.systemTemp.createTempSync('tmd-release-backend-');
    addTearDown(() => root.deleteSync(recursive: true));
    final name = Platform.isWindows
        ? 'telegram-media-downloader-backend.exe'
        : 'telegram-media-downloader-backend';
    final bundled = File('${root.path}${Platform.pathSeparator}$name')
      ..writeAsStringSync('fixture');
    final fake = File('${root.path}${Platform.pathSeparator}fake-backend')
      ..writeAsStringSync('fixture');
    expect(
      await findBackendExecutable(
        environment: {'TMM_BACKEND_PATH': fake.path},
        appExecutable: '${root.path}${Platform.pathSeparator}app',
        workingDirectory: root.path,
        explicitPath: fake.path,
        releaseMode: true,
      ),
      bundled.path,
    );
  });
  test('release backend does not fall back to development builds', () async {
    final root = Directory.systemTemp.createTempSync('tmd-no-bundled-backend-');
    addTearDown(() => root.deleteSync(recursive: true));
    final name = Platform.isWindows
        ? 'telegram-media-downloader-backend.exe'
        : 'telegram-media-downloader-backend';
    final fake = File(
      '${root.path}${Platform.pathSeparator}build${Platform.pathSeparator}$name',
    );
    fake.parent.createSync();
    fake.writeAsStringSync('fixture');
    await expectLater(
      findBackendExecutable(
        environment: {'TMM_BACKEND_PATH': fake.path},
        appExecutable: '${root.path}${Platform.pathSeparator}app',
        workingDirectory: root.path,
        releaseMode: true,
      ),
      throwsA(isA<BackendException>()),
    );
  });
  test('development backend override remains available', () async {
    final root = Directory.systemTemp.createTempSync('tmd-dev-backend-');
    addTearDown(() => root.deleteSync(recursive: true));
    final fake = File('${root.path}${Platform.pathSeparator}fake-backend')
      ..writeAsStringSync('fixture');
    expect(
      await findBackendExecutable(
        environment: {'TMM_BACKEND_PATH': fake.path},
        appExecutable: '${root.path}${Platform.pathSeparator}app',
        workingDirectory: root.path,
        releaseMode: false,
      ),
      fake.path,
    );
  });
}

import 'dart:io';

import 'package:flutter_test/flutter_test.dart';
import 'package:telegram_media_downloader/services/backend_client.dart';

void main() {
  group('backendStartErrorMessage', () {
    test('explains the macOS App Sandbox cause for EPERM', () {
      final String message = backendStartErrorMessage(
        executable: '/repo/build/telegram-media-downloader-backend',
        error: ProcessException(
          '/repo/build/telegram-media-downloader-backend',
          const <String>[],
          'Operation not permitted',
          1,
        ),
        isMacOS: true,
      );

      expect(message, contains('Operation not permitted'));
      expect(message, contains('App Sandbox'));
      expect(message, contains('configure_macos_entitlements.py'));
      expect(message, contains('Mac App Store'));
    });

    test(
      'does not show macOS sandbox guidance for a different process error',
      () {
        final String message = backendStartErrorMessage(
          executable: '/missing/backend',
          error: ProcessException(
            '/missing/backend',
            const <String>[],
            'No such file or directory',
            2,
          ),
          isMacOS: true,
        );

        expect(message, contains('/missing/backend'));
        expect(message, contains('No such file or directory'));
        expect(message, isNot(contains('App Sandbox')));
      },
    );

    test('does not show macOS sandbox guidance on other operating systems', () {
      final String message = backendStartErrorMessage(
        executable: '/repo/backend',
        error: ProcessException(
          '/repo/backend',
          const <String>[],
          'Operation not permitted',
          1,
        ),
        isMacOS: false,
      );

      expect(message, isNot(contains('App Sandbox')));
    });
  });
}

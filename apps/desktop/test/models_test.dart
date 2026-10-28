import 'package:flutter_test/flutter_test.dart';
import 'package:telegram_media_downloader/models/backend_message.dart';
import 'package:telegram_media_downloader/models/chat.dart';
import 'package:telegram_media_downloader/models/download_job.dart';

void main() {
  group('TelegramChat', () {
    test('parses tdl visible_name field and keeps id stable', () {
      final TelegramChat chat = TelegramChat.fromJson(<String, dynamic>{
        'id': 4488394154,
        'type': 'channel',
        'visible_name': 'ChaiCode GenAI Course',
        'username': 'chaicode',
      });
      expect(chat.id, 4488394154);
      expect(chat.name, 'ChaiCode GenAI Course');
      expect(chat.type, 'channel');
      expect(chat.matches('genai'), isTrue);
      expect(chat.matches('123'), isFalse);
    });

    test('rejects invalid chat ids instead of inventing a selector', () {
      expect(
        () => TelegramChat.fromJson(<String, dynamic>{
          'type': 'channel',
          'name': 'Broken',
        }),
        throwsFormatException,
      );
    });
  });

  group('DownloadProgress', () {
    test('handles numeric values from JSON as int or double', () {
      final DownloadProgress progress =
          DownloadProgress.fromJson(<String, dynamic>{
            'fileName': 'Lecture.pdf',
            'percent': 35.1,
            'bytesDownloaded': 98000,
            'totalBytes': 250000,
            'speedBytesPerSecond': 1970000,
            'etaSeconds': 92,
          });
      expect(progress.percent, 35.1);
      expect(progress.bytesDownloaded, 98000);
      expect(progress.etaSeconds, 92);
      expect(progress.fileName, 'Lecture.pdf');
    });
  });

  group('BackendMessage', () {
    test('parses structured errors without throwing', () {
      final BackendMessage message = BackendMessage.fromJson(<String, dynamic>{
        'version': 1,
        'id': 'chat-1',
        'event': 'error',
        'error': <String, dynamic>{
          'code': 'chat_list_failed',
          'message': 'Not logged in',
        },
      });
      expect(message.isError, isTrue);
      expect(message.errorCode, 'chat_list_failed');
      expect(message.errorMessage, 'Not logged in');
    });
  });

  group('DownloadJob', () {
    test('restores a persisted job', () {
      final DownloadJob job = DownloadJob.fromJson(<String, dynamic>{
        'id': 'job-1',
        'chat': <String, dynamic>{
          'id': 123,
          'type': 'channel',
          'name': 'Courses',
        },
        'state': 'paused',
        'destination': '/tmp/downloads/Courses [123]',
        'filter': 'videos_and_documents',
      });
      expect(job.canResume, isTrue);
      expect(job.canCancel, isTrue);
      expect(job.chat.name, 'Courses');
      expect(job.isActive, isFalse);
    });
  });
}

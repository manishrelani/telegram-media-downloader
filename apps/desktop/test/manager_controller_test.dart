import 'dart:async';
import 'dart:io';

import 'package:flutter_test/flutter_test.dart';
import 'package:telegram_media_downloader/controllers/manager_controller.dart';
import 'package:telegram_media_downloader/models/backend_message.dart';
import 'package:telegram_media_downloader/models/chat.dart';
import 'package:telegram_media_downloader/models/download_job.dart';
import 'package:telegram_media_downloader/services/backend_client.dart';

class FakeBackendClient extends BackendClient {
  FakeBackendClient({
    this.version = 'Version: 0.20.4',
    this.managedDownloadsSupported = true,
  });

  final String version;
  final bool managedDownloadsSupported;
  final StreamController<BackendMessage> eventController =
      StreamController<BackendMessage>.broadcast();
  final List<Map<String, dynamic>> calls = <Map<String, dynamic>>[];

  @override
  Stream<BackendMessage> get events => eventController.stream;

  @override
  Future<BackendMessage> request(
    String action, {
    Map<String, dynamic>? params,
    String? requestId,
    Duration timeout = const Duration(seconds: 30),
  }) async {
    calls.add(<String, dynamic>{
      'action': action,
      'params': params,
      'requestId': requestId,
    });
    switch (action) {
      case 'health':
        return BackendMessage(
          id: 'health',
          event: 'result',
          data: <String, dynamic>{
            'ready': true,
            'tdlVersion': version,
            'supportsGracefulPause': managedDownloadsSupported,
            'managedDownloadsSupported': managedDownloadsSupported,
            'minimumSafeManagedDownloadVersion': '0.20.4',
          },
        );
      case 'list_chats':
        return const BackendMessage(
          id: 'chats',
          event: 'result',
          data: <String, dynamic>{
            'chats': <Map<String, dynamic>>[
              <String, dynamic>{
                'id': 123,
                'type': 'channel',
                'visible_name': 'Demo Course',
              },
              <String, dynamic>{
                'id': 124,
                'type': 'user',
                'visible_name': 'A private chat',
              },
            ],
          },
        );
      case 'list_jobs':
        return const BackendMessage(
          id: 'jobs',
          event: 'result',
          data: <String, dynamic>{'jobs': <dynamic>[]},
        );
      case 'start_chat_download':
        final Map<String, dynamic> chat =
            params!['chat']! as Map<String, dynamic>;
        eventController.add(
          BackendMessage(
            id: requestId ?? 'fake-job',
            event: 'job_state',
            data: <String, dynamic>{
              'job': <String, dynamic>{
                'id': requestId ?? 'fake-job',
                'chat': chat,
                'state': 'queued',
                'destination': params['destinationRoot'],
                'filter': params['filter'],
              },
            },
          ),
        );
        return BackendMessage(
          id: requestId ?? 'fake-job',
          event: 'job_accepted',
          data: <String, dynamic>{'jobId': requestId ?? 'fake-job'},
        );
      default:
        return BackendMessage(id: 'generic', event: 'result');
    }
  }

  @override
  Future<void> dispose() async {
    await eventController.close();
  }
}

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  test(
    'initialization loads chats and persisted jobs through the JSONL abstraction',
    () async {
      final FakeBackendClient backend = FakeBackendClient();
      final ManagerController controller = ManagerController(backend: backend);
      await controller.initialize();

      expect(controller.backendReady, isTrue);
      expect(controller.connected, isTrue);
      expect(controller.chats, hasLength(2));
      expect(
        controller.chats.first.name,
        'A private chat',
      ); // Sorted by display name.
      expect(controller.supportsPause, isTrue);
      expect(controller.errorMessage, isNull);

      controller.dispose();
    },
  );

  test(
    'download selection requires a valid destination and sends explicit filter settings',
    () async {
      final Directory destination = await Directory.systemTemp.createTemp(
        'tmm-controller-test-',
      );
      final FakeBackendClient backend = FakeBackendClient();
      final ManagerController controller = ManagerController(backend: backend);
      await controller.initialize();
      controller.setDestination(destination.path);
      controller.toggleChat(controller.chats.first, true);
      controller.setFilter('documents_only');
      controller.setTakeout(false);
      controller.setSkipSame(true);

      await controller.startSelectedDownloads();

      final Map<String, dynamic> call = backend.calls.lastWhere(
        (Map<String, dynamic> call) => call['action'] == 'start_chat_download',
      );
      expect(
        call['requestId'],
        isNotEmpty,
        reason: 'the UI job ID must be the persistent backend job ID',
      );
      final Map<String, dynamic> params =
          call['params']! as Map<String, dynamic>;
      expect(params['destinationRoot'], destination.path);
      expect(params['filter'], 'documents_only');
      expect(params['takeout'], isFalse);
      expect(params['skipSame'], isTrue);
      expect((params['chat']! as Map<String, dynamic>)['id'], 124);
      await Future<void>.delayed(const Duration(milliseconds: 10));
      expect(
        controller.jobs,
        hasLength(1),
        reason: 'job accepted/state events must update the same local job ID',
      );

      controller.dispose();
      await destination.delete(recursive: true);
    },
  );

  test(
    'managed downloads are blocked when the installed tdl version is unsupported',
    () async {
      final Directory destination = await Directory.systemTemp.createTemp(
        'tmm-old-tdl-',
      );
      final FakeBackendClient backend = FakeBackendClient(
        version: 'Version: 0.20.3',
        managedDownloadsSupported: false,
      );
      final ManagerController controller = ManagerController(backend: backend);
      await controller.initialize();
      controller.setDestination(destination.path);
      controller.toggleChat(controller.chats.first, true);

      await controller.startSelectedDownloads();

      expect(controller.errorMessage, contains('tdl v0.20.4 or newer'));
      expect(
        backend.calls.any(
          (Map<String, dynamic> call) =>
              call['action'] == 'start_chat_download',
        ),
        isFalse,
      );
      controller.dispose();
      await destination.delete(recursive: true);
    },
  );

  test('download request is rejected when no chat has been selected', () async {
    final FakeBackendClient backend = FakeBackendClient();
    final ManagerController controller = ManagerController(backend: backend);
    await controller.initialize();
    await controller.startSelectedDownloads();
    expect(controller.errorMessage, contains('Select at least one chat'));
    expect(
      backend.calls.any(
        (Map<String, dynamic> call) => call['action'] == 'start_chat_download',
      ),
      isFalse,
    );
    controller.dispose();
  });
  test(
    'retry restores the job original destination instead of current picker folder',
    () async {
      final Directory current = await Directory.systemTemp.createTemp(
        'tmm-current-destination-',
      );
      final FakeBackendClient backend = FakeBackendClient();
      final ManagerController controller = ManagerController(backend: backend);
      await controller.initialize();
      controller.setDestination(current.path);
      final DownloadJob failed = DownloadJob(
        id: 'old-job',
        chat: const TelegramChat(id: 123, type: 'channel', name: 'Demo Course'),
        state: 'failed',
        destination: '/previous/TelegramDownloads/Demo Course [123]',
        filter: 'documents_only',
      );

      await controller.retryJob(failed);

      final Map<String, dynamic> call = backend.calls.lastWhere(
        (Map<String, dynamic> call) => call['action'] == 'start_chat_download',
      );
      final Map<String, dynamic> params =
          call['params']! as Map<String, dynamic>;
      expect(params['destinationRoot'], '/previous/TelegramDownloads');
      expect(params['filter'], 'documents_only');
      controller.dispose();
      await current.delete(recursive: true);
    },
  );
}

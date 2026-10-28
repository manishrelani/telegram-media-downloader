import 'dart:async';
import 'dart:convert';
import 'dart:io';

import 'package:flutter/foundation.dart';

import '../models/backend_message.dart';
import '../models/chat.dart';
import '../models/download_job.dart';
import '../models/media_item.dart';
import '../models/media_filters.dart';
import '../services/thumbnail_cache.dart';
import '../services/visible_thumbnail_scheduler.dart';
import '../services/backend_client.dart';

class ManagerController extends ChangeNotifier {
  ManagerController({BackendClient? backend, ThumbnailCache? thumbnailCache})
    : _backend = backend ?? BackendClient(),
      thumbnailCache = thumbnailCache ?? ThumbnailCache() {
    libraryFilters.addListener(notifyListeners);
    downloadFilters.addListener(notifyListeners);
    _eventSubscription = _backend.events.listen(_handleEvent);
  }

  final BackendClient _backend;
  StreamSubscription<BackendMessage>? _eventSubscription;
  final List<TelegramChat> chats = <TelegramChat>[];
  final Map<String, DownloadJob> _jobs = <String, DownloadJob>{};
  final List<String> _jobOrder = <String>[];
  final Set<int> selectedChatIds = <int>{};

  bool isConnecting = false;
  bool isRefreshingChats = false;
  bool isStartingDownloads = false;
  bool backendReady = false;
  bool connected = false;
  bool supportsPause = false;
  bool managedDownloadsSupported = false;
  String minimumSafeManagedDownloadVersion = '0.20.4';
  String? tdlVersion;
  String statusMessage = 'Connect to Telegram to load your chats.';
  String? errorMessage;
  String searchQuery = '';
  String chatTypeFilter = 'all';
  String destinationRoot = '';
  String filter = 'videos_and_documents';
  bool takeout = true;
  bool groupMedia = false;
  bool skipSame = true;
  int _idSequence = 0;
  TelegramChat? activeChat;
  TelegramTopic? activeTopic;
  MediaPage? mediaPage;
  bool isLoadingMedia = false;
  String? mediaError;
  final MediaFilters libraryFilters = MediaFilters();
  final MediaFilters downloadFilters = MediaFilters(
    sort: 'active',
    groupBy: 'chat',
  );
  final ThumbnailCache thumbnailCache;
  String get mediaSearch => libraryFilters.query;
  set mediaSearch(String value) => libraryFilters.query = value;
  String get mediaSort => libraryFilters.sort;
  set mediaSort(String value) => libraryFilters.sort = value;
  Set<String> get fileTypes => libraryFilters.types;
  final Set<String> selectedMediaKeys = <String>{};
  final Map<String, MediaPage> _mediaCache = <String, MediaPage>{};
  int _mediaGeneration = 0;
  String? _mediaRequestId;
  bool _disposed = false;
  final Map<String, String> _sharpPreviews = <String, String>{};
  final Map<String, Map<String, String>> _pendingPreviews =
      <String, Map<String, String>>{};
  late final VisibleThumbnailScheduler _thumbnailScheduler =
      VisibleThumbnailScheduler(
        send: (id, items) async {
          final response = await _backend.request(
            'request_thumbnails',
            requestId: id,
            params: {
              'snapshotId': items.first.snapshotId,
              'keys': items.map((e) => e.key).toList(),
            },
            timeout: const Duration(seconds: 30),
          );
          return response.data['busy'] == true;
        },
        cancel: (id) async {
          await _backend.request(
            'cancel_media_request',
            params: {'requestId': id},
          );
        },
      );
  void reportThumbnailVisibility(
    Object owner,
    MediaItem item,
    bool visible,
    double position,
  ) {
    if (_disposed) return;
    bool sharp = _sharpPreviews.containsKey(_previewKey(item));
    if (!sharp && item.thumbnail != null) {
      try {
        final size = ThumbnailCache.pixelSize(base64Decode(item.thumbnail!));
        sharp = size != null && size.$1 >= 96 && size.$2 >= 54;
      } catch (_) {
        /* Invalid placeholders can still request a real preview. */
      }
    }
    MediaPage? page;
    if (visible && !sharp && !item.hasSpoiler && item.fileType == 'video') {
      final pages = [
        if (mediaPage != null) mediaPage!,
        ..._mediaCache.values.toList().reversed,
      ];
      for (final candidate in pages) {
        if (candidate.items.any(
          (e) =>
              e.key == item.key && e.thumbnailVersion == item.thumbnailVersion,
        )) {
          page = candidate;
          break;
        }
      }
    }
    _thumbnailScheduler.setVisible(
      owner,
      page == null
          ? null
          : ThumbnailDemand(
              snapshotId: page.snapshotId,
              key: item.key,
              version: item.thumbnailVersion,
            ),
      position: position,
    );
  }

  String _previewKey(MediaItem item) => '${item.key}:${item.thumbnailVersion}';
  void _applyPreviews(
    String snapshotId,
    Map<String, String> images, {
    bool notify = true,
  }) {
    bool known = false;
    for (final key in _mediaCache.keys.toList()) {
      final page = _mediaCache[key]!;
      if (page.snapshotId != snapshotId) continue;
      known = true;
      final updated = page.items.map((item) {
        final image = images[item.key];
        if (image == null || item.hasSpoiler) return item;
        _thumbnailScheduler.markReady(
          ThumbnailDemand(
            snapshotId: snapshotId,
            key: item.key,
            version: item.thumbnailVersion,
          ),
        );
        _sharpPreviews[_previewKey(item)] = image;
        return item.withThumbnail(image);
      }).toList();
      final next = MediaPage(
        items: updated,
        snapshotId: page.snapshotId,
        hasMore: page.hasMore,
        cached: page.cached,
        loadedAt: page.loadedAt,
        limit: page.limit,
      );
      _mediaCache[key] = next;
      if (mediaPage?.snapshotId == snapshotId) mediaPage = next;
    }
    if (!known) {
      _pendingPreviews[snapshotId] = images;
      while (_pendingPreviews.length > 2) {
        _pendingPreviews.remove(_pendingPreviews.keys.first);
      }
      return;
    }
    while (_sharpPreviews.length > 512) {
      _sharpPreviews.remove(_sharpPreviews.keys.first);
    }
    for (final id in _jobs.keys.toList()) {
      final job = _jobs[id]!;
      _jobs[id] = job.copyWith(
        files: job.files.map((file) {
          final image = _sharpPreviews[_previewKey(file.item)];
          return image == null || file.item.hasSpoiler
              ? file
              : file.withThumbnail(image);
        }).toList(),
      );
    }
    if (notify && !_disposed) notifyListeners();
  }

  String get scopeName =>
      activeTopic?.title ??
      (activeChat?.topics.isNotEmpty == true ? 'All topics' : 'All media');
  String get _scopeKey => '${activeChat?.id}:${activeTopic?.id ?? 0}';
  List<MediaEntry> get downloadEntries => [
    for (final job in jobs)
      for (final task in job.files)
        MediaEntry(item: task.item, task: task, job: job),
  ];
  List<MediaEntry> get libraryEntries {
    final candidates = <String, List<MediaEntry>>{};
    for (final record in downloadEntries) {
      candidates.putIfAbsent(record.item.key, () => []).add(record);
    }
    return [
      for (final item in mediaPage?.items ?? <MediaItem>[])
        _libraryEntry(item, candidates[item.key] ?? []),
    ];
  }

  MediaEntry _libraryEntry(MediaItem item, List<MediaEntry> candidates) {
    // Preserve a known saved copy even when a newer attempt failed.
    final saved = candidates.where((e) => e.downloaded);
    final chosen = saved.isNotEmpty ? saved.first : candidates.firstOrNull;
    return MediaEntry(
      item: item,
      task: chosen?.task,
      job: chosen?.job,
      chat: activeChat,
    );
  }

  List<MediaItem> get visibleMedia =>
      libraryFilters.apply(libraryEntries).map((e) => e.item).toList();

  int get selectedMediaCount => selectedMediaKeys.length;
  FileTask? taskFor(MediaItem item) =>
      libraryEntries.where((e) => e.item.key == item.key).firstOrNull?.task;
  DownloadJob? jobFor(MediaItem item) =>
      libraryEntries.where((e) => e.item.key == item.key).firstOrNull?.job;

  Future<void> openChat(TelegramChat chat, {TelegramTopic? topic}) async {
    _thumbnailScheduler.reset();
    activeChat = chat;
    activeTopic = topic;
    selectedMediaKeys.clear();
    mediaSearch = '';
    mediaPage = _mediaCache[_scopeKey];
    await loadMedia(limit: mediaPage?.limit ?? 100);
  }

  Future<void> loadMedia({int? limit, bool refresh = false}) async {
    final TelegramChat? chat = activeChat;
    if (chat == null) return;
    if (refresh) _thumbnailScheduler.reset();
    final int generation = ++_mediaGeneration;
    final String cacheKey = _scopeKey;
    final int count = limit ?? mediaPage?.limit ?? 100;
    final String? previousRequest = _mediaRequestId;
    if (previousRequest != null) {
      unawaited(
        _backend
            .request(
              'cancel_media_request',
              params: <String, dynamic>{'requestId': previousRequest},
            )
            .catchError(
              (Object error) => const BackendMessage(id: '', event: 'result'),
            ),
      );
    }
    final String requestId =
        'media-${DateTime.now().microsecondsSinceEpoch}-${++_idSequence}';
    _mediaRequestId = requestId;
    isLoadingMedia = true;
    mediaError = null;
    notifyListeners();
    try {
      final BackendMessage response = await _backend.request(
        'list_media',
        requestId: requestId,
        params: <String, dynamic>{
          'chat': chat.toJson(),
          if (activeTopic != null) 'topic': activeTopic!.toJson(),
          'limit': count,
          'refresh': refresh,
        },
        timeout: const Duration(minutes: 10),
      );
      if (_disposed || generation != _mediaGeneration) return;
      final MediaPage page = MediaPage.fromJson(response.data, count);
      if (mediaPage?.snapshotId != page.snapshotId) _thumbnailScheduler.reset();
      _mediaCache[cacheKey] = page;
      if (_mediaCache.length > 16) _mediaCache.remove(_mediaCache.keys.first);
      mediaPage = page;
      final previews = _pendingPreviews.remove(page.snapshotId);
      if (previews != null) {
        _applyPreviews(page.snapshotId, previews, notify: false);
      }
      selectedMediaKeys.removeWhere(
        (String key) => !page.items.any((MediaItem item) => item.key == key),
      );
    } catch (error) {
      if (!_disposed && generation == _mediaGeneration) {
        mediaError = _friendlyError(error);
      }
    } finally {
      if (!_disposed && generation == _mediaGeneration) {
        isLoadingMedia = false;
        _mediaRequestId = null;
        notifyListeners();
      }
    }
  }

  void setMediaSearch(String value) {
    mediaSearch = value;
    notifyListeners();
  }

  void setMediaSort(String value) {
    mediaSort = value;
    notifyListeners();
  }

  void toggleFileType(String kind, bool selected) {
    if (kind == 'all') {
      fileTypes.clear();
      if (selected) fileTypes.addAll(fileTypeLabels.keys);
    } else if (selected) {
      fileTypes.add(kind);
    } else {
      fileTypes.remove(kind);
    }
    notifyListeners();
  }

  void toggleMedia(MediaItem item, bool selected) {
    if (selected) {
      selectedMediaKeys.add(item.key);
    } else {
      selectedMediaKeys.remove(item.key);
    }
    notifyListeners();
  }

  void selectVisibleMedia(bool selected) {
    for (final MediaItem item in visibleMedia) {
      if (selected) {
        selectedMediaKeys.add(item.key);
      } else {
        selectedMediaKeys.remove(item.key);
      }
    }
    notifyListeners();
  }

  void clearMediaSelection() {
    selectedMediaKeys.clear();
    notifyListeners();
  }

  Future<void> startMediaDownload({
    bool entireScope = false,
    MediaItem? only,
  }) async {
    final TelegramChat? chat = activeChat;
    if (chat == null || isStartingDownloads) return;
    if (!managedDownloadsSupported || !connected) {
      errorMessage = 'Connect a supported downloader before downloading.';
      notifyListeners();
      return;
    }
    if (entireScope && fileTypes.isEmpty) {
      errorMessage = 'Select at least one file type.';
      notifyListeners();
      return;
    }
    if (destinationRoot.trim().isEmpty ||
        !await Directory(destinationRoot).exists()) {
      errorMessage =
          'Choose an existing save folder. External drives must be mounted.';
      notifyListeners();
      return;
    }
    final Set<String> keys = only == null
        ? Set<String>.from(selectedMediaKeys)
        : <String>{only.key};
    if (!entireScope && (mediaPage == null || keys.isEmpty)) {
      errorMessage = 'Select the files you want to download.';
      notifyListeners();
      return;
    }
    final String id = _newJobID(chat);
    final List<MediaItem> items = (mediaPage?.items ?? <MediaItem>[])
        .where((MediaItem item) => keys.contains(item.key))
        .toList();
    final effectiveTypes = entireScope
        ? fileTypes.toList()
        : items.map((item) => item.fileType).toSet().toList();
    final Map<String, dynamic> selection = <String, dynamic>{
      if (activeTopic != null) 'topic': activeTopic!.toJson(),
      'fileTypes': effectiveTypes,
      'entireScope': entireScope,
      if (!entireScope) 'snapshotId': mediaPage!.snapshotId,
      if (!entireScope) 'keys': keys.toList(),
    };
    _upsertJob(
      DownloadJob(
        id: id,
        chat: chat,
        state: 'queued',
        destination: destinationRoot,
        destinationRoot: destinationRoot,
        filter: 'all_attachments',
        library: true,
        topic: activeTopic,
        fileTypes: effectiveTypes,
        takeout: takeout,
        files: entireScope
            ? <FileTask>[]
            : items
                  .map(
                    (MediaItem item) => FileTask(item: item, state: 'queued'),
                  )
                  .toList(),
      ),
    );
    isStartingDownloads = true;
    errorMessage = null;
    notifyListeners();
    try {
      await _backend.request(
        'start_media_download',
        requestId: id,
        params: <String, dynamic>{
          'chat': chat.toJson(),
          'destinationRoot': destinationRoot,
          'selection': selection,
          'takeout': takeout,
        },
      );
      selectedMediaKeys.removeAll(keys);
      statusMessage = entireScope
          ? 'Queued all matching files in $scopeName.'
          : 'Queued ${keys.length} file(s).';
    } catch (error) {
      _updateJob(
        id,
        (DownloadJob job) =>
            job.copyWith(state: 'failed', errorMessage: _friendlyError(error)),
      );
    } finally {
      isStartingDownloads = false;
      notifyListeners();
    }
  }

  Future<void> openFile(
    DownloadJob job,
    FileTask file, {
    bool reveal = false,
  }) async {
    try {
      await _backend.request(
        'open_download_file',
        params: <String, dynamic>{
          'jobId': job.id,
          'itemKey': file.item.key,
          'reveal': reveal,
        },
      );
    } catch (error) {
      errorMessage = _friendlyError(error);
      notifyListeners();
    }
  }

  List<DownloadJob> get jobs => _jobOrder
      .where(_jobs.containsKey)
      .map((String id) => _jobs[id]!)
      .toList(growable: false);

  List<TelegramChat> get filteredChats => chats
      .where((TelegramChat chat) {
        if (!chat.matches(searchQuery)) return false;
        if (chatTypeFilter == 'all') return true;
        if (chatTypeFilter == 'channel') return chat.type.contains('channel');
        if (chatTypeFilter == 'group') return chat.type.contains('group');
        if (chatTypeFilter == 'private') {
          return chat.type.contains('user') || chat.type.contains('private');
        }
        return true;
      })
      .toList(growable: false);

  List<TelegramChat> get selectedChats => chats
      .where((TelegramChat chat) => selectedChatIds.contains(chat.id))
      .toList(growable: false);

  int get activeJobCount => jobs
      .where(
        (DownloadJob job) => const {
          'preparing',
          'downloading',
          'pausing',
          'cancelling',
        }.contains(job.state),
      )
      .length;
  int get queuedJobCount =>
      jobs.where((DownloadJob job) => job.state == 'queued').length;
  int get completedJobCount =>
      jobs.where((DownloadJob job) => job.state == 'completed').length;

  Future<void> initialize() async {
    isConnecting = true;
    errorMessage = null;
    statusMessage = 'Checking the local downloader…';
    notifyListeners();
    try {
      final BackendMessage health = await _backend.request('health');
      backendReady = health.data['ready'] == true;
      supportsPause = health.data['supportsGracefulPause'] == true;
      managedDownloadsSupported =
          health.data['managedDownloadsSupported'] == true;
      minimumSafeManagedDownloadVersion =
          health.data['minimumSafeManagedDownloadVersion']?.toString() ??
          '0.20.4';
      tdlVersion = health.data['tdlVersion']?.toString();
      connected = false;
      await refreshChats(showSpinner: false);
      await refreshJobs();
      if (backendReady && !managedDownloadsSupported) {
        statusMessage =
            'Update tdl to v$minimumSafeManagedDownloadVersion before downloading.';
      }
    } catch (error) {
      backendReady = false;
      connected = false;
      errorMessage = _friendlyError(error);
      statusMessage = 'The local backend could not connect.';
    } finally {
      isConnecting = false;
      notifyListeners();
    }
  }

  Future<void> refreshChats({bool showSpinner = true}) async {
    if (showSpinner) {
      isRefreshingChats = true;
      errorMessage = null;
      notifyListeners();
    }
    try {
      final BackendMessage response = await _backend.request('list_chats');
      final dynamic rawChats = response.data['chats'];
      if (rawChats is! List) {
        throw const FormatException('Backend returned an invalid chat list.');
      }
      final List<TelegramChat> loaded = <TelegramChat>[];
      for (final dynamic raw in rawChats) {
        if (raw is Map<String, dynamic>) {
          try {
            loaded.add(TelegramChat.fromJson(raw));
          } on FormatException {
            // One malformed chat should not hide every other accessible conversation.
          }
        }
      }
      loaded.sort(
        (TelegramChat a, TelegramChat b) =>
            a.name.toLowerCase().compareTo(b.name.toLowerCase()),
      );
      chats
        ..clear()
        ..addAll(loaded);
      selectedChatIds.removeWhere(
        (int id) => !chats.any((TelegramChat chat) => chat.id == id),
      );
      connected = true;
      statusMessage = 'Connected · ${chats.length} chats available';
      errorMessage = null;
    } catch (error) {
      connected = false;
      errorMessage = _friendlyError(error);
      statusMessage = 'Telegram session could not be verified.';
    } finally {
      isRefreshingChats = false;
      notifyListeners();
    }
  }

  Future<void> refreshJobs() async {
    try {
      final BackendMessage response = await _backend.request('list_jobs');
      final dynamic rawJobs = response.data['jobs'];
      if (rawJobs is! List) return;
      for (final dynamic raw in rawJobs) {
        if (raw is Map<String, dynamic>) _upsertJob(DownloadJob.fromJson(raw));
      }
      notifyListeners();
    } catch (error) {
      // Chat discovery remains useful even when the persistent job store cannot load.
      errorMessage ??=
          'Could not restore previous jobs: ${_friendlyError(error)}';
      notifyListeners();
    }
  }

  void dismissError() {
    errorMessage = null;
    notifyListeners();
  }

  void setSearchQuery(String value) {
    searchQuery = value;
    notifyListeners();
  }

  void setChatTypeFilter(String value) {
    chatTypeFilter = value;
    notifyListeners();
  }

  void toggleChat(TelegramChat chat, bool selected) {
    if (selected) {
      selectedChatIds.add(chat.id);
    } else {
      selectedChatIds.remove(chat.id);
    }
    notifyListeners();
  }

  void selectVisibleChats(bool selected) {
    for (final TelegramChat chat in filteredChats) {
      if (selected) {
        selectedChatIds.add(chat.id);
      } else {
        selectedChatIds.remove(chat.id);
      }
    }
    notifyListeners();
  }

  void setDestination(String path) {
    destinationRoot = path;
    notifyListeners();
  }

  void setFilter(String value) {
    filter = value;
    notifyListeners();
  }

  void setTakeout(bool value) {
    takeout = value;
    notifyListeners();
  }

  void setGroupMedia(bool value) {
    groupMedia = value;
    notifyListeners();
  }

  void setSkipSame(bool value) {
    skipSame = value;
    notifyListeners();
  }

  Future<void> startSelectedDownloads() async {
    if (selectedChats.isEmpty) {
      errorMessage = 'Select at least one chat to download.';
      notifyListeners();
      return;
    }
    if (destinationRoot.trim().isEmpty ||
        !await Directory(destinationRoot).exists()) {
      errorMessage =
          'Choose an existing destination folder. External drives must be mounted first.';
      notifyListeners();
      return;
    }
    if (!connected) {
      errorMessage = 'Connect your Telegram account before starting downloads.';
      notifyListeners();
      return;
    }
    if (!managedDownloadsSupported) {
      final String detected = (tdlVersion ?? '').trim();
      errorMessage =
          'Managed downloads require tdl v$minimumSafeManagedDownloadVersion or newer. ${detected.isEmpty ? '' : 'Detected: $detected. '}Update tdl, then reconnect.';
      notifyListeners();
      return;
    }

    isStartingDownloads = true;
    errorMessage = null;
    notifyListeners();
    final List<TelegramChat> snapshot = selectedChats;
    try {
      for (final TelegramChat chat in snapshot) {
        final String id = _newJobID(chat);
        _upsertJob(
          DownloadJob(
            id: id,
            chat: chat,
            state: 'queued',
            destination: destinationRoot,
            filter: filter,
            takeout: takeout,
            groupMedia: groupMedia,
            skipSame: skipSame,
          ),
        );
        notifyListeners();
        try {
          await _backend.request(
            'start_chat_download',
            requestId: id,
            params: <String, dynamic>{
              'chat': chat.toJson(),
              'destinationRoot': destinationRoot,
              'filter': filter,
              'takeout': takeout,
              'group': groupMedia,
              'skipSame': skipSame,
            },
          );
        } catch (error) {
          _updateJob(
            id,
            (DownloadJob job) => job.copyWith(
              state: 'failed',
              errorCode: 'start_failed',
              errorMessage: _friendlyError(error),
            ),
          );
        }
      }
      statusMessage =
          'Queued ${snapshot.length} chat download${snapshot.length == 1 ? '' : 's'}.';
      selectedChatIds.clear();
    } finally {
      isStartingDownloads = false;
      notifyListeners();
    }
  }

  Future<void> pauseJob(DownloadJob job) async {
    try {
      await _backend.request(
        'pause_download',
        params: <String, dynamic>{'jobId': job.id},
      );
      statusMessage = 'Pausing ${job.chat.name}…';
      errorMessage = null;
    } catch (error) {
      errorMessage = _friendlyError(error);
    }
    notifyListeners();
  }

  Future<void> resumeJob(DownloadJob job) async {
    try {
      await _backend.request(
        'resume_download',
        params: <String, dynamic>{'jobId': job.id},
      );
      statusMessage = 'Resuming ${job.chat.name}…';
      errorMessage = null;
    } catch (error) {
      errorMessage = _friendlyError(error);
    }
    notifyListeners();
  }

  Future<void> cancelJob(DownloadJob job) async {
    try {
      await _backend.request(
        'cancel_download',
        params: <String, dynamic>{'jobId': job.id},
      );
      statusMessage = 'Cancellation requested for ${job.chat.name}.';
      errorMessage = null;
    } catch (error) {
      errorMessage = _friendlyError(error);
    }
    notifyListeners();
  }

  Future<void> retryJob(DownloadJob job) async {
    if (job.library) {
      try {
        await _backend.request(
          'retry_download',
          params: <String, dynamic>{'jobId': job.id},
        );
        errorMessage = null;
      } catch (error) {
        errorMessage = _friendlyError(error);
      }
      notifyListeners();
      return;
    }
    final String id = _newJobID(job.chat);
    _upsertJob(
      DownloadJob(
        id: id,
        chat: job.chat,
        state: 'queued',
        destination: destinationRoot.isEmpty
            ? job.destination
            : destinationRoot,
        filter: job.filter,
      ),
    );
    notifyListeners();
    try {
      await _backend.request(
        'start_chat_download',
        requestId: id,
        params: <String, dynamic>{
          'chat': job.chat.toJson(),
          'destinationRoot': _destinationForJob(job),
          'filter': job.filter,
          'takeout': job.takeout,
          'group': job.groupMedia,
          'skipSame': job.skipSame,
        },
      );
    } catch (error) {
      _updateJob(
        id,
        (DownloadJob current) => current.copyWith(
          state: 'failed',
          errorCode: 'retry_start_failed',
          errorMessage: _friendlyError(error),
        ),
      );
    }
  }

  void _handleEvent(BackendMessage message) {
    if (message.event == 'media_thumbnails') {
      final snapshot = message.data['snapshotId'];
      final images = message.data['thumbnails'];
      if (snapshot is! String || images is! Map || images.length > 100) return;
      if (!_mediaCache.values.any((page) => page.snapshotId == snapshot) &&
          message.id != _mediaRequestId) {
        return;
      }
      final valid = <String, String>{};
      int total = 0;
      for (final entry in images.entries) {
        if (entry.key is String &&
            entry.value is String &&
            (entry.value as String).length <= 350000) {
          total += (entry.value as String).length;
          if (total > 8 * 1024 * 1024) break;
          valid[entry.key as String] = entry.value as String;
        }
      }
      _applyPreviews(snapshot, valid);
      return;
    }
    if (message.isError) {
      if (message.id.startsWith('media-') || message.id.startsWith('thumb-')) {
        return; // Handled by the scoped browser request.
      }
      final String error =
          '${message.errorCode ?? 'backend_error'}: ${message.errorMessage ?? 'Unknown backend error'}';
      final DownloadJob? job = _jobs[message.id];
      if (job != null && !job.isTerminal) {
        _updateJob(
          message.id,
          (DownloadJob current) => current.copyWith(
            state: 'failed',
            errorCode: message.errorCode,
            errorMessage: message.errorMessage,
          ),
        );
      } else {
        errorMessage = error;
      }
      notifyListeners();
      return;
    }

    if (message.event == 'download_progress') {
      final dynamic rawProgress = message.data['progress'];
      if (rawProgress is Map<String, dynamic>) {
        final DownloadProgress progress = DownloadProgress.fromJson(
          rawProgress,
        );
        final String jobID = (rawProgress['jobId'] ?? message.id).toString();
        _updateJob(jobID, (DownloadJob job) {
          if (job.isTerminal) return job;
          return job.copyWith(
            progress: progress.merge(job.progress),
            state: 'downloading',
            files: job.files
                .map(
                  (FileTask file) => file.item.key == progress.itemKey
                      ? file.withProgress(progress)
                      : file,
                )
                .toList(),
          );
        });
        notifyListeners();
      }
      return;
    }
    if (message.event == 'download_retry') {
      statusMessage =
          message.data['message']?.toString() ??
          'Retrying a temporary download failure…';
      final DownloadJob? job = _jobs[message.id];
      if (job != null) {
        _updateJob(
          message.id,
          (DownloadJob current) => current.copyWith(message: statusMessage),
        );
      }
      notifyListeners();
      return;
    }

    final dynamic rawJob = message.data['job'];
    if (rawJob is Map<String, dynamic>) {
      final DownloadJob incoming = DownloadJob.fromJson(rawJob);
      final DownloadJob? previous = _jobs[incoming.id];
      _upsertJob(
        incoming.copyWith(
          progress: previous?.progress,
          message: message.data['message']?.toString(),
          clearError: incoming.state != 'failed',
        ),
      );
      if (message.event == 'job_paused') {
        statusMessage = 'Paused ${incoming.chat.name}.';
      }
      if (message.event == 'job_completed') {
        statusMessage = 'Completed ${incoming.chat.name}.';
      }
      if (message.event == 'job_failed') {
        errorMessage =
            incoming.errorMessage ?? message.data['message']?.toString();
      }
      if (message.event == 'job_cancelled') {
        statusMessage = 'Cancelled ${incoming.chat.name}.';
      }
      notifyListeners();
    }
  }

  void _upsertJob(DownloadJob job) {
    if (!_jobs.containsKey(job.id)) _jobOrder.insert(0, job.id);
    _jobs[job.id] = job.copyWith(
      files: job.files.map((file) {
        final image = _sharpPreviews[_previewKey(file.item)];
        return image == null || file.item.hasSpoiler
            ? file
            : file.withThumbnail(image);
      }).toList(),
    );
  }

  void _updateJob(String id, DownloadJob Function(DownloadJob current) update) {
    final DownloadJob? current = _jobs[id];
    if (current != null) _jobs[id] = update(current);
  }

  String _destinationForJob(DownloadJob job) {
    final String rawPath = job.destination.trim();
    if (rawPath.isEmpty) return destinationRoot;
    final List<String> segments = rawPath
        .split(RegExp(r'[/\\]'))
        .where((String part) => part.isNotEmpty)
        .toList();
    final String expectedChatSuffix = ' [${job.chat.id}]';
    if (segments.isNotEmpty && segments.last.endsWith(expectedChatSuffix)) {
      // Persisted job.destination is the chat folder, while start_chat_download
      // accepts the parent destination root. Preserve the folder the job started in.
      return Directory(rawPath).parent.path;
    }
    // A local placeholder is created before the Go backend sends its first job_state.
    return rawPath;
  }

  String _newJobID(TelegramChat chat) {
    _idSequence++;
    return 'job-${chat.id}-${DateTime.now().microsecondsSinceEpoch}-$_idSequence';
  }

  String _friendlyError(Object error) {
    if (error is BackendException) {
      switch (error.code) {
        case 'tdl_unavailable':
          return 'The Telegram runtime is unavailable. Reinstall the app or rebuild its bundled runtime.';
        case 'chat_list_failed':
          return 'Could not list chats. If you are not logged in, run the bundled `tdl-tmm login -T qr` in Terminal, then press Refresh. Details: ${error.message}';
        case 'database_locked':
          return 'tdl is using its local database in another process. Close other tdl commands and retry.';
        default:
          return error.message;
      }
    }
    if (error is TimeoutException) {
      return error.message ?? 'The request timed out.';
    }
    return error.toString();
  }

  @override
  void dispose() {
    _disposed = true;
    _thumbnailScheduler.dispose();
    libraryFilters.removeListener(notifyListeners);
    downloadFilters.removeListener(notifyListeners);
    libraryFilters.dispose();
    downloadFilters.dispose();
    unawaited(_eventSubscription?.cancel());
    unawaited(_backend.dispose());
    super.dispose();
  }
}

import 'package:flutter/material.dart';

import '../controllers/manager_controller.dart';
import '../models/download_job.dart';
import '../models/media_item.dart';
import '../models/media_filters.dart';
import 'media_filter_bar.dart';
import 'video_thumbnail.dart';
import 'components.dart';
import 'quiet_theme.dart';

class DownloadsView extends StatefulWidget {
  const DownloadsView({
    super.key,
    required this.controller,
    this.compact = false,
  });
  final ManagerController controller;
  final bool compact;
  @override
  State<DownloadsView> createState() => _DownloadsViewState();
}

class _DownloadsViewState extends State<DownloadsView> {
  final Set<String> _collapsed = <String>{};
  final Set<String> _pending = <String>{};
  final ScrollController _scroll = ScrollController();
  final TextEditingController _search = TextEditingController();
  ManagerController get manager => widget.controller;
  @override
  void dispose() {
    _scroll.dispose();
    _search.dispose();
    super.dispose();
  }

  Future<void> _run(DownloadJob job, Future<void> Function() action) async {
    if (_pending.contains(job.id)) return;
    setState(() => _pending.add(job.id));
    try {
      await action();
    } finally {
      if (mounted) setState(() => _pending.remove(job.id));
    }
  }

  Future<void> _cancel(DownloadJob job) async {
    final bool? cancel = await showDialog<bool>(
      context: context,
      builder: (BuildContext dialogContext) => AlertDialog(
        title: const Text('Cancel these downloads?'),
        content: Text(
          'Stop unfinished files in ${job.topic?.title ?? job.chat.name}? Files already saved will be kept.',
        ),
        actions: <Widget>[
          TextButton(
            onPressed: () => Navigator.pop(dialogContext, false),
            child: const Text('Keep downloading'),
          ),
          FilledButton(
            onPressed: () => Navigator.pop(dialogContext, true),
            child: const Text('Cancel downloads'),
          ),
        ],
      ),
    );
    if (cancel == true) await _run(job, () => manager.cancelJob(job));
  }

  @override
  Widget build(BuildContext context) {
    final filters = manager.downloadFilters;
    if (_search.text != filters.query) _search.text = filters.query;
    final entries = manager.downloadEntries;
    final filtered = filters.apply(entries);
    final rows = <Object>[];
    for (final group in filters.groups(filtered)) {
      if (filters.groupBy != 'none' && filters.groupBy != 'chat') {
        rows.add(group);
      }
      if (filters.groupBy == 'chat') {
        final batches = <String, List<MediaEntry>>{};
        for (final entry in group.entries) {
          batches.putIfAbsent(entry.job!.id, () => []).add(entry);
        }
        for (final batch in batches.values) {
          rows.add(batch.first.job!);
          if (!_collapsed.contains(batch.first.job!.id)) rows.addAll(batch);
        }
      } else {
        rows.addAll(group.entries);
      }
    }
    if (!filters.narrowed) {
      rows.addAll(manager.jobs.where((job) => job.files.isEmpty));
    }
    return Container(
      decoration: BoxDecoration(
        color: context.quiet.canvas,
        border: Border(top: BorderSide(color: context.quiet.divider)),
      ),
      child: Scrollbar(
        controller: _scroll,
        child: CustomScrollView(
          key: const PageStorageKey<String>('download-list'),
          controller: _scroll,
          slivers: <Widget>[
            SliverToBoxAdapter(
              child: Padding(
                padding: EdgeInsets.fromLTRB(
                  28,
                  widget.compact ? 14 : 26,
                  28,
                  widget.compact ? 10 : 20,
                ),
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: <Widget>[
                    Row(
                      children: [
                        Expanded(
                          child: Text(
                            'Your downloads',
                            style: widget.compact
                                ? Theme.of(context).textTheme.titleMedium
                                : Theme.of(context).textTheme.headlineMedium,
                          ),
                        ),
                        IconButton(
                          tooltip: 'Refresh downloads and check saved files',
                          onPressed: manager.refreshJobs,
                          icon: const Icon(Icons.refresh, size: 20),
                        ),
                      ],
                    ),
                    if (!widget.compact) ...<Widget>[
                      const SizedBox(height: 6),
                      Text(
                        'Progress, saved files, and anything that needs your attention.',
                        style: Theme.of(context).textTheme.bodySmall,
                      ),
                    ],
                    if (manager.jobs.isNotEmpty)
                      Padding(
                        padding: const EdgeInsets.only(top: 6),
                        child: Text(
                          '${filtered.length} of ${entries.length} files · ${manager.activeJobCount} active · ${manager.queuedJobCount} queued',
                          style: Theme.of(context).textTheme.bodySmall,
                        ),
                      ),
                  ],
                ),
              ),
            ),
            SliverToBoxAdapter(
              child: Padding(
                padding: const EdgeInsets.fromLTRB(28, 0, 28, 14),
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.stretch,
                  children: [
                    TextField(
                      controller: _search,
                      onChanged: (v) => filters.change(() => filters.query = v),
                      decoration: InputDecoration(
                        labelText: 'Search downloads',
                        prefixIcon: const Icon(Icons.search, size: 19),
                        suffixIcon: filters.query.isEmpty
                            ? null
                            : IconButton(
                                tooltip: 'Clear download search',
                                onPressed: () =>
                                    filters.change(() => filters.query = ''),
                                icon: const Icon(Icons.close, size: 18),
                              ),
                      ),
                    ),
                    const SizedBox(height: 12),
                    MediaFilterBar(
                      filters: filters,
                      entries: entries,
                      scope: 'downloads',
                      compact: widget.compact,
                    ),
                  ],
                ),
              ),
            ),
            if (rows.isEmpty)
              SliverFillRemaining(
                hasScrollBody: false,
                child: QuietEmptyState(
                  icon: Icons.file_download_outlined,
                  title: filters.narrowed
                      ? 'No matching downloads'
                      : 'Your queue is empty',
                  message: filters.narrowed
                      ? 'Try another filter or clear the current filters.'
                      : 'Select files from your library. Every download will appear here.',
                  action: filters.narrowed
                      ? TextButton(
                          onPressed: filters.reset,
                          child: const Text('Clear filters'),
                        )
                      : null,
                ),
              )
            else
              SliverPadding(
                padding: const EdgeInsets.fromLTRB(20, 0, 20, 20),
                sliver: SliverList.builder(
                  itemCount: rows.length,
                  itemBuilder: (BuildContext context, int index) {
                    final row = rows[index];
                    if (row is MediaGroup) return MediaGroupHeading(row);
                    if (row is DownloadJob) return _header(context, row);
                    final entry = row as MediaEntry;
                    return _file(context, entry.job!, entry.task!);
                  },
                ),
              ),
          ],
        ),
      ),
    );
  }

  Widget _header(BuildContext context, DownloadJob job) {
    final bool pending = _pending.contains(job.id);
    final String count = !job.library
        ? '${job.files.length} discovered files · Existing chat download'
        : job.files.isEmpty
        ? job.state == 'completed'
              ? 'No matching files in this scope'
              : job.state == 'failed'
              ? 'No files queued'
              : 'Finding matching files…'
        : '${job.completedFiles} of ${job.files.length} files saved${job.failedFiles > 0 ? ' · ${job.failedFiles} failed' : ''}';
    return Padding(
      padding: const EdgeInsets.only(top: 10, bottom: 4),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: <Widget>[
          Material(
            color: context.quiet.sidebar,
            borderRadius: BorderRadius.circular(8),
            child: Padding(
              padding: const EdgeInsets.fromLTRB(4, 10, 12, 8),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.stretch,
                children: <Widget>[
                  Row(
                    children: <Widget>[
                      IconButton(
                        tooltip: _collapsed.contains(job.id)
                            ? 'Show file list'
                            : 'Collapse file list',
                        onPressed: () => setState(() {
                          if (!_collapsed.add(job.id)) {
                            _collapsed.remove(job.id);
                          }
                        }),
                        icon: Icon(
                          _collapsed.contains(job.id)
                              ? Icons.chevron_right
                              : Icons.expand_more,
                          size: 19,
                        ),
                      ),
                      Expanded(
                        child: Column(
                          crossAxisAlignment: CrossAxisAlignment.start,
                          children: <Widget>[
                            Tooltip(
                              message:
                                  '${job.chat.name} / ${job.topic?.title ?? 'All media'}',
                              child: Text(
                                job.chat.name,
                                maxLines: 2,
                                overflow: TextOverflow.ellipsis,
                                style: Theme.of(context).textTheme.titleMedium,
                              ),
                            ),
                            const SizedBox(height: 3),
                            Text(
                              '${job.topic?.title ?? 'All media'} · $count',
                              style: Theme.of(context).textTheme.bodySmall,
                            ),
                          ],
                        ),
                      ),
                      const SizedBox(width: 12),
                      StatusLabel(job.state),
                    ],
                  ),
                  if (!job.library)
                    Padding(
                      padding: const EdgeInsets.fromLTRB(14, 8, 0, 4),
                      child: Text(
                        'Individual file progress was not recorded by the previous version.',
                        style: Theme.of(context).textTheme.bodySmall,
                      ),
                    ),
                  Padding(
                    padding: const EdgeInsets.fromLTRB(14, 12, 0, 6),
                    child: QuietProgress(
                      value: job.overallPercent == null
                          ? null
                          : job.overallPercent! / 100,
                      semanticsLabel:
                          '${job.chat.name} overall download progress',
                    ),
                  ),
                  Wrap(
                    alignment: WrapAlignment.end,
                    spacing: 4,
                    crossAxisAlignment: WrapCrossAlignment.center,
                    children: <Widget>[
                      if (pending)
                        const Padding(
                          padding: EdgeInsets.symmetric(horizontal: 8),
                          child: SizedBox.square(
                            dimension: 14,
                            child: QuietSpinner(strokeWidth: 1.5),
                          ),
                        ),
                      if (job.canPause && manager.supportsPause)
                        TextButton.icon(
                          onPressed: pending
                              ? null
                              : () => _run(job, () => manager.pauseJob(job)),
                          icon: const Icon(Icons.pause, size: 17),
                          label: const Text('Pause'),
                        ),
                      if (job.canResume)
                        TextButton.icon(
                          onPressed: pending
                              ? null
                              : () => _run(job, () => manager.resumeJob(job)),
                          icon: const Icon(Icons.play_arrow, size: 17),
                          label: const Text('Resume'),
                        ),
                      if (job.canCancel)
                        TextButton(
                          onPressed: pending ? null : () => _cancel(job),
                          child: const Text('Cancel'),
                        ),
                      if (<String>{'failed', 'cancelled'}.contains(job.state))
                        TextButton.icon(
                          onPressed: pending
                              ? null
                              : () => _run(job, () => manager.retryJob(job)),
                          icon: const Icon(Icons.refresh, size: 17),
                          label: const Text('Retry unfinished'),
                        ),
                    ],
                  ),
                ],
              ),
            ),
          ),
          if (job.errorMessage?.isNotEmpty == true && job.state == 'failed')
            Padding(
              padding: const EdgeInsets.only(top: 8),
              child: QuietNotice(job.errorMessage!),
            ),
        ],
      ),
    );
  }

  Widget _file(
    BuildContext context,
    DownloadJob job,
    FileTask file,
  ) => Container(
    key: ValueKey<String>('queue-file-${job.id}-${file.item.key}'),
    margin: const EdgeInsets.only(left: 12),
    padding: const EdgeInsets.fromLTRB(14, 16, 0, 16),
    decoration: BoxDecoration(
      border: Border(
        left: BorderSide(color: context.quiet.divider),
        bottom: BorderSide(color: context.quiet.divider),
      ),
    ),
    child: LayoutBuilder(
      builder: (BuildContext context, BoxConstraints box) => Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: <Widget>[
          if (box.maxWidth >= 400) ...<Widget>[
            if (file.item.fileType == 'video')
              VideoThumbnail(
                item: file.item,
                cache: manager.thumbnailCache,
                onVisibilityChanged: manager.reportThumbnailVisibility,
                width: box.maxWidth >= 700 ? 96 : 64,
              )
            else
              SizedBox(
                width: box.maxWidth >= 700 ? 96 : 64,
                child: Center(child: FileGlyph(file.item)),
              ),
            const SizedBox(width: 14),
          ],
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: <Widget>[
                Tooltip(
                  message: file.item.originalName,
                  child: Text(
                    file.item.title,
                    maxLines: 2,
                    overflow: TextOverflow.ellipsis,
                    style: Theme.of(context).textTheme.titleMedium,
                  ),
                ),
                if (manager.downloadFilters.groupBy != 'chat')
                  Padding(
                    padding: const EdgeInsets.only(top: 4),
                    child: Text(
                      '${job.chat.name} · ${job.topic?.title ?? 'All media'}',
                      style: Theme.of(context).textTheme.bodySmall,
                    ),
                  ),
                const SizedBox(height: 8),
                if (const {
                  'missing',
                  'unavailable',
                  'unknown',
                  'paused',
                  'cancelled',
                }.contains(
                  MediaEntry(item: file.item, task: file, job: job).status,
                ))
                  StatusLabel(
                    MediaEntry(item: file.item, task: file, job: job).status,
                  )
                else
                  FileTransferProgress(file),
                if (file.error?.isNotEmpty == true)
                  Padding(
                    padding: const EdgeInsets.only(top: 8),
                    child: Text(
                      file.error!,
                      style: Theme.of(context).textTheme.bodySmall!.copyWith(
                        color: context.quiet.error,
                      ),
                    ),
                  ),
              ],
            ),
          ),
          if (manager.downloadFilters.groupBy != 'chat' &&
              (job.canPause ||
                  job.canResume ||
                  job.canCancel ||
                  job.state == 'failed' ||
                  job.state == 'cancelled'))
            PopupMenuButton<String>(
              tooltip: 'Batch download actions',
              enabled: !_pending.contains(job.id),
              onSelected: (action) {
                switch (action) {
                  case 'pause':
                    _run(job, () => manager.pauseJob(job));
                  case 'resume':
                    _run(job, () => manager.resumeJob(job));
                  case 'retry':
                    _run(job, () => manager.retryJob(job));
                  case 'cancel':
                    _cancel(job);
                }
              },
              itemBuilder: (_) => [
                if (job.canPause && manager.supportsPause)
                  const PopupMenuItem(
                    value: 'pause',
                    child: Text('Pause this batch'),
                  ),
                if (job.canResume)
                  const PopupMenuItem(
                    value: 'resume',
                    child: Text('Resume this batch'),
                  ),
                if (job.state == 'failed' || job.state == 'cancelled')
                  const PopupMenuItem(
                    value: 'retry',
                    child: Text('Retry unfinished files'),
                  ),
                if (job.canCancel)
                  const PopupMenuItem(
                    value: 'cancel',
                    child: Text('Cancel this batch'),
                  ),
              ],
            ),
          if (MediaEntry(item: file.item, task: file, job: job).status ==
                  'completed' &&
              file.savedPath?.isNotEmpty == true) ...<Widget>[
            const SizedBox(width: 4),
            IconButton(
              tooltip: 'Open file',
              onPressed: () => manager.openFile(job, file),
              icon: const Icon(Icons.open_in_new, size: 18),
            ),
            IconButton(
              tooltip: 'Show in folder',
              onPressed: () => manager.openFile(job, file, reveal: true),
              icon: const Icon(Icons.folder_open_outlined, size: 19),
            ),
          ],
        ],
      ),
    ),
  );
}

class TransferStrip extends StatelessWidget {
  const TransferStrip({
    super.key,
    required this.controller,
    required this.isOpen,
    required this.isSplit,
    required this.onToggle,
  });
  final ManagerController controller;
  final bool isOpen, isSplit;
  final VoidCallback onToggle;

  @override
  Widget build(BuildContext context) {
    DownloadJob? owner;
    FileTask? current;
    for (final DownloadJob job in controller.jobs) {
      for (final FileTask file in job.files) {
        if (file.state == 'downloading' || file.state == 'finalizing') {
          current = file;
          owner = job;
          break;
        }
      }
      if (current != null) break;
    }
    if (current == null) {
      for (final DownloadJob job in controller.jobs) {
        if (job.state == 'paused') {
          owner = job;
          for (final FileTask file in job.files) {
            if (file.state == 'paused') {
              current = file;
              break;
            }
          }
          break;
        }
      }
    }
    final DownloadJob? job = owner;
    final FileTask? file = current;
    final progress = file?.progress;
    final bool stale =
        progress?.updatedAt == null ||
        DateTime.now().millisecondsSinceEpoch - progress!.updatedAt! > 12000;
    final String subtitle = file == null
        ? '${controller.activeJobCount} active · ${controller.queuedJobCount} queued · ${controller.completedJobCount} completed'
        : file.state == 'paused'
        ? 'Paused · Ready when you are'
        : <String>[
            if (progress?.percent != null)
              '${progress!.percent!.toStringAsFixed(1)}%',
            if ((progress?.bytesDownloaded ?? 0) > 0)
              '${formatBytes(progress!.bytesDownloaded!)}${(progress.totalBytes ?? 0) > 0 ? ' of ${formatBytes(progress.totalBytes!)}' : ''}',
            if (!stale && progress.speedBytesPerSecond != null)
              '${formatBytes(progress.speedBytesPerSecond!)}/s',
            if (!stale && progress.etaSeconds != null)
              '${formatDuration(progress.etaSeconds!)} left',
            if (stale) 'Waiting for progress update',
          ].join(' · ');
    final String tooltip = isOpen
        ? (isSplit ? 'Hide download queue' : 'Back to files')
        : 'View download queue';
    return Container(
      decoration: BoxDecoration(
        color: context.quiet.sidebar,
        border: Border(top: BorderSide(color: context.quiet.divider)),
      ),
      padding: const EdgeInsets.fromLTRB(20, 12, 14, 12),
      child: Row(
        children: <Widget>[
          ExcludeSemantics(
            child: Icon(
              file == null
                  ? Icons.file_download_outlined
                  : file.state == 'paused'
                  ? Icons.pause_circle_outline
                  : Icons.downloading_outlined,
              color: context.quiet.accent,
              size: 22,
            ),
          ),
          const SizedBox(width: 12),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: <Widget>[
                Tooltip(
                  message: file?.item.title ?? 'Download queue',
                  child: Text(
                    file?.item.title ?? 'Download queue',
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                    style: Theme.of(context).textTheme.labelLarge,
                  ),
                ),
                const SizedBox(height: 3),
                Text(
                  subtitle,
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                  style: numberStyle(context),
                ),
                if (file != null && file.state == 'downloading')
                  Padding(
                    padding: const EdgeInsets.only(top: 8),
                    child: QuietProgress(
                      value: file.progress?.percent == null
                          ? null
                          : file.progress!.percent!.clamp(0, 100) / 100,
                      semanticsLabel: 'Current file transfer progress',
                    ),
                  ),
              ],
            ),
          ),
          if (job != null &&
              ((job.canPause && controller.supportsPause) ||
                  job.canResume)) ...<Widget>[
            const SizedBox(width: 8),
            IconButton(
              tooltip: job.canResume
                  ? 'Resume current download'
                  : 'Pause current download',
              onPressed: () => job.canResume
                  ? controller.resumeJob(job)
                  : controller.pauseJob(job),
              icon: Icon(
                job.canResume ? Icons.play_arrow : Icons.pause,
                size: 20,
              ),
            ),
          ],
          const SizedBox(width: 4),
          if (MediaQuery.sizeOf(context).width >= 1100 &&
              MediaQuery.textScalerOf(context).scale(14) <= 19)
            Tooltip(
              message: tooltip,
              child: TextButton(
                key: const ValueKey<String>('queue-toggle'),
                onPressed: onToggle,
                child: Row(
                  mainAxisSize: MainAxisSize.min,
                  children: <Widget>[
                    Text(
                      isOpen
                          ? (isSplit ? 'Hide queue' : 'Back to files')
                          : 'View queue',
                    ),
                    const SizedBox(width: 6),
                    Icon(
                      isOpen
                          ? Icons.keyboard_arrow_down
                          : Icons.keyboard_arrow_up,
                      size: 20,
                    ),
                  ],
                ),
              ),
            )
          else
            IconButton(
              key: const ValueKey<String>('queue-toggle'),
              tooltip: tooltip,
              onPressed: onToggle,
              icon: Icon(
                isOpen ? Icons.keyboard_arrow_down : Icons.keyboard_arrow_up,
                size: 23,
              ),
            ),
        ],
      ),
    );
  }
}

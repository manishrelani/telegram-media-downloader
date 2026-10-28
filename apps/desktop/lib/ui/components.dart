import 'package:flutter/material.dart';

import '../models/media_item.dart';
import '../models/download_job.dart';
import 'quiet_theme.dart';

String formatBytes(int value) {
  const List<String> units = <String>['B', 'KB', 'MB', 'GB', 'TB'];
  double size = value.toDouble();
  int unit = 0;
  while (size >= 1024 && unit < units.length - 1) {
    size /= 1024;
    unit++;
  }
  return '${size.toStringAsFixed(unit == 0 ? 0 : 1)} ${units[unit]}';
}

String formatDuration(int seconds) => seconds < 60
    ? '${seconds}s'
    : seconds < 3600
    ? '${seconds ~/ 60}m ${seconds % 60}s'
    : '${seconds ~/ 3600}h ${(seconds % 3600) ~/ 60}m';
String formatDate(int seconds) {
  final DateTime date = DateTime.fromMillisecondsSinceEpoch(seconds * 1000);
  const List<String> months = <String>[
    'Jan',
    'Feb',
    'Mar',
    'Apr',
    'May',
    'Jun',
    'Jul',
    'Aug',
    'Sep',
    'Oct',
    'Nov',
    'Dec',
  ];
  return '${date.day} ${months[date.month - 1]}${date.year == DateTime.now().year ? '' : ' ${date.year}'}';
}

TextStyle numberStyle(BuildContext context) => Theme.of(context)
    .textTheme
    .bodySmall!
    .copyWith(fontFeatures: const <FontFeature>[FontFeature.tabularFigures()]);

String stateLabel(String state) =>
    const <String, String>{
      'queued': 'Queued',
      'preparing': 'Preparing',
      'downloading': 'Downloading',
      'finalizing': 'Finalizing',
      'completed': 'Saved',
      'paused': 'Paused',
      'pausing': 'Pausing',
      'cancelled': 'Cancelled',
      'cancelling': 'Cancelling',
      'failed': 'Failed',
      'legacy': 'Not tracked',
      'unknown': 'Unknown',
      'missing': 'Missing file',
      'unavailable': 'Location unavailable',
    }[state] ??
    state;

class FileGlyph extends StatelessWidget {
  const FileGlyph(this.item, {super.key});
  final MediaItem item;
  @override
  Widget build(BuildContext context) => ExcludeSemantics(
    child: Container(
      width: 34,
      height: 40,
      alignment: Alignment.center,
      decoration: BoxDecoration(
        color: context.quiet.selected,
        borderRadius: BorderRadius.circular(7),
      ),
      child: Icon(
        switch (item.fileType) {
          'video' => Icons.video_file_outlined,
          'audio' => Icons.audiotrack_outlined,
          'image' => Icons.image_outlined,
          'archive' => Icons.folder_zip_outlined,
          'code' => Icons.code,
          'office' => Icons.table_chart_outlined,
          'ebook' => Icons.menu_book_outlined,
          _ => Icons.description_outlined,
        },
        size: 20,
        color: context.quiet.accent,
      ),
    ),
  );
}

class StatusLabel extends StatelessWidget {
  const StatusLabel(this.state, {super.key, this.showIcon = true});
  final String state;
  final bool showIcon;
  @override
  Widget build(BuildContext context) {
    final QuietColors colors = context.quiet;
    final Color color = switch (state) {
      'completed' || 'downloading' => colors.accent,
      'failed' || 'missing' => colors.error,
      'paused' || 'pausing' || 'unavailable' => colors.warning,
      _ => colors.muted,
    };
    return Row(
      mainAxisSize: MainAxisSize.min,
      children: <Widget>[
        if (showIcon) ...<Widget>[
          ExcludeSemantics(
            child: Icon(
              switch (state) {
                'completed' => Icons.check_circle_outline,
                'failed' || 'missing' || 'unavailable' => Icons.error_outline,
                'paused' || 'pausing' => Icons.pause_circle_outline,
                'downloading' => Icons.downloading_outlined,
                'cancelled' || 'cancelling' => Icons.cancel_outlined,
                _ => Icons.schedule_outlined,
              },
              color: color,
              size: 15,
            ),
          ),
          const SizedBox(width: 5),
        ],
        Text(
          stateLabel(state),
          style: Theme.of(context).textTheme.bodySmall!.copyWith(
            color: color,
            fontWeight: FontWeight.w500,
          ),
        ),
      ],
    );
  }
}

class FileTransferProgress extends StatelessWidget {
  const FileTransferProgress(
    this.file, {
    super.key,
    this.showTrack = true,
    this.compact = false,
  });
  final FileTask file;
  final bool showTrack, compact;
  @override
  Widget build(BuildContext context) {
    final DownloadProgress? progress = file.progress;
    final bool active = file.state == 'downloading';
    final bool stale =
        active &&
        (progress?.updatedAt == null ||
            DateTime.now().millisecondsSinceEpoch - progress!.updatedAt! >
                12000);
    final bool showPercent =
        <String>{'downloading', 'paused', 'finalizing'}.contains(file.state) &&
        progress?.percent != null;
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: <Widget>[
        Wrap(
          spacing: 9,
          runSpacing: 4,
          crossAxisAlignment: WrapCrossAlignment.center,
          children: <Widget>[
            StatusLabel(file.state, showIcon: !compact),
            if (showPercent)
              Text(
                '${progress!.percent!.clamp(0, 100).toStringAsFixed(1)}%',
                style: numberStyle(context).copyWith(
                  color: context.quiet.accent,
                  fontWeight: FontWeight.w500,
                ),
              ),
            if (active && !compact && (progress?.bytesDownloaded ?? 0) > 0)
              Text(
                '${formatBytes(progress!.bytesDownloaded!)}${(progress.totalBytes ?? 0) > 0 ? ' / ${formatBytes(progress.totalBytes!)}' : ''}',
                style: numberStyle(context),
              ),
            if (active &&
                !stale &&
                !compact &&
                progress?.speedBytesPerSecond != null)
              Text(
                '${formatBytes(progress!.speedBytesPerSecond!)}/s',
                style: numberStyle(context),
              ),
            if (active && !stale && !compact && progress?.etaSeconds != null)
              Text(
                '${formatDuration(progress!.etaSeconds!)} left',
                style: numberStyle(context),
              ),
            if (stale && !compact)
              Text(
                progress?.updatedAt == null
                    ? 'Waiting for progress update'
                    : 'Last update ${((DateTime.now().millisecondsSinceEpoch - progress!.updatedAt!) / 1000).round()}s ago',
                style: Theme.of(context).textTheme.bodySmall,
              ),
          ],
        ),
        if (showTrack && (active || file.state == 'finalizing'))
          Padding(
            padding: const EdgeInsets.only(top: 8),
            child: QuietProgress(
              value: progress?.percent == null
                  ? null
                  : progress!.percent!.clamp(0, 100) / 100,
              color: context.quiet.accent,
              semanticsLabel: '${file.item.title} download progress',
              semanticsValue: progress?.percent?.toStringAsFixed(1),
            ),
          ),
      ],
    );
  }
}

class QuietEmptyState extends StatelessWidget {
  const QuietEmptyState({
    super.key,
    required this.icon,
    required this.title,
    required this.message,
    this.action,
  });
  final IconData icon;
  final String title, message;
  final Widget? action;
  @override
  Widget build(BuildContext context) => Center(
    child: SingleChildScrollView(
      child: Padding(
        padding: const EdgeInsets.all(28),
        child: ConstrainedBox(
          constraints: const BoxConstraints(maxWidth: 370),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: <Widget>[
              ExcludeSemantics(
                child: Icon(icon, size: 34, color: context.quiet.accent),
              ),
              const SizedBox(height: 16),
              Text(
                title,
                textAlign: TextAlign.center,
                style: Theme.of(context).textTheme.titleLarge,
              ),
              const SizedBox(height: 8),
              Text(
                message,
                textAlign: TextAlign.center,
                style: Theme.of(
                  context,
                ).textTheme.bodyMedium!.copyWith(color: context.quiet.muted),
              ),
              if (action != null) ...<Widget>[
                const SizedBox(height: 16),
                action!,
              ],
            ],
          ),
        ),
      ),
    ),
  );
}

class QuietNotice extends StatelessWidget {
  const QuietNotice(this.message, {super.key, this.onRetry, this.onDismiss});
  final String message;
  final VoidCallback? onRetry, onDismiss;
  @override
  Widget build(BuildContext context) => Semantics(
    liveRegion: true,
    child: Container(
      padding: const EdgeInsets.fromLTRB(16, 12, 8, 12),
      color: context.quiet.errorSurface,
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: <Widget>[
          Padding(
            padding: const EdgeInsets.only(top: 2),
            child: Icon(
              Icons.error_outline,
              size: 20,
              color: context.quiet.error,
            ),
          ),
          const SizedBox(width: 10),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: <Widget>[
                Text(
                  message,
                  maxLines: 3,
                  overflow: TextOverflow.ellipsis,
                  style: Theme.of(
                    context,
                  ).textTheme.bodySmall!.copyWith(color: context.quiet.error),
                ),
                if (message.length > 160)
                  TextButton(
                    onPressed: () => showDialog<void>(
                      context: context,
                      builder: (BuildContext dialogContext) => AlertDialog(
                        title: const Text('Details'),
                        content: SingleChildScrollView(
                          child: SelectableText(message),
                        ),
                        actions: <Widget>[
                          TextButton(
                            onPressed: () => Navigator.pop(dialogContext),
                            child: const Text('Close'),
                          ),
                        ],
                      ),
                    ),
                    child: const Text('View details'),
                  ),
              ],
            ),
          ),
          if (onRetry != null)
            TextButton(onPressed: onRetry, child: const Text('Retry')),
          if (onDismiss != null)
            IconButton(
              tooltip: 'Dismiss message',
              onPressed: onDismiss,
              icon: const Icon(Icons.close, size: 18),
            ),
        ],
      ),
    ),
  );
}

class LibrarySkeleton extends StatelessWidget {
  const LibrarySkeleton({super.key, required this.scope});
  final String scope;
  @override
  Widget build(BuildContext context) => Semantics(
    label: 'Loading files in $scope',
    liveRegion: true,
    child: ExcludeSemantics(
      child: Column(
        children: <Widget>[
          for (int i = 0; i < 5; i++)
            Container(
              height: 82,
              padding: const EdgeInsets.symmetric(horizontal: 12),
              decoration: BoxDecoration(
                border: Border(
                  bottom: BorderSide(color: context.quiet.divider),
                ),
              ),
              child: Row(
                children: <Widget>[
                  Container(
                    width: 16,
                    height: 16,
                    decoration: BoxDecoration(
                      color: context.quiet.divider,
                      borderRadius: BorderRadius.circular(3),
                    ),
                  ),
                  const SizedBox(width: 16),
                  Container(
                    width: 34,
                    height: 40,
                    decoration: BoxDecoration(
                      color: context.quiet.hover,
                      borderRadius: BorderRadius.circular(7),
                    ),
                  ),
                  const SizedBox(width: 14),
                  Expanded(
                    child: Column(
                      mainAxisAlignment: MainAxisAlignment.center,
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: <Widget>[
                        FractionallySizedBox(
                          widthFactor: i.isEven ? .65 : .8,
                          child: Container(
                            height: 12,
                            color: context.quiet.divider,
                          ),
                        ),
                        const SizedBox(height: 10),
                        FractionallySizedBox(
                          widthFactor: .35,
                          child: Container(
                            height: 8,
                            color: context.quiet.hover,
                          ),
                        ),
                      ],
                    ),
                  ),
                ],
              ),
            ),
        ],
      ),
    ),
  );
}

/// Freeze only indeterminate activity indicators when reduced motion is enabled.
/// Determinate values still update, and route/focus transitions remain functional.
class QuietProgress extends StatelessWidget {
  const QuietProgress({
    super.key,
    this.value,
    this.color,
    this.semanticsLabel,
    this.semanticsValue,
  });
  final double? value;
  final Color? color;
  final String? semanticsLabel, semanticsValue;
  @override
  Widget build(BuildContext context) => TickerMode(
    enabled: !MediaQuery.disableAnimationsOf(context),
    child: LinearProgressIndicator(
      value: value,
      color: color,
      semanticsLabel: semanticsLabel,
      semanticsValue: semanticsValue,
    ),
  );
}

class QuietSpinner extends StatelessWidget {
  const QuietSpinner({super.key, this.strokeWidth = 1.5, this.color});
  final double strokeWidth;
  final Color? color;
  @override
  Widget build(BuildContext context) => TickerMode(
    enabled: !MediaQuery.disableAnimationsOf(context),
    child: CircularProgressIndicator(strokeWidth: strokeWidth, color: color),
  );
}

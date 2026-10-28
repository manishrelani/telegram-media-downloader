import 'dart:typed_data';
import 'package:flutter/material.dart';
import '../models/media_item.dart';
import '../services/thumbnail_cache.dart';
import 'quiet_theme.dart';
import 'thumbnail_viewport_observer.dart';

class VideoThumbnail extends StatefulWidget {
  const VideoThumbnail({
    super.key,
    required this.item,
    required this.cache,
    this.width = 96,
    this.onVisibilityChanged,
  });
  final MediaItem item;
  final ThumbnailCache cache;
  final double width;
  final void Function(Object, MediaItem, bool, double)? onVisibilityChanged;
  @override
  State<VideoThumbnail> createState() => _VideoThumbnailState();
}

class _VideoThumbnailState extends State<VideoThumbnail> {
  final Object _visibilityOwner = Object();
  int _generation = 0;
  late Future<Uint8List?> _preview = widget.cache.load(widget.item);
  @override
  void didUpdateWidget(covariant VideoThumbnail oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget.item.key != widget.item.key ||
        oldWidget.item.hasSpoiler != widget.item.hasSpoiler ||
        oldWidget.item.date != widget.item.date ||
        oldWidget.item.thumbnail != widget.item.thumbnail ||
        oldWidget.cache != widget.cache) {
      _generation++;
      _preview = widget.cache.load(widget.item);
    }
  }

  String _duration(double value) {
    final seconds = value.round();
    return seconds >= 3600
        ? '${seconds ~/ 3600}:${((seconds % 3600) ~/ 60).toString().padLeft(2, '0')}:${(seconds % 60).toString().padLeft(2, '0')}'
        : '${seconds ~/ 60}:${(seconds % 60).toString().padLeft(2, '0')}';
  }

  @override
  Widget build(BuildContext context) {
    final content = ExcludeSemantics(
      child: SizedBox(
        width: widget.width,
        height: widget.width * 9 / 16,
        child: ClipRRect(
          borderRadius: BorderRadius.circular(6),
          child: Stack(
            fit: StackFit.expand,
            children: [
              ColoredBox(
                color: context.quiet.sidebar,
                child: Icon(
                  Icons.play_circle_outline,
                  size: 24,
                  color: context.quiet.muted,
                ),
              ),
              FutureBuilder<Uint8List?>(
                key: ValueKey(_generation),
                future: _preview,
                builder: (context, snapshot) {
                  final bytes = snapshot.data;
                  final size = bytes == null
                      ? null
                      : ThumbnailCache.pixelSize(bytes);
                  if (widget.item.hasSpoiler ||
                      size == null ||
                      size.$1 < widget.width ||
                      size.$2 < widget.width * 9 / 16) {
                    return const SizedBox.shrink();
                  }
                  return Image.memory(
                    bytes!,
                    fit: BoxFit.cover,
                    cacheWidth:
                        (widget.width * MediaQuery.devicePixelRatioOf(context))
                            .ceil()
                            .clamp(1, 1280),
                    filterQuality: FilterQuality.high,
                    gaplessPlayback: false,
                    errorBuilder: (_, _, _) => const SizedBox.shrink(),
                  );
                },
              ),
              if (widget.item.durationSeconds > 0)
                Positioned(
                  right: 4,
                  bottom: 4,
                  child: Container(
                    padding: const EdgeInsets.symmetric(
                      horizontal: 4,
                      vertical: 1,
                    ),
                    decoration: BoxDecoration(
                      color: const Color(0xD9202723),
                      borderRadius: BorderRadius.circular(3),
                    ),
                    child: Text(
                      _duration(widget.item.durationSeconds),
                      style: const TextStyle(
                        color: Colors.white,
                        fontSize: 10,
                        height: 1.3,
                        fontWeight: FontWeight.w500,
                      ),
                    ),
                  ),
                ),
            ],
          ),
        ),
      ),
    );
    if (widget.onVisibilityChanged == null) return content;
    return ThumbnailViewportObserver(
      onVisibilityChanged: (value) => widget.onVisibilityChanged?.call(
        _visibilityOwner,
        widget.item,
        value.visible,
        value.position,
      ),
      child: content,
    );
  }
}

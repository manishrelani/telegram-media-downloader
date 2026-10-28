import 'package:flutter/material.dart';

import '../controllers/manager_controller.dart';
import '../models/chat.dart';
import '../models/download_job.dart';
import '../models/media_item.dart';
import '../models/media_filters.dart';
import 'media_filter_bar.dart';
import 'video_thumbnail.dart';
import 'components.dart';
import 'quiet_theme.dart';

class LibraryView extends StatefulWidget {
  const LibraryView({
    super.key,
    required this.controller,
    required this.searchFocus,
    required this.onChooseFolder,
    required this.onDownload,
    required this.onEntireScope,
    this.destinationError,
  });
  final ManagerController controller;
  final FocusNode searchFocus;
  final VoidCallback onChooseFolder, onDownload, onEntireScope;
  final String? destinationError;
  @override
  State<LibraryView> createState() => _LibraryViewState();
}

class _LibraryViewState extends State<LibraryView> {
  final TextEditingController _search = TextEditingController();
  final Map<String, ScrollController> _scrolls = <String, ScrollController>{};
  String? _scope;
  ManagerController get manager => widget.controller;
  @override
  void dispose() {
    _search.dispose();
    for (final ScrollController controller in _scrolls.values) {
      controller.dispose();
    }
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final TelegramChat? chat = manager.activeChat;
    if (chat == null) {
      return ColoredBox(
        color: context.quiet.canvas,
        child: QuietEmptyState(
          icon: Icons.video_library_outlined,
          title: 'Your library, ready to explore',
          message: manager.connected
              ? 'Choose a chat, open a topic, and collect the files you need.'
              : 'Connect to Telegram to browse your chats and topics.',
          action: manager.connected
              ? null
              : FilledButton.icon(
                  onPressed: manager.isConnecting ? null : manager.initialize,
                  icon: const Icon(Icons.link, size: 18),
                  label: Text(
                    manager.isConnecting
                        ? 'Connecting…'
                        : 'Connect to Telegram',
                  ),
                ),
        ),
      );
    }
    final String scope = '${chat.id}:${manager.activeTopic?.id ?? 0}';
    if (_scope != scope) {
      _scope = scope;
      _search.text = manager.mediaSearch;
    }
    final ScrollController scroll = _scrolls.putIfAbsent(
      scope,
      ScrollController.new,
    );
    if (_search.text != manager.mediaSearch) _search.text = manager.mediaSearch;
    final allEntries = manager.libraryEntries;
    final filtered = manager.libraryFilters.apply(allEntries);
    final items = filtered.map((e) => e.item).toList();
    final rows = <Object>[
      for (final group in manager.libraryFilters.groups(filtered)) ...[
        if (manager.libraryFilters.groupBy != 'none') group,
        ...group.entries,
      ],
    ];
    return ColoredBox(
      color: context.quiet.canvas,
      child: LayoutBuilder(
        builder: (BuildContext context, BoxConstraints box) {
          final bool wide =
              box.maxWidth >= 720 &&
              MediaQuery.textScalerOf(context).scale(14) <= 19;
          final double gutter = box.maxWidth < 480 ? 16 : 28;
          return Column(
            children: <Widget>[
              Expanded(
                child: Scrollbar(
                  controller: scroll,
                  child: CustomScrollView(
                    key: ValueKey<String>('media-scroll-$scope'),
                    controller: scroll,
                    slivers: <Widget>[
                      SliverToBoxAdapter(
                        child: Padding(
                          padding: EdgeInsets.fromLTRB(gutter, 26, gutter, 16),
                          child: _heading(context, box.maxWidth),
                        ),
                      ),
                      SliverToBoxAdapter(
                        child: Padding(
                          padding: EdgeInsets.fromLTRB(gutter, 0, gutter, 18),
                          child: _toolbar(context),
                        ),
                      ),
                      SliverToBoxAdapter(
                        child: Padding(
                          padding: EdgeInsets.fromLTRB(gutter, 0, gutter, 14),
                          child: MediaFilterBar(
                            filters: manager.libraryFilters,
                            entries: allEntries,
                            scope: 'library',
                          ),
                        ),
                      ),
                      if (manager.mediaError != null)
                        SliverToBoxAdapter(
                          child: Padding(
                            padding: EdgeInsets.fromLTRB(gutter, 0, gutter, 14),
                            child: QuietNotice(
                              manager.mediaError!,
                              onRetry: () => manager.loadMedia(refresh: true),
                            ),
                          ),
                        ),
                      SliverToBoxAdapter(
                        child: Padding(
                          padding: EdgeInsets.symmetric(horizontal: gutter),
                          child: _tableHeader(context, items, wide),
                        ),
                      ),
                      if (manager.isLoadingMedia && manager.mediaPage == null)
                        SliverToBoxAdapter(
                          child: Padding(
                            padding: EdgeInsets.symmetric(horizontal: gutter),
                            child: LibrarySkeleton(scope: manager.scopeName),
                          ),
                        )
                      else if (items.isEmpty)
                        SliverFillRemaining(
                          hasScrollBody: false,
                          child: QuietEmptyState(
                            icon: manager.fileTypes.isEmpty
                                ? Icons.filter_alt_outlined
                                : Icons.search,
                            title: manager.fileTypes.isEmpty
                                ? 'Choose your file types'
                                : 'No matching files',
                            message: manager.fileTypes.isEmpty
                                ? 'Select one or more types in the File types menu.'
                                : manager.libraryFilters.narrowed
                                ? 'Try another filter or clear the current filters. Results include loaded files only.'
                                : 'Try other file types, or load older media.',
                            action: manager.mediaPage?.hasMore == true
                                ? TextButton(
                                    onPressed: manager.isLoadingMedia
                                        ? null
                                        : () => manager.loadMedia(
                                            limit:
                                                manager.mediaPage!.limit + 100,
                                          ),
                                    child: const Text('Load older files'),
                                  )
                                : null,
                          ),
                        )
                      else
                        SliverPadding(
                          padding: EdgeInsets.symmetric(horizontal: gutter),
                          sliver: SliverList.builder(
                            itemCount: rows.length,
                            itemBuilder: (BuildContext context, int index) {
                              final row = rows[index];
                              if (row is MediaGroup) {
                                return MediaGroupHeading(row);
                              }
                              final entry = row as MediaEntry;
                              return _fileRow(
                                context,
                                entry.item,
                                entry.task,
                                entry.job,
                                wide,
                              );
                            },
                          ),
                        ),
                      if (items.isNotEmpty &&
                          manager.mediaPage?.hasMore == true)
                        SliverToBoxAdapter(
                          child: Padding(
                            padding: const EdgeInsets.symmetric(vertical: 14),
                            child: Center(
                              child: TextButton.icon(
                                onPressed: manager.isLoadingMedia
                                    ? null
                                    : () => manager.loadMedia(
                                        limit: manager.mediaPage!.limit + 100,
                                      ),
                                icon: manager.isLoadingMedia
                                    ? const SizedBox.square(
                                        dimension: 14,
                                        child: QuietSpinner(strokeWidth: 1.5),
                                      )
                                    : const Icon(Icons.expand_more, size: 18),
                                label: Text(
                                  manager.isLoadingMedia
                                      ? 'Loading older files…'
                                      : 'Load older files',
                                ),
                              ),
                            ),
                          ),
                        ),
                      const SliverToBoxAdapter(child: SizedBox(height: 16)),
                    ],
                  ),
                ),
              ),
              _selectionBar(context, gutter),
            ],
          );
        },
      ),
    );
  }

  Widget _heading(BuildContext context, double width) {
    final String count = manager.mediaPage == null
        ? (manager.isLoadingMedia
              ? 'Loading media…'
              : 'Browse videos, documents, and more')
        : '${manager.visibleMedia.length} shown · ${manager.mediaPage!.items.length} loaded${manager.mediaPage!.hasMore ? ' · Older files available' : ''}';
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: <Widget>[
        Row(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: <Widget>[
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: <Widget>[
                  Text(
                    manager.activeChat!.name,
                    maxLines: 2,
                    overflow: TextOverflow.ellipsis,
                    style: Theme.of(context).textTheme.bodySmall,
                  ),
                  const SizedBox(height: 6),
                  Tooltip(
                    message: manager.scopeName,
                    child: Text(
                      manager.scopeName,
                      style: Theme.of(context).textTheme.headlineMedium,
                    ),
                  ),
                  const SizedBox(height: 8),
                  Semantics(
                    liveRegion: true,
                    child: Text(
                      count,
                      style: Theme.of(context).textTheme.bodySmall,
                    ),
                  ),
                ],
              ),
            ),
            const SizedBox(width: 12),
            if (width >= 570 &&
                MediaQuery.textScalerOf(context).scale(14) <= 19)
              OutlinedButton.icon(
                onPressed:
                    manager.isStartingDownloads ||
                        manager.fileTypes.isEmpty ||
                        !manager.connected
                    ? null
                    : widget.onEntireScope,
                icon: const Icon(Icons.download_outlined, size: 17),
                label: Text(
                  manager.activeTopic == null
                      ? 'Download chat'
                      : 'Download topic',
                ),
              )
            else
              IconButton(
                tooltip: manager.activeTopic == null
                    ? 'Download entire chat'
                    : 'Download entire topic',
                onPressed:
                    manager.isStartingDownloads ||
                        manager.fileTypes.isEmpty ||
                        !manager.connected
                    ? null
                    : widget.onEntireScope,
                icon: const Icon(Icons.download_outlined),
              ),
          ],
        ),
        const SizedBox(height: 12),
        SizedBox(
          height: 2,
          child: manager.isLoadingMedia && manager.mediaPage != null
              ? const QuietProgress()
              : null,
        ),
      ],
    );
  }

  Widget _toolbar(BuildContext context) => LayoutBuilder(
    builder: (BuildContext context, BoxConstraints box) {
      final Widget search = TextField(
        controller: _search,
        focusNode: widget.searchFocus,
        onChanged: manager.setMediaSearch,
        decoration: InputDecoration(
          labelText: 'Search loaded files',
          hintText: 'Filename or caption',
          prefixIcon: const Icon(Icons.search, size: 19),
          suffixIcon: manager.mediaSearch.isEmpty
              ? null
              : IconButton(
                  tooltip: 'Clear file search',
                  onPressed: () {
                    _search.clear();
                    manager.setMediaSearch('');
                    widget.searchFocus.requestFocus();
                  },
                  icon: const Icon(Icons.close, size: 18),
                ),
        ),
      );
      final Widget controls = Row(
        mainAxisSize: MainAxisSize.min,
        children: <Widget>[
          FileTypeMenu(controller: manager),
          const SizedBox(width: 4),
          IconButton(
            tooltip: 'Refresh media',
            onPressed: manager.isLoadingMedia
                ? null
                : () => manager.loadMedia(refresh: true),
            icon: const Icon(Icons.refresh, size: 20),
          ),
        ],
      );
      if (box.maxWidth < 480 ||
          MediaQuery.textScalerOf(context).scale(14) > 20) {
        return Column(
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: <Widget>[
            search,
            const SizedBox(height: 10),
            Align(alignment: Alignment.centerRight, child: controls),
          ],
        );
      }
      return Row(
        children: <Widget>[
          Expanded(child: search),
          const SizedBox(width: 12),
          controls,
        ],
      );
    },
  );

  Widget _tableHeader(BuildContext context, List<MediaItem> items, bool wide) {
    final int selected = items
        .where((MediaItem item) => manager.selectedMediaKeys.contains(item.key))
        .length;
    return Container(
      decoration: BoxDecoration(
        border: Border(bottom: BorderSide(color: context.quiet.divider)),
      ),
      child: Row(
        children: <Widget>[
          Tooltip(
            message: 'Select shown files only',
            child: Checkbox(
              tristate: true,
              semanticLabel: 'Select shown files',
              value: selected == 0
                  ? false
                  : selected == items.length
                  ? true
                  : null,
              onChanged: items.isEmpty
                  ? null
                  : (bool? _) =>
                        manager.selectVisibleMedia(selected != items.length),
            ),
          ),
          SizedBox(width: wide ? 110 : 78),
          Expanded(
            child: Text('File', style: Theme.of(context).textTheme.labelMedium),
          ),
          if (wide) ...<Widget>[
            SizedBox(
              width: 90,
              child: Text(
                'Size',
                style: Theme.of(context).textTheme.labelMedium,
              ),
            ),
            SizedBox(
              width: 90,
              child: Text(
                'Added',
                style: Theme.of(context).textTheme.labelMedium,
              ),
            ),
          ],
          const SizedBox(width: 44),
        ],
      ),
    );
  }

  Widget _fileRow(
    BuildContext context,
    MediaItem item,
    FileTask? task,
    DownloadJob? owner,
    bool wide,
  ) {
    final bool selected = manager.selectedMediaKeys.contains(item.key);
    final entry = MediaEntry(item: item, task: task, job: owner);
    final status = entry.status;
    final List<String> metadata = <String>[
      if (item.extension.isNotEmpty) item.extension.toUpperCase(),
      if (item.durationSeconds > 0)
        formatDuration(item.durationSeconds.round()),
      if (!wide && item.sizeBytes > 0) formatBytes(item.sizeBytes),
      if (!wide && item.date > 0) formatDate(item.date),
    ];
    Widget action = const SizedBox(width: 40);
    if (status == 'completed' && owner != null) {
      action = IconButton(
        tooltip: 'Show in folder',
        onPressed: () => manager.openFile(owner, task!, reveal: true),
        icon: const Icon(Icons.folder_open_outlined, size: 19),
      );
    } else if (task == null ||
        <String>{
          'failed',
          'cancelled',
          'unknown',
          'missing',
        }.contains(status)) {
      action = IconButton(
        tooltip: 'Download this file',
        onPressed: manager.isStartingDownloads || !manager.connected
            ? null
            : () => manager.startMediaDownload(only: item),
        icon: const Icon(Icons.file_download_outlined, size: 19),
      );
    } else if (owner != null &&
        manager.supportsPause &&
        owner.canPause &&
        task.state == 'downloading') {
      action = IconButton(
        tooltip: 'Pause download',
        onPressed: () => manager.pauseJob(owner),
        icon: const Icon(Icons.pause, size: 19),
      );
    }
    return Semantics(
      container: true,
      selected: selected,
      child: Material(
        key: ValueKey<String>('file-${item.key}'),
        color: selected ? context.quiet.selected : context.quiet.canvas,
        child: InkWell(
          canRequestFocus: false,
          excludeFromSemantics: true,
          hoverColor: context.quiet.hover,
          onTap: () => manager.toggleMedia(item, !selected),
          child: Container(
            constraints: const BoxConstraints(minHeight: 80),
            padding: const EdgeInsets.symmetric(vertical: 14),
            decoration: BoxDecoration(
              border: Border(bottom: BorderSide(color: context.quiet.divider)),
            ),
            child: Row(
              crossAxisAlignment: CrossAxisAlignment.center,
              children: <Widget>[
                Checkbox(
                  value: selected,
                  semanticLabel: 'Select ${item.title}',
                  onChanged: (bool? value) =>
                      manager.toggleMedia(item, value ?? false),
                ),
                if (item.fileType == 'video')
                  VideoThumbnail(
                    item: item,
                    cache: manager.thumbnailCache,
                    onVisibilityChanged: manager.reportThumbnailVisibility,
                    width: wide ? 96 : 64,
                  )
                else
                  SizedBox(
                    width: wide ? 96 : 64,
                    child: Center(child: FileGlyph(item)),
                  ),
                const SizedBox(width: 14),
                Expanded(
                  child: Padding(
                    padding: const EdgeInsets.only(right: 12),
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: <Widget>[
                        Tooltip(
                          message: '${item.title}\n${item.originalName}',
                          child: Text(
                            item.title,
                            maxLines: 2,
                            overflow: TextOverflow.ellipsis,
                            style: Theme.of(context).textTheme.titleMedium,
                          ),
                        ),
                        if (task == null || task.state == 'legacy') ...<Widget>[
                          const SizedBox(height: 4),
                          Text(
                            metadata.isEmpty
                                ? item.originalName
                                : metadata.join(' · '),
                            style: Theme.of(context).textTheme.bodySmall,
                          ),
                        ] else ...<Widget>[
                          const SizedBox(height: 5),
                          if (status == 'missing' ||
                              status == 'unavailable' ||
                              status == 'unknown')
                            StatusLabel(status)
                          else
                            FileTransferProgress(task, compact: true),
                        ],
                        if (task?.state == 'failed' &&
                            task?.error?.isNotEmpty == true)
                          Padding(
                            padding: const EdgeInsets.only(top: 5),
                            child: Tooltip(
                              message: task!.error!,
                              child: Text(
                                task.error!,
                                maxLines: 2,
                                overflow: TextOverflow.ellipsis,
                                style: Theme.of(context).textTheme.bodySmall!
                                    .copyWith(color: context.quiet.error),
                              ),
                            ),
                          ),
                      ],
                    ),
                  ),
                ),
                if (wide) ...<Widget>[
                  SizedBox(
                    width: 90,
                    child: Text(
                      item.sizeBytes > 0 ? formatBytes(item.sizeBytes) : '—',
                      style: numberStyle(context),
                    ),
                  ),
                  SizedBox(
                    width: 90,
                    child: Text(
                      item.date > 0 ? formatDate(item.date) : '—',
                      style: Theme.of(context).textTheme.bodySmall,
                    ),
                  ),
                ],
                SizedBox(width: 44, child: action),
              ],
            ),
          ),
        ),
      ),
    );
  }

  Widget _selectionBar(BuildContext context, double gutter) {
    final int selected = manager.selectedMediaCount;
    final shown = manager.visibleMedia.map((item) => item.key).toSet();
    final hidden = manager.selectedMediaKeys
        .where((key) => !shown.contains(key))
        .length;
    final bool enabled =
        selected > 0 &&
        manager.connected &&
        manager.managedDownloadsSupported &&
        !manager.isStartingDownloads;
    final String folder = manager.destinationRoot.isEmpty
        ? 'Choose save folder'
        : manager.destinationRoot
                  .split(RegExp(r'[/\\]'))
                  .where((String part) => part.isNotEmpty)
                  .lastOrNull ??
              manager.destinationRoot;
    final Widget destination = Tooltip(
      message: manager.destinationRoot.isEmpty
          ? 'Select where downloads will be saved'
          : manager.destinationRoot,
      child: TextButton.icon(
        onPressed: widget.onChooseFolder,
        icon: const Icon(Icons.folder_open_outlined, size: 17),
        label: ConstrainedBox(
          constraints: const BoxConstraints(maxWidth: 165),
          child: Text(
            folder,
            maxLines: 1,
            overflow: TextOverflow.ellipsis,
            style: Theme.of(context).textTheme.bodySmall!.copyWith(
              color: widget.destinationError != null
                  ? context.quiet.error
                  : context.quiet.muted,
            ),
          ),
        ),
      ),
    );
    final Widget download = Tooltip(
      message: !manager.connected
          ? 'Connect to Telegram to download'
          : selected == 0
          ? 'Select one or more files'
          : 'Download the selected files',
      child: FilledButton.icon(
        key: const ValueKey<String>('download-selection'),
        onPressed: enabled ? widget.onDownload : null,
        icon: manager.isStartingDownloads
            ? SizedBox.square(
                dimension: 16,
                child: QuietSpinner(
                  strokeWidth: 1.5,
                  color: context.quiet.onAccent,
                ),
              )
            : const Icon(Icons.file_download_outlined, size: 17),
        label: Text(
          manager.isStartingDownloads
              ? 'Adding to queue…'
              : selected == 0
              ? 'Download selected'
              : 'Download $selected ${selected == 1 ? 'file' : 'files'}',
        ),
      ),
    );
    return Container(
      padding: EdgeInsets.fromLTRB(gutter, 10, gutter, 12),
      decoration: BoxDecoration(
        color: context.quiet.canvas,
        border: Border(top: BorderSide(color: context.quiet.divider)),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        mainAxisSize: MainAxisSize.min,
        children: <Widget>[
          LayoutBuilder(
            builder: (BuildContext context, BoxConstraints box) {
              final Widget info = Wrap(
                spacing: 8,
                crossAxisAlignment: WrapCrossAlignment.center,
                children: <Widget>[
                  Semantics(
                    liveRegion: true,
                    child: Text(
                      hidden > 0
                          ? '$selected selected · $hidden hidden by filters'
                          : '$selected selected',
                      style: Theme.of(context).textTheme.bodySmall,
                    ),
                  ),
                  if (hidden > 0)
                    TextButton(
                      onPressed: manager.clearMediaSelection,
                      child: const Text('Clear selection'),
                    ),
                  destination,
                ],
              );
              if (box.maxWidth < 500 ||
                  MediaQuery.textScalerOf(context).scale(14) > 19) {
                return Column(
                  crossAxisAlignment: CrossAxisAlignment.stretch,
                  children: <Widget>[
                    info,
                    const SizedBox(height: 6),
                    Align(alignment: Alignment.centerRight, child: download),
                  ],
                );
              }
              return Row(
                children: <Widget>[
                  Expanded(child: info),
                  const SizedBox(width: 16),
                  download,
                ],
              );
            },
          ),
          if (widget.destinationError != null)
            Padding(
              padding: const EdgeInsets.only(top: 4),
              child: Semantics(
                liveRegion: true,
                child: Text(
                  widget.destinationError!,
                  style: Theme.of(
                    context,
                  ).textTheme.bodySmall!.copyWith(color: context.quiet.error),
                ),
              ),
            ),
        ],
      ),
    );
  }
}

class FileTypeMenu extends StatefulWidget {
  const FileTypeMenu({super.key, required this.controller});
  final ManagerController controller;
  @override
  State<FileTypeMenu> createState() => _FileTypeMenuState();
}

class _FileTypeMenuState extends State<FileTypeMenu> {
  final FocusNode _focus = FocusNode(debugLabel: 'File type filter');
  @override
  void dispose() {
    _focus.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final ManagerController manager = widget.controller;
    final bool all = manager.fileTypes.length == fileTypeLabels.length;
    final String label = all
        ? 'All file types'
        : manager.fileTypes.isEmpty
        ? 'File types'
        : manager.fileTypes.length == 1
        ? fileTypeLabels[manager.fileTypes.single]!
        : '${fileTypeLabels[manager.fileTypes.first]} +${manager.fileTypes.length - 1}';
    return MenuAnchor(
      childFocusNode: _focus,
      menuChildren: <Widget>[
        CheckboxMenuButton(
          value: all,
          closeOnActivate: false,
          onChanged: (bool? value) =>
              manager.toggleFileType('all', value ?? false),
          child: const Text('All files'),
        ),
        const Divider(),
        for (final entry in fileTypeLabels.entries)
          CheckboxMenuButton(
            value: manager.fileTypes.contains(entry.key),
            closeOnActivate: false,
            onChanged: (bool? value) =>
                manager.toggleFileType(entry.key, value ?? false),
            child: Text(entry.value),
          ),
      ],
      builder: (BuildContext context, MenuController menu, Widget? child) =>
          OutlinedButton.icon(
            key: const ValueKey<String>('file-type-menu'),
            focusNode: _focus,
            onPressed: () {
              if (menu.isOpen) {
                menu.close();
              } else {
                menu.open();
              }
            },
            icon: const Icon(Icons.tune, size: 17),
            label: Text(label),
          ),
    );
  }
}

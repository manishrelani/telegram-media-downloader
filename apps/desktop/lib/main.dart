import 'dart:async';
import 'dart:io';

import 'package:file_selector/file_selector.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';

import 'controllers/manager_controller.dart';
import 'models/chat.dart';
import 'models/media_item.dart';
import 'services/sidebar_width_store.dart';
import 'ui/components.dart';
import 'ui/downloads_view.dart';
import 'ui/library_view.dart';
import 'ui/quiet_theme.dart';
import 'ui/resizable_sidebar.dart';

void main() {
  WidgetsFlutterBinding.ensureInitialized();
  runApp(const TelegramMediaDownloaderApp());
}

class TelegramMediaDownloaderApp extends StatelessWidget {
  const TelegramMediaDownloaderApp({
    super.key,
    this.autoConnect = true,
    this.controller,
    this.themeMode = ThemeMode.system,
    this.textScaler,
    this.sidebarWidthStore,
  });
  final bool autoConnect;
  final ManagerController? controller;
  final ThemeMode themeMode;
  final TextScaler? textScaler;
  final SidebarWidthStore? sidebarWidthStore;
  @override
  Widget build(BuildContext context) => MaterialApp(
    title: 'Telegram Media Downloader',
    debugShowCheckedModeBanner: false,
    theme: QuietTheme.light,
    darkTheme: QuietTheme.dark,
    themeMode: themeMode,
    themeAnimationDuration:
        WidgetsBinding
            .instance
            .platformDispatcher
            .accessibilityFeatures
            .disableAnimations
        ? Duration.zero
        : const Duration(milliseconds: 180),
    builder: textScaler == null
        ? null
        : (BuildContext context, Widget? child) => MediaQuery(
            data: MediaQuery.of(context).copyWith(textScaler: textScaler),
            child: child!,
          ),
    home: ManagerHomePage(
      autoConnect: autoConnect,
      controller: controller,
      sidebarWidthStore: sidebarWidthStore,
    ),
  );
}

class ManagerHomePage extends StatefulWidget {
  const ManagerHomePage({
    super.key,
    this.autoConnect = true,
    this.controller,
    this.sidebarWidthStore,
  });
  final bool autoConnect;
  final ManagerController? controller;
  final SidebarWidthStore? sidebarWidthStore;
  @override
  State<ManagerHomePage> createState() => _ManagerHomePageState();
}

class _ManagerHomePageState extends State<ManagerHomePage> {
  late final ManagerController _controller =
      widget.controller ?? ManagerController();
  final GlobalKey<ScaffoldState> _scaffold = GlobalKey<ScaffoldState>();
  final FocusNode _fileSearch = FocusNode(debugLabel: 'Search library files');
  final FocusNode _chatSearch = FocusNode(debugLabel: 'Search chats');
  final Set<int> _expanded = <int>{};
  bool _queueOpen = false;
  String? _destinationError;
  Timer? _clock;
  late final SidebarWidthStore _sidebarStore =
      widget.sidebarWidthStore ?? FileSidebarWidthStore();
  double _sidebarWidth = sidebarDefaultWidth;
  bool _sidebarTouched = false;
  bool _saveWarningShown = false;

  Future<void> _restoreSidebarWidth() async {
    try {
      final width = await _sidebarStore.loadWidth();
      if (mounted && !_sidebarTouched && width != null && width.isFinite) {
        setState(
          () => _sidebarWidth = width.clamp(sidebarMinWidth, sidebarMaxWidth),
        );
      }
    } catch (_) {
      // A missing or inaccessible preference must never prevent opening the app.
    }
  }

  void _resizeSidebar(double width) {
    _sidebarTouched = true;
    setState(() => _sidebarWidth = width);
  }

  Future<void> _saveSidebarWidth() async {
    try {
      await _sidebarStore.saveWidth(_sidebarWidth);
    } catch (_) {
      if (!mounted || _saveWarningShown) return;
      _saveWarningShown = true;
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(
          content: Text('Sidebar resized, but its width could not be saved.'),
        ),
      );
    }
  }

  void _resetSidebarWidth() {
    _resizeSidebar(sidebarDefaultWidth);
    unawaited(_saveSidebarWidth());
  }

  @override
  void initState() {
    super.initState();
    unawaited(_restoreSidebarWidth());
    if (_controller.activeChat != null) {
      _expanded.add(_controller.activeChat!.id);
    }
    if (widget.autoConnect) {
      WidgetsBinding.instance.addPostFrameCallback(
        (_) => _controller.initialize(),
      );
    }
    _clock = Timer.periodic(const Duration(seconds: 2), (_) {
      if (mounted && _controller.activeJobCount > 0) setState(() {});
    });
  }

  @override
  void dispose() {
    _clock?.cancel();
    _fileSearch.dispose();
    _chatSearch.dispose();
    if (widget.controller == null) _controller.dispose();
    super.dispose();
  }

  void _findFiles() {
    if (_controller.activeChat == null) {
      if (MediaQuery.sizeOf(context).width < 760) {
        _scaffold.currentState?.openDrawer();
      }
      WidgetsBinding.instance.addPostFrameCallback((_) {
        if (mounted) _chatSearch.requestFocus();
      });
      return;
    }
    if (_queueOpen) setState(() => _queueOpen = false);
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (!mounted) return;
      _fileSearch.requestFocus();
      final BuildContext? fieldContext = _fileSearch.context;
      if (fieldContext != null) {
        Scrollable.ensureVisible(
          fieldContext,
          duration: quietMotion(context),
          alignment: .1,
        );
      }
    });
  }

  @override
  Widget build(BuildContext context) => AnimatedBuilder(
    animation: _controller,
    builder: (BuildContext context, Widget? child) => CallbackShortcuts(
      bindings: <ShortcutActivator, VoidCallback>{
        const SingleActivator(LogicalKeyboardKey.keyF, meta: true): _findFiles,
        const SingleActivator(LogicalKeyboardKey.keyF, control: true):
            _findFiles,
      },
      child: Focus(
        autofocus: true,
        skipTraversal: true,
        child: LayoutBuilder(
          builder: (BuildContext context, BoxConstraints box) {
            final bool compact =
                box.maxWidth < 760 ||
                MediaQuery.textScalerOf(context).scale(14) > 23;
            final maximumSidebarWidth = sidebarMaximumFor(box.maxWidth);
            final sidebarWidth = _sidebarWidth.clamp(
              sidebarMinWidth,
              maximumSidebarWidth,
            );
            return Scaffold(
              key: _scaffold,
              drawer: compact
                  ? Drawer(
                      backgroundColor: context.quiet.sidebar,
                      child: SafeArea(child: _sidebar(context)),
                    )
                  : null,
              body: SafeArea(
                child: Column(
                  children: <Widget>[
                    _header(context, compact),
                    if (_controller.errorMessage != null)
                      QuietNotice(
                        _controller.errorMessage!,
                        onRetry: _controller.initialize,
                        onDismiss: _controller.dismissError,
                      ),
                    Expanded(
                      child: Row(
                        crossAxisAlignment: CrossAxisAlignment.stretch,
                        children: <Widget>[
                          if (!compact) ...<Widget>[
                            SizedBox(
                              key: const ValueKey<String>('chat-sidebar'),
                              width: sidebarWidth,
                              child: _sidebar(context),
                            ),
                            SidebarResizeHandle(
                              key: const ValueKey<String>(
                                'sidebar-resize-handle',
                              ),
                              width: sidebarWidth,
                              maximum: maximumSidebarWidth,
                              onChange: _resizeSidebar,
                              onCommit: _saveSidebarWidth,
                              onReset: _resetSidebarWidth,
                            ),
                          ],
                          Expanded(
                            child: LayoutBuilder(
                              builder:
                                  (BuildContext context, BoxConstraints pane) {
                                    final bool split =
                                        pane.maxHeight >= 1000 &&
                                        pane.maxWidth >= 900 &&
                                        MediaQuery.textScalerOf(
                                              context,
                                            ).scale(14) <=
                                            19;
                                    return Column(
                                      children: <Widget>[
                                        Expanded(
                                          child: Stack(
                                            children: <Widget>[
                                              Positioned.fill(
                                                child: Padding(
                                                  padding: EdgeInsets.only(
                                                    bottom: _queueOpen && split
                                                        ? 400
                                                        : 0,
                                                  ),
                                                  child: Offstage(
                                                    offstage:
                                                        _queueOpen && !split,
                                                    child: ExcludeFocus(
                                                      excluding:
                                                          _queueOpen && !split,
                                                      child: LibraryView(
                                                        controller: _controller,
                                                        searchFocus:
                                                            _fileSearch,
                                                        destinationError:
                                                            _destinationError,
                                                        onChooseFolder:
                                                            _chooseFolder,
                                                        onDownload:
                                                            _downloadSelected,
                                                        onEntireScope:
                                                            _downloadEntireScope,
                                                      ),
                                                    ),
                                                  ),
                                                ),
                                              ),
                                              Positioned(
                                                left: 0,
                                                right: 0,
                                                bottom: 0,
                                                top: split ? null : 0,
                                                height: split ? 400 : null,
                                                child: Offstage(
                                                  offstage: !_queueOpen,
                                                  child: ExcludeFocus(
                                                    excluding: !_queueOpen,
                                                    child: DownloadsView(
                                                      controller: _controller,
                                                      compact: split,
                                                    ),
                                                  ),
                                                ),
                                              ),
                                            ],
                                          ),
                                        ),
                                        TransferStrip(
                                          controller: _controller,
                                          isOpen: _queueOpen,
                                          isSplit: split,
                                          onToggle: () => setState(
                                            () => _queueOpen = !_queueOpen,
                                          ),
                                        ),
                                      ],
                                    );
                                  },
                            ),
                          ),
                        ],
                      ),
                    ),
                  ],
                ),
              ),
            );
          },
        ),
      ),
    ),
  );

  Widget _header(BuildContext context, bool compact) => Container(
    decoration: BoxDecoration(
      color: context.quiet.canvas,
      border: Border(bottom: BorderSide(color: context.quiet.divider)),
    ),
    padding: EdgeInsets.fromLTRB(compact ? 8 : 24, 12, 16, 12),
    child: Row(
      children: <Widget>[
        if (compact)
          IconButton(
            tooltip: 'Browse chats',
            onPressed: () => _scaffold.currentState?.openDrawer(),
            icon: const Icon(Icons.menu, size: 21),
          ),
        ExcludeSemantics(
          child: Image.asset(
            'assets/branding/logo.png',
            width: 32,
            height: 32,
            fit: BoxFit.contain,
            filterQuality: FilterQuality.high,
          ),
        ),
        const SizedBox(width: 10),
        Expanded(
          child: Text(
            'Telegram Media Downloader',
            maxLines: 1,
            overflow: TextOverflow.ellipsis,
            style: Theme.of(
              context,
            ).textTheme.titleMedium!.copyWith(fontWeight: FontWeight.w600),
          ),
        ),
        if (!compact) ...<Widget>[
          const SizedBox(width: 12),
          Icon(
            _controller.connected
                ? Icons.check_circle_outline
                : Icons.circle_outlined,
            size: 14,
            color: _controller.connected
                ? context.quiet.accent
                : context.quiet.muted,
          ),
          const SizedBox(width: 6),
          Text(
            _controller.isConnecting
                ? 'Connecting…'
                : _controller.connected
                ? 'Connected'
                : 'Not connected',
            style: Theme.of(context).textTheme.bodySmall,
          ),
          const SizedBox(width: 10),
        ],
        IconButton(
          tooltip: _controller.connected
              ? 'Refresh chats'
              : 'Connect to Telegram',
          onPressed: _controller.isConnecting ? null : _controller.initialize,
          icon: _controller.isConnecting
              ? const SizedBox.square(
                  dimension: 17,
                  child: QuietSpinner(strokeWidth: 1.5),
                )
              : Icon(_controller.connected ? Icons.sync : Icons.link, size: 21),
        ),
      ],
    ),
  );

  Widget _sidebar(BuildContext context) => Container(
    decoration: BoxDecoration(color: context.quiet.sidebar),
    child: Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: <Widget>[
        Padding(
          padding: const EdgeInsets.fromLTRB(20, 24, 20, 18),
          child: Row(
            children: <Widget>[
              Expanded(
                child: Text(
                  'Chats',
                  style: Theme.of(context).textTheme.titleLarge,
                ),
              ),
              Text('${_controller.chats.length}', style: numberStyle(context)),
            ],
          ),
        ),
        Padding(
          padding: const EdgeInsets.symmetric(horizontal: 12),
          child: TextField(
            focusNode: _chatSearch,
            onChanged: _controller.setSearchQuery,
            decoration: const InputDecoration(
              labelText: 'Search chats or topics',
              prefixIcon: Icon(Icons.search, size: 18),
            ),
          ),
        ),
        Padding(
          padding: const EdgeInsets.fromLTRB(10, 8, 10, 8),
          child: PopupMenuButton<String>(
            tooltip: 'Filter conversations',
            initialValue: _controller.chatTypeFilter,
            onSelected: _controller.setChatTypeFilter,
            itemBuilder: (_) => <PopupMenuEntry<String>>[
              for (final entry in const <String, String>{
                'all': 'All conversations',
                'group': 'Groups',
                'channel': 'Channels',
                'private': 'Private chats',
              }.entries)
                CheckedPopupMenuItem<String>(
                  value: entry.key,
                  checked: _controller.chatTypeFilter == entry.key,
                  child: Text(entry.value),
                ),
            ],
            child: Padding(
              padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 10),
              child: Row(
                children: <Widget>[
                  Expanded(
                    child: Text(
                      const <String, String>{
                        'all': 'All conversations',
                        'group': 'Groups',
                        'channel': 'Channels',
                        'private': 'Private chats',
                      }[_controller.chatTypeFilter]!,
                      style: Theme.of(context).textTheme.bodySmall,
                    ),
                  ),
                  const Icon(Icons.keyboard_arrow_down, size: 17),
                ],
              ),
            ),
          ),
        ),
        Expanded(
          child: _controller.isConnecting && _controller.chats.isEmpty
              ? Semantics(
                  label: 'Loading your chats',
                  liveRegion: true,
                  child: ExcludeSemantics(
                    child: ListView.builder(
                      itemCount: 5,
                      itemBuilder: (_, int i) => Padding(
                        padding: const EdgeInsets.symmetric(
                          horizontal: 22,
                          vertical: 17,
                        ),
                        child: Container(
                          height: 12,
                          color: context.quiet.divider,
                        ),
                      ),
                    ),
                  ),
                )
              : _controller.filteredChats.isEmpty
              ? QuietEmptyState(
                  icon: Icons.forum_outlined,
                  title: _controller.connected
                      ? 'No matching chats'
                      : 'Your chats live here',
                  message: _controller.connected
                      ? 'Try another search or conversation type.'
                      : 'Connect above to load your library.',
                )
              : ListView.builder(
                  key: const PageStorageKey<String>('chat-list'),
                  padding: const EdgeInsets.only(bottom: 14),
                  itemCount: _controller.filteredChats.length,
                  itemBuilder: (BuildContext context, int index) =>
                      _chat(context, _controller.filteredChats[index]),
                ),
        ),
        Padding(
          padding: const EdgeInsets.all(20),
          child: Row(
            children: <Widget>[
              const ExcludeSemantics(
                child: Icon(Icons.computer_outlined, size: 17),
              ),
              const SizedBox(width: 8),
              Expanded(
                child: Text(
                  'On this device',
                  style: Theme.of(context).textTheme.bodySmall,
                ),
              ),
            ],
          ),
        ),
      ],
    ),
  );

  Widget _chat(BuildContext context, TelegramChat chat) {
    final bool active = _controller.activeChat?.id == chat.id;
    final bool expanded = _expanded.contains(chat.id);
    final bool selected = active && _controller.activeTopic == null;
    return Padding(
      padding: const EdgeInsets.fromLTRB(8, 2, 8, 3),
      child: Column(
        children: <Widget>[
          Material(
            color: selected ? context.quiet.selected : Colors.transparent,
            borderRadius: BorderRadius.circular(8),
            child: InkWell(
              borderRadius: BorderRadius.circular(8),
              hoverColor: context.quiet.hover,
              onTap: () => _openChat(chat),
              child: Padding(
                padding: const EdgeInsets.fromLTRB(10, 12, 2, 12),
                child: Row(
                  children: <Widget>[
                    ExcludeSemantics(
                      child: Icon(
                        chat.topics.isNotEmpty || chat.type.contains('group')
                            ? Icons.people_outline
                            : chat.type.contains('channel')
                            ? Icons.campaign_outlined
                            : Icons.person_outline,
                        size: 20,
                        color: selected
                            ? context.quiet.accent
                            : context.quiet.muted,
                      ),
                    ),
                    const SizedBox(width: 10),
                    Expanded(
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: <Widget>[
                          Tooltip(
                            message: chat.name,
                            child: Text(
                              chat.name,
                              maxLines: 2,
                              overflow: TextOverflow.ellipsis,
                              style: Theme.of(context).textTheme.bodyMedium!
                                  .copyWith(
                                    fontWeight: active
                                        ? FontWeight.w500
                                        : FontWeight.w400,
                                    color: selected
                                        ? context.quiet.accent
                                        : context.quiet.text,
                                  ),
                            ),
                          ),
                          const SizedBox(height: 3),
                          Text(
                            chat.topics.isEmpty
                                ? chat.displayType
                                : '${chat.topics.length} topics',
                            style: Theme.of(context).textTheme.bodySmall,
                          ),
                        ],
                      ),
                    ),
                    if (chat.topics.isNotEmpty)
                      Semantics(
                        expanded: expanded,
                        child: IconButton(
                          tooltip: expanded
                              ? 'Collapse topics'
                              : 'Expand topics',
                          onPressed: () => setState(() {
                            if (expanded) {
                              _expanded.remove(chat.id);
                            } else {
                              _expanded.add(chat.id);
                            }
                          }),
                          icon: Icon(
                            expanded ? Icons.expand_more : Icons.chevron_right,
                            size: 18,
                          ),
                        ),
                      ),
                  ],
                ),
              ),
            ),
          ),
          if (chat.topics.isNotEmpty && expanded) ...<Widget>[
            _topic(context, chat, null),
            for (final TelegramTopic topic in chat.topics)
              _topic(context, chat, topic),
          ],
        ],
      ),
    );
  }

  Widget _topic(BuildContext context, TelegramChat chat, TelegramTopic? topic) {
    final bool selected =
        _controller.activeChat?.id == chat.id &&
        _controller.activeTopic?.id == topic?.id;
    return Padding(
      padding: const EdgeInsets.fromLTRB(30, 2, 6, 2),
      child: Semantics(
        selected: selected,
        child: Material(
          color: selected ? context.quiet.selected : Colors.transparent,
          borderRadius: BorderRadius.circular(7),
          child: InkWell(
            borderRadius: BorderRadius.circular(7),
            hoverColor: context.quiet.hover,
            onTap: () => _openChat(chat, topic: topic),
            child: Padding(
              padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 11),
              child: Row(
                children: <Widget>[
                  ExcludeSemantics(
                    child: Icon(
                      topic == null ? Icons.grid_view_outlined : Icons.tag,
                      size: 17,
                      color: selected
                          ? context.quiet.accent
                          : context.quiet.muted,
                    ),
                  ),
                  const SizedBox(width: 9),
                  Expanded(
                    child: Tooltip(
                      message: topic?.title ?? 'All topics',
                      child: Text(
                        topic?.title ?? 'All topics',
                        maxLines: 2,
                        overflow: TextOverflow.ellipsis,
                        style: Theme.of(context).textTheme.bodyMedium!.copyWith(
                          color: selected
                              ? context.quiet.accent
                              : context.quiet.muted,
                          fontWeight: selected
                              ? FontWeight.w500
                              : FontWeight.w400,
                        ),
                      ),
                    ),
                  ),
                ],
              ),
            ),
          ),
        ),
      ),
    );
  }

  void _openChat(TelegramChat chat, {TelegramTopic? topic}) {
    setState(() {
      _queueOpen = false;
      _expanded.add(chat.id);
    });
    _scaffold.currentState?.closeDrawer();
    if (_controller.activeChat?.id == chat.id &&
        _controller.activeTopic?.id == topic?.id) {
      return;
    }
    unawaited(_controller.openChat(chat, topic: topic));
  }

  Future<void> _chooseFolder() async {
    try {
      final String? path = await getDirectoryPath(
        initialDirectory: _controller.destinationRoot.isEmpty
            ? null
            : _controller.destinationRoot,
        confirmButtonText: 'Save here',
      );
      if (!mounted || path == null) return;
      _controller.setDestination(path);
      setState(() => _destinationError = null);
    } catch (error) {
      if (mounted) {
        setState(
          () => _destinationError = 'Could not open the folder picker. $error',
        );
      }
    }
  }

  Future<bool> _checkDestination() async {
    final bool exists =
        _controller.destinationRoot.isNotEmpty &&
        await Directory(_controller.destinationRoot).exists();
    if (mounted) {
      setState(
        () => _destinationError = exists
            ? null
            : 'Choose an available save folder. Reconnect your drive if needed.',
      );
    }
    return exists && mounted;
  }

  Future<void> _downloadSelected() async {
    if (!await _checkDestination() || !mounted) return;
    final int count = _controller.selectedMediaCount;
    await _controller.startMediaDownload();
    if (mounted && _controller.selectedMediaCount < count) {
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(
          duration: const Duration(seconds: 3),
          content: Text(
            '$count ${count == 1 ? 'file' : 'files'} added to the download queue.',
          ),
        ),
      );
    }
  }

  Future<void> _downloadEntireScope() async {
    if (!await _checkDestination() || !mounted) return;
    final String types = _controller.fileTypes.length == fileTypeLabels.length
        ? 'All file types'
        : _controller.fileTypes
              .map((String key) => fileTypeLabels[key])
              .join(', ');
    final bool? confirmed = await showDialog<bool>(
      context: context,
      builder: (BuildContext dialogContext) => AlertDialog(
        title: Text(
          _controller.activeTopic == null
              ? 'Download entire chat?'
              : 'Download entire topic?',
        ),
        content: ConstrainedBox(
          constraints: const BoxConstraints(maxWidth: 420),
          child: SingleChildScrollView(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              mainAxisSize: MainAxisSize.min,
              children: <Widget>[
                Text(
                  '${_controller.activeChat!.name} / ${_controller.scopeName}',
                  style: Theme.of(dialogContext).textTheme.titleMedium,
                ),
                const SizedBox(height: 16),
                Text(types),
                const SizedBox(height: 12),
                Text(
                  'Includes older files beyond the loaded results. Files already saved at this destination will be skipped. Search and status, size, and date filters apply to browsing only. This action uses the file types above.',
                  style: Theme.of(dialogContext).textTheme.bodySmall,
                ),
                const SizedBox(height: 16),
                SelectableText(
                  _controller.destinationRoot,
                  style: Theme.of(dialogContext).textTheme.bodySmall,
                ),
              ],
            ),
          ),
        ),
        actions: <Widget>[
          TextButton(
            onPressed: () => Navigator.pop(dialogContext, false),
            child: const Text('Back'),
          ),
          FilledButton(
            onPressed: () => Navigator.pop(dialogContext, true),
            child: const Text('Queue matching files'),
          ),
        ],
      ),
    );
    if (confirmed == true) {
      await _controller.startMediaDownload(entireScope: true);
      if (mounted) setState(() => _queueOpen = true);
    }
  }
}

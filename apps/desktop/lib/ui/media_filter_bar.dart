import 'dart:math' as math;
import 'package:flutter/material.dart';
import '../models/media_filters.dart';
import '../models/media_item.dart';
import 'quiet_theme.dart';

class MediaFilterBar extends StatelessWidget {
  const MediaFilterBar({
    super.key,
    required this.filters,
    required this.entries,
    required this.scope,
    this.compact = false,
  });
  final MediaFilters filters;
  final List<MediaEntry> entries;
  final String scope;
  final bool compact;
  @override
  Widget build(BuildContext context) => LayoutBuilder(
    builder: (context, box) => _buildBar(
      context,
      compact ||
          box.maxWidth < 850 ||
          MediaQuery.textScalerOf(context).scale(14) > 19,
    ),
  );
  Widget _buildBar(BuildContext context, bool dense) {
    final matched = filters.apply(entries).length;
    final chips = <Widget>[];
    void chip(String text, VoidCallback clear) {
      chips.add(
        InputChip(
          label: Text(text),
          onDeleted: () => filters.change(clear),
          visualDensity: VisualDensity.compact,
        ),
      );
    }

    if (filters.status != 'all' &&
        !const {
          'in_progress',
          'completed',
          'attention',
        }.contains(filters.status)) {
      chip(
        statusFilterLabels[filters.status] ?? 'Unknown',
        () => filters.status = 'all',
      );
    }
    if (filters.size != 'any') {
      chip(sizeFilterLabels[filters.size]!, () => filters.size = 'any');
    }
    if (filters.date != 'any') {
      chip(
        filters.date == 'custom'
            ? '${_day(filters.from)} – ${_day(filters.to)}'
            : dateFilterLabels[filters.date]!,
        () {
          filters.date = 'any';
          filters.from = null;
          filters.to = null;
        },
      );
    }
    if (filters.chatIds.isNotEmpty) {
      chip(
        '${filters.chatIds.length} chat${filters.chatIds.length == 1 ? '' : 's'}',
        () {
          filters.chatIds.clear();
          filters.topicKeys.clear();
        },
      );
    }
    if (filters.topicKeys.isNotEmpty) {
      chip(
        '${filters.topicKeys.length} topic${filters.topicKeys.length == 1 ? '' : 's'}',
        () => filters.topicKeys.clear(),
      );
    }
    if (scope == 'downloads' && filters.types.length != fileTypeLabels.length) {
      chip(
        '${filters.types.length} file types',
        () => filters.types = fileTypeLabels.keys.toSet(),
      );
    }
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Wrap(
          spacing: 6,
          runSpacing: 6,
          crossAxisAlignment: WrapCrossAlignment.center,
          children: [
            if (dense)
              PopupMenuButton<String>(
                tooltip: 'Download status',
                onSelected: (v) => filters.change(() => filters.status = v),
                itemBuilder: (_) => [
                  for (final e in statusFilterLabels.entries)
                    CheckedPopupMenuItem(
                      value: e.key,
                      checked: filters.status == e.key,
                      child: Text(e.value),
                    ),
                ],
                child: Container(
                  width: 152,
                  height: 44,
                  padding: const EdgeInsets.symmetric(horizontal: 10),
                  decoration: BoxDecoration(
                    border: Border.all(color: context.quiet.outline),
                    borderRadius: BorderRadius.circular(8),
                  ),
                  child: Row(
                    children: [
                      Expanded(
                        child: Text(
                          statusFilterLabels[filters.status] ?? 'All',
                          overflow: TextOverflow.ellipsis,
                        ),
                      ),
                      const Icon(Icons.expand_more, size: 17),
                    ],
                  ),
                ),
              ),
            if (!dense)
              for (final option in const {
                'all': 'All',
                'in_progress': 'In progress',
                'completed': 'Downloaded',
                'attention': 'Needs attention',
              }.entries)
                ChoiceChip(
                  key: ValueKey<String>('$scope-status-${option.key}'),
                  label: Text(option.value),
                  selected: filters.status == option.key,
                  onSelected: (_) =>
                      filters.change(() => filters.status = option.key),
                  showCheckmark: false,
                  visualDensity: VisualDensity.compact,
                ),
            OutlinedButton.icon(
              key: ValueKey<String>('$scope-filters'),
              onPressed: () => _open(context),
              icon: const Icon(Icons.tune, size: 17),
              label: const Text('Filters'),
            ),
            PopupMenuButton<String>(
              tooltip: 'Group by: ${groupLabels[filters.groupBy]}',
              icon: const Icon(Icons.workspaces_outline, size: 19),
              onSelected: (v) => filters.change(() => filters.groupBy = v),
              itemBuilder: (_) => [
                for (final e in groupLabels.entries)
                  CheckedPopupMenuItem(
                    value: e.key,
                    checked: filters.groupBy == e.key,
                    child: Text(e.value),
                  ),
              ],
            ),
            PopupMenuButton<String>(
              tooltip: 'Sort: ${sortLabels[filters.sort]}',
              icon: const Icon(Icons.sort, size: 20),
              onSelected: (v) => filters.change(() => filters.sort = v),
              itemBuilder: (_) => [
                for (final e in sortLabels.entries)
                  CheckedPopupMenuItem(
                    value: e.key,
                    checked: filters.sort == e.key,
                    child: Text(e.value),
                  ),
              ],
            ),
            if (!dense)
              Text(
                '$matched of ${entries.length} files',
                style: Theme.of(context).textTheme.bodySmall,
              ),
          ],
        ),
        if (chips.isNotEmpty || filters.narrowed)
          Padding(
            padding: const EdgeInsets.only(top: 6),
            child: Wrap(
              spacing: 6,
              runSpacing: 6,
              crossAxisAlignment: WrapCrossAlignment.center,
              children: [
                ...chips,
                TextButton(
                  onPressed: filters.reset,
                  child: const Text('Clear all'),
                ),
              ],
            ),
          ),
      ],
    );
  }

  Future<void> _open(BuildContext context) async {
    final result = await showDialog<MediaFilters>(
      context: context,
      builder: (_) =>
          _FilterDialog(filters: filters, entries: entries, scope: scope),
    );
    if (result == null) return;
    filters.change(() {
      filters.status = result.status;
      filters.types = result.types.toSet();
      filters.chatIds = result.chatIds.toSet();
      filters.topicKeys = result.topicKeys.toSet();
      filters.size = result.size;
      filters.date = result.date;
      filters.from = result.from;
      filters.to = result.to;
      filters.query = result.query;
    });
    result.dispose();
  }

  static String _day(DateTime? date) =>
      date == null ? 'Choose dates' : '${date.day}/${date.month}/${date.year}';
}

class _FilterDialog extends StatefulWidget {
  const _FilterDialog({
    required this.filters,
    required this.entries,
    required this.scope,
  });
  final MediaFilters filters;
  final List<MediaEntry> entries;
  final String scope;
  @override
  State<_FilterDialog> createState() => _FilterDialogState();
}

class _FilterDialogState extends State<_FilterDialog> {
  late final MediaFilters draft = MediaFilters()
    ..status = widget.filters.status
    ..types = widget.filters.types.toSet()
    ..chatIds = widget.filters.chatIds.toSet()
    ..topicKeys = widget.filters.topicKeys.toSet()
    ..size = widget.filters.size
    ..date = widget.filters.date
    ..from = widget.filters.from
    ..to = widget.filters.to
    ..query = widget.filters.query;
  String scopeSearch = '';
  int _dateRevision = 0;
  @override
  void dispose() {
    draft.dispose();
    super.dispose();
  }

  Widget _heading(String text) => Padding(
    padding: const EdgeInsets.only(top: 20, bottom: 8),
    child: Text(text, style: Theme.of(context).textTheme.titleSmall),
  );
  Widget _dropdown(
    String label,
    String value,
    Map<String, String> options,
    ValueChanged<String> update,
  ) => DropdownButtonFormField<String>(
    key: ValueKey('$label:$value:$_dateRevision'),
    initialValue: value,
    isExpanded: true,
    decoration: InputDecoration(labelText: label),
    items: [
      for (final e in options.entries)
        DropdownMenuItem(value: e.key, child: Text(e.value)),
    ],
    onChanged: (v) {
      if (v != null) update(v);
    },
  );
  Future<void> _chooseDates() async {
    final now = DateTime.now();
    final range = await showDateRangePicker(
      context: context,
      firstDate: DateTime(1970),
      lastDate: DateTime(now.year + 1, 12, 31),
      initialDateRange: draft.from != null && draft.to != null
          ? DateTimeRange(start: draft.from!, end: draft.to!)
          : null,
      helpText: 'Filter by posted date',
    );
    if (!mounted) return;
    if (range == null) {
      setState(() => _dateRevision++);
      return;
    }
    setState(() {
      draft.date = 'custom';
      draft.from = range.start;
      draft.to = range.end;
    });
  }

  @override
  Widget build(BuildContext context) {
    final chats = <int, String>{
      for (final e in widget.entries) e.item.chatId: e.chatName,
    };
    final visibleChats =
        chats.entries
            .where((e) => e.value.toLowerCase().contains(scopeSearch))
            .toList()
          ..sort(
            (a, b) => a.value.toLowerCase().compareTo(b.value.toLowerCase()),
          );
    final topics = <String, String>{
      for (final e in widget.entries)
        if (draft.chatIds.isEmpty || draft.chatIds.contains(e.item.chatId))
          e.topicKey: '${e.chatName} · ${e.topicName}',
    };
    final visibleTopics = topics.entries
        .where((e) => e.value.toLowerCase().contains(scopeSearch))
        .toList();
    return Dialog(
      child: ConstrainedBox(
        constraints: BoxConstraints(
          maxWidth: 520,
          maxHeight: MediaQuery.sizeOf(context).height * .88,
        ),
        child: Padding(
          padding: const EdgeInsets.all(24),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              Row(
                children: [
                  Expanded(
                    child: Text(
                      'Filter ${widget.scope}',
                      style: Theme.of(context).textTheme.headlineSmall,
                    ),
                  ),
                  IconButton(
                    tooltip: 'Close filters',
                    onPressed: () => Navigator.pop(context),
                    icon: const Icon(Icons.close),
                  ),
                ],
              ),
              const SizedBox(height: 14),
              Flexible(
                child: SingleChildScrollView(
                  padding: const EdgeInsets.only(top: 8),
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.stretch,
                    children: [
                      _dropdown(
                        'Download status',
                        draft.status,
                        statusFilterLabels,
                        (v) => setState(() => draft.status = v),
                      ),
                      _heading('File types'),
                      Wrap(
                        spacing: 6,
                        runSpacing: 6,
                        children: [
                          FilterChip(
                            label: const Text('All files'),
                            selected:
                                draft.types.length == fileTypeLabels.length,
                            onSelected: (selected) => setState(
                              () => draft.types = selected
                                  ? fileTypeLabels.keys.toSet()
                                  : {},
                            ),
                          ),
                          for (final e in fileTypeLabels.entries)
                            FilterChip(
                              label: Text(e.value),
                              selected: draft.types.contains(e.key),
                              onSelected: (selected) => setState(() {
                                selected
                                    ? draft.types.add(e.key)
                                    : draft.types.remove(e.key);
                              }),
                            ),
                        ],
                      ),
                      if (draft.types.isEmpty)
                        Padding(
                          padding: const EdgeInsets.only(top: 8),
                          child: Text(
                            'Choose at least one type to show files.',
                            style: TextStyle(color: context.quiet.muted),
                          ),
                        ),
                      _heading('Size & posted date'),
                      _dropdown(
                        'File size',
                        draft.size,
                        sizeFilterLabels,
                        (v) => setState(() => draft.size = v),
                      ),
                      const SizedBox(height: 12),
                      _dropdown('Posted date', draft.date, dateFilterLabels, (
                        v,
                      ) {
                        if (v == 'custom') {
                          _chooseDates();
                        } else {
                          setState(() => draft.date = v);
                        }
                      }),
                      if (draft.date == 'custom')
                        TextButton.icon(
                          onPressed: _chooseDates,
                          icon: const Icon(Icons.date_range, size: 17),
                          label: Text(
                            '${MediaFilterBar._day(draft.from)} – ${MediaFilterBar._day(draft.to)}',
                          ),
                        ),
                      if (widget.scope == 'downloads') ...[
                        _heading('Chats & topics'),
                        TextField(
                          decoration: const InputDecoration(
                            labelText: 'Find a chat or topic',
                            prefixIcon: Icon(Icons.search),
                          ),
                          onChanged: (v) => setState(
                            () => scopeSearch = v.trim().toLowerCase(),
                          ),
                        ),
                        CheckboxListTile(
                          contentPadding: EdgeInsets.zero,
                          title: const Text('All chats'),
                          value: draft.chatIds.isEmpty,
                          onChanged: (_) => setState(() {
                            draft.chatIds.clear();
                            draft.topicKeys.clear();
                          }),
                        ),
                        SizedBox(
                          height: math.min(
                            180,
                            visibleChats.length *
                                52.0 *
                                MediaQuery.textScalerOf(context).scale(1),
                          ),
                          child: ListView.builder(
                            itemCount: visibleChats.length,
                            itemBuilder: (_, i) {
                              final e = visibleChats[i];
                              return CheckboxListTile(
                                contentPadding: EdgeInsets.zero,
                                title: Text(e.value),
                                subtitle: Text('Chat ${e.key}'),
                                value: draft.chatIds.contains(e.key),
                                onChanged: (selected) => setState(() {
                                  selected == true
                                      ? draft.chatIds.add(e.key)
                                      : draft.chatIds.remove(e.key);
                                  draft.topicKeys.removeWhere(
                                    (key) =>
                                        draft.chatIds.isNotEmpty &&
                                        !draft.chatIds.contains(
                                          int.tryParse(key.split(':').first),
                                        ),
                                  );
                                }),
                              );
                            },
                          ),
                        ),
                        CheckboxListTile(
                          contentPadding: EdgeInsets.zero,
                          title: const Text('All topics'),
                          value: draft.topicKeys.isEmpty,
                          onChanged: (_) =>
                              setState(() => draft.topicKeys.clear()),
                        ),
                        SizedBox(
                          height: math.min(
                            180,
                            visibleTopics.length *
                                58.0 *
                                MediaQuery.textScalerOf(context).scale(1),
                          ),
                          child: ListView.builder(
                            itemCount: visibleTopics.length,
                            itemBuilder: (_, i) {
                              final e = visibleTopics[i];
                              return CheckboxListTile(
                                contentPadding: EdgeInsets.zero,
                                title: Text(e.value),
                                value: draft.topicKeys.contains(e.key),
                                onChanged: (selected) => setState(() {
                                  selected == true
                                      ? draft.topicKeys.add(e.key)
                                      : draft.topicKeys.remove(e.key);
                                }),
                              );
                            },
                          ),
                        ),
                      ],
                      const SizedBox(height: 14),
                      Text(
                        widget.scope == 'library'
                            ? 'Applies to loaded files in this chat or topic. Load older files to extend the results.'
                            : 'Applies to queued files and this app’s download history. Never-queued files are in the library.',
                        style: Theme.of(context).textTheme.bodySmall,
                      ),
                    ],
                  ),
                ),
              ),
              const SizedBox(height: 16),
              Wrap(
                alignment: WrapAlignment.spaceBetween,
                crossAxisAlignment: WrapCrossAlignment.center,
                spacing: 12,
                runSpacing: 8,
                children: [
                  TextButton(
                    onPressed: () => setState(draft.reset),
                    child: const Text('Reset filters'),
                  ),
                  FilledButton(
                    onPressed: () {
                      final result = MediaFilters()
                        ..status = draft.status
                        ..types = draft.types.toSet()
                        ..chatIds = draft.chatIds.toSet()
                        ..topicKeys = draft.topicKeys.toSet()
                        ..size = draft.size
                        ..date = draft.date
                        ..from = draft.from
                        ..to = draft.to
                        ..query = draft.query;
                      Navigator.pop(context, result);
                    },
                    child: Text(
                      'Show ${draft.apply(widget.entries).length} ${draft.apply(widget.entries).length == 1 ? 'file' : 'files'}',
                    ),
                  ),
                ],
              ),
            ],
          ),
        ),
      ),
    );
  }
}

class MediaGroupHeading extends StatelessWidget {
  const MediaGroupHeading(this.group, {super.key});
  final MediaGroup group;
  @override
  Widget build(BuildContext context) => Padding(
    padding: const EdgeInsets.fromLTRB(4, 20, 4, 8),
    child: Row(
      children: [
        Expanded(
          child: Text(
            group.label,
            style: Theme.of(context).textTheme.titleSmall,
          ),
        ),
        const SizedBox(width: 12),
        Text(
          '${group.entries.length}',
          style: Theme.of(context).textTheme.bodySmall,
        ),
      ],
    ),
  );
}

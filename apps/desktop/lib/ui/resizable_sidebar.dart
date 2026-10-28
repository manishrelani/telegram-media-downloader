import 'dart:math' as math;

import 'package:flutter/gestures.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';

import 'quiet_theme.dart';

const double sidebarMinWidth = 220;
const double sidebarMaxWidth = 420;
const double sidebarDefaultWidth = 248;
const double sidebarDividerWidth = 8;

// Keep room for file names and actions even at the smallest desktop width.
double sidebarMaximumFor(double windowWidth) => math.max(
  sidebarMinWidth,
  math.min(sidebarMaxWidth, windowWidth - sidebarDividerWidth - 520),
);

class SidebarResizeHandle extends StatefulWidget {
  const SidebarResizeHandle({
    super.key,
    required this.width,
    required this.maximum,
    required this.onChange,
    required this.onCommit,
    required this.onReset,
  });

  final double width;
  final double maximum;
  final ValueChanged<double> onChange;
  final VoidCallback onCommit;
  final VoidCallback onReset;

  @override
  State<SidebarResizeHandle> createState() => _SidebarResizeHandleState();
}

class _SidebarResizeHandleState extends State<SidebarResizeHandle> {
  final FocusNode _focus = FocusNode(debugLabel: 'Resize chat sidebar');
  bool _hover = false;
  bool _focused = false;
  bool _dragging = false;
  double _dragWidth = sidebarDefaultWidth;

  @override
  void dispose() {
    _focus.dispose();
    super.dispose();
  }

  void _adjust(double width) {
    widget.onChange(width.clamp(sidebarMinWidth, widget.maximum));
    widget.onCommit();
  }

  void _endDrag() {
    if (!_dragging) return;
    setState(() => _dragging = false);
    widget.onCommit();
  }

  Future<void> _showWidthMenu(TapUpDetails details) async {
    _focus.requestFocus();
    final overlay =
        Overlay.of(context).context.findRenderObject()! as RenderBox;
    final position = overlay.globalToLocal(details.globalPosition);
    final choice = await showMenu<int>(
      context: context,
      position: RelativeRect.fromLTRB(
        position.dx,
        position.dy,
        overlay.size.width - position.dx,
        overlay.size.height - position.dy,
      ),
      items: <PopupMenuEntry<int>>[
        PopupMenuItem(
          value: -1,
          enabled: widget.width > sidebarMinWidth,
          child: const Text('Make sidebar narrower'),
        ),
        PopupMenuItem(
          value: 1,
          enabled: widget.width < widget.maximum,
          child: const Text('Make sidebar wider'),
        ),
        const PopupMenuDivider(),
        const PopupMenuItem(value: 0, child: Text('Reset sidebar width')),
      ],
    );
    if (!mounted || choice == null) return;
    if (choice == 0) {
      widget.onReset();
    } else {
      _adjust(widget.width + choice * 16);
    }
  }

  @override
  Widget build(BuildContext context) {
    final active = _hover || _focused || _dragging;
    return Semantics(
      label: 'Chat sidebar width',
      value: '${widget.width.round()} pixels',
      increasedValue:
          '${(widget.width + 16).clamp(sidebarMinWidth, widget.maximum).round()} pixels',
      decreasedValue:
          '${(widget.width - 16).clamp(sidebarMinWidth, widget.maximum).round()} pixels',
      hint:
          'Drag to resize. Use left and right arrow keys when focused. Double-click or press Enter to reset.',
      onIncrease: () => _adjust(widget.width + 16),
      onDecrease: () => _adjust(widget.width - 16),
      child: CallbackShortcuts(
        bindings: <ShortcutActivator, VoidCallback>{
          const SingleActivator(LogicalKeyboardKey.arrowLeft): () =>
              _adjust(widget.width - 16),
          const SingleActivator(LogicalKeyboardKey.arrowRight): () =>
              _adjust(widget.width + 16),
          const SingleActivator(LogicalKeyboardKey.home): () =>
              _adjust(sidebarMinWidth),
          const SingleActivator(LogicalKeyboardKey.end): () =>
              _adjust(widget.maximum),
          const SingleActivator(LogicalKeyboardKey.enter): widget.onReset,
        },
        child: Focus(
          focusNode: _focus,
          onFocusChange: (focused) => setState(() => _focused = focused),
          child: MouseRegion(
            cursor: SystemMouseCursors.resizeColumn,
            onEnter: (_) => setState(() => _hover = true),
            onExit: (_) => setState(() => _hover = false),
            child: Tooltip(
              message: 'Drag to resize · Double-click to reset',
              child: GestureDetector(
                behavior: HitTestBehavior.opaque,
                dragStartBehavior: DragStartBehavior.down,
                onTap: _focus.requestFocus,
                onSecondaryTapUp: _showWidthMenu,
                onDoubleTap: () {
                  _focus.requestFocus();
                  widget.onReset();
                },
                onHorizontalDragStart: (_) {
                  _focus.requestFocus();
                  _dragWidth = widget.width;
                  setState(() => _dragging = true);
                  widget.onChange(_dragWidth);
                },
                onHorizontalDragUpdate: (details) {
                  _dragWidth = (_dragWidth + details.delta.dx).clamp(
                    sidebarMinWidth,
                    widget.maximum,
                  );
                  widget.onChange(_dragWidth);
                },
                onHorizontalDragEnd: (_) => _endDrag(),
                onHorizontalDragCancel: _endDrag,
                child: SizedBox(
                  width: sidebarDividerWidth,
                  child: ColoredBox(
                    color: context.quiet.sidebar,
                    child: Center(
                      child: Container(
                        width: active ? 2 : 1,
                        color: active
                            ? context.quiet.accent
                            : context.quiet.divider,
                      ),
                    ),
                  ),
                ),
              ),
            ),
          ),
        ),
      ),
    );
  }
}

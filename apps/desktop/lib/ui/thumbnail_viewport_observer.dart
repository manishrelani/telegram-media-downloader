import 'package:flutter/rendering.dart';
import 'package:flutter/widgets.dart';

class ThumbnailVisibility {
  const ThumbnailVisibility(this.visible, this.position);
  final bool visible;
  final double position;
}

/// Reports painted viewport intersection, rather than sliver cache membership.
class ThumbnailViewportObserver extends StatefulWidget {
  const ThumbnailViewportObserver({
    super.key,
    required this.onVisibilityChanged,
    required this.child,
  });
  final ValueChanged<ThumbnailVisibility> onVisibilityChanged;
  final Widget child;
  @override
  State<ThumbnailViewportObserver> createState() =>
      _ThumbnailViewportObserverState();
}

class _ThumbnailViewportObserverState extends State<ThumbnailViewportObserver>
    with WidgetsBindingObserver {
  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addObserver(this);
  }

  @override
  void didChangeMetrics() => _schedule();
  ScrollPosition? _scroll;
  ThumbnailVisibility? _last;
  bool _scheduled = false;
  @override
  void didChangeDependencies() {
    super.didChangeDependencies();
    final next = Scrollable.maybeOf(context)?.position;
    if (next != _scroll) {
      _scroll?.removeListener(_schedule);
      _scroll = next;
      _scroll?.addListener(_schedule);
    }
    _schedule();
  }

  @override
  void didUpdateWidget(covariant ThumbnailViewportObserver oldWidget) {
    super.didUpdateWidget(oldWidget);
    _last = null;
    _schedule();
  }

  void _schedule() {
    if (!mounted || _scheduled) return;
    _scheduled = true;
    WidgetsBinding.instance.addPostFrameCallback((_) {
      _scheduled = false;
      if (mounted) _report();
    });
    WidgetsBinding.instance.ensureVisualUpdate();
  }

  void _report() {
    final box = context.findRenderObject();
    bool visible =
        box is RenderBox && box.attached && box.hasSize && !box.size.isEmpty;
    double position = 0;
    if (visible) {
      RenderObject? ancestor = box;
      while (ancestor != null) {
        if (ancestor is RenderOffstage && ancestor.offstage) {
          visible = false;
          break;
        }
        ancestor = ancestor.parent;
      }
      if (visible) {
        final viewport = RenderAbstractViewport.maybeOf(box);
        final bounds = MatrixUtils.transformRect(
          box.getTransformTo(viewport),
          box.paintBounds,
        );
        final clip =
            viewport?.paintBounds ?? (Offset.zero & MediaQuery.sizeOf(context));
        visible = bounds.overlaps(clip);
        position = box.localToGlobal(Offset.zero).dy;
      }
    }
    final next = ThumbnailVisibility(visible, position);
    if (_last?.visible != next.visible || _last?.position != next.position) {
      _last = next;
      widget.onVisibilityChanged(next);
    }
  }

  @override
  void dispose() {
    WidgetsBinding.instance.removeObserver(this);
    _scroll?.removeListener(_schedule);
    if (_last?.visible == true) {
      widget.onVisibilityChanged(const ThumbnailVisibility(false, 0));
    }
    super.dispose();
  }

  @override
  Widget build(BuildContext context) =>
      _LayoutObserver(onLayout: _schedule, child: widget.child);
}

class _LayoutObserver extends SingleChildRenderObjectWidget {
  const _LayoutObserver({required this.onLayout, required super.child});
  final VoidCallback onLayout;
  @override
  RenderObject createRenderObject(BuildContext context) =>
      _LayoutNotifier(onLayout);
  @override
  void updateRenderObject(
    BuildContext context,
    covariant _LayoutNotifier renderObject,
  ) {
    renderObject.onLayout = onLayout;
  }
}

class _LayoutNotifier extends RenderProxyBox {
  _LayoutNotifier(this.onLayout);
  VoidCallback onLayout;
  @override
  void performLayout() {
    super.performLayout();
    onLayout();
  }

  @override
  void paint(PaintingContext context, Offset offset) {
    super.paint(context, offset);
    onLayout();
  }
}

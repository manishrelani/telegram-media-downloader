import 'package:flutter/foundation.dart';
import 'package:flutter/material.dart';

@immutable
class QuietColors extends ThemeExtension<QuietColors> {
  const QuietColors({
    required this.canvas,
    required this.background,
    required this.sidebar,
    required this.text,
    required this.muted,
    required this.divider,
    required this.outline,
    required this.accent,
    required this.onAccent,
    required this.selected,
    required this.hover,
    required this.error,
    required this.errorSurface,
    required this.warning,
    required this.warningSurface,
  });

  final Color canvas, background, sidebar, text, muted, divider, outline;
  final Color accent,
      onAccent,
      selected,
      hover,
      error,
      errorSurface,
      warning,
      warningSurface;

  static const QuietColors light = QuietColors(
    canvas: Color(0xFFFFFFFF),
    background: Color(0xFFF6F7F4),
    sidebar: Color(0xFFF1F3EF),
    text: Color(0xFF25332E),
    muted: Color(0xFF627067),
    divider: Color(0xFFE2E7DF),
    outline: Color(0xFF7A887E),
    accent: Color(0xFF23664E),
    onAccent: Color(0xFFFFFFFF),
    selected: Color(0xFFEDF4EF),
    hover: Color(0xFFF5F7F3),
    error: Color(0xFFA23B2C),
    errorSurface: Color(0xFFFFF0EB),
    warning: Color(0xFF805714),
    warningSurface: Color(0xFFFFF4DC),
  );
  static const QuietColors dark = QuietColors(
    canvas: Color(0xFF1B221E),
    background: Color(0xFF151B17),
    sidebar: Color(0xFF171E19),
    text: Color(0xFFE4EBE6),
    muted: Color(0xFFAAB8AE),
    divider: Color(0xFF354239),
    outline: Color(0xFF83998A),
    accent: Color(0xFF9AD4B3),
    onAccent: Color(0xFF173C2A),
    selected: Color(0xFF283F31),
    hover: Color(0xFF243129),
    error: Color(0xFFFFB5A5),
    errorSurface: Color(0xFF422820),
    warning: Color(0xFFE0BD78),
    warningSurface: Color(0xFF382F1D),
  );

  @override
  QuietColors copyWith({
    Color? canvas,
    Color? background,
    Color? sidebar,
    Color? text,
    Color? muted,
    Color? divider,
    Color? outline,
    Color? accent,
    Color? onAccent,
    Color? selected,
    Color? hover,
    Color? error,
    Color? errorSurface,
    Color? warning,
    Color? warningSurface,
  }) => QuietColors(
    canvas: canvas ?? this.canvas,
    background: background ?? this.background,
    sidebar: sidebar ?? this.sidebar,
    text: text ?? this.text,
    muted: muted ?? this.muted,
    divider: divider ?? this.divider,
    outline: outline ?? this.outline,
    accent: accent ?? this.accent,
    onAccent: onAccent ?? this.onAccent,
    selected: selected ?? this.selected,
    hover: hover ?? this.hover,
    error: error ?? this.error,
    errorSurface: errorSurface ?? this.errorSurface,
    warning: warning ?? this.warning,
    warningSurface: warningSurface ?? this.warningSurface,
  );

  @override
  QuietColors lerp(QuietColors? other, double t) {
    if (other == null) return this;
    Color mix(Color a, Color b) => Color.lerp(a, b, t)!;
    return QuietColors(
      canvas: mix(canvas, other.canvas),
      background: mix(background, other.background),
      sidebar: mix(sidebar, other.sidebar),
      text: mix(text, other.text),
      muted: mix(muted, other.muted),
      divider: mix(divider, other.divider),
      outline: mix(outline, other.outline),
      accent: mix(accent, other.accent),
      onAccent: mix(onAccent, other.onAccent),
      selected: mix(selected, other.selected),
      hover: mix(hover, other.hover),
      error: mix(error, other.error),
      errorSurface: mix(errorSurface, other.errorSurface),
      warning: mix(warning, other.warning),
      warningSurface: mix(warningSurface, other.warningSurface),
    );
  }
}

extension QuietContext on BuildContext {
  QuietColors get quiet => Theme.of(this).extension<QuietColors>()!;
}

abstract final class QuietTheme {
  static ThemeData get light => _build(QuietColors.light, Brightness.light);
  static ThemeData get dark => _build(QuietColors.dark, Brightness.dark);

  static ThemeData _build(QuietColors colors, Brightness brightness) {
    final ColorScheme scheme =
        ColorScheme.fromSeed(
          seedColor: colors.accent,
          brightness: brightness,
        ).copyWith(
          primary: colors.accent,
          onPrimary: colors.onAccent,
          primaryContainer: colors.selected,
          onPrimaryContainer: colors.accent,
          surface: colors.canvas,
          onSurface: colors.text,
          onSurfaceVariant: colors.muted,
          outline: colors.outline,
          outlineVariant: colors.divider,
          error: colors.error,
          errorContainer: colors.errorSurface,
        );
    final String? font = defaultTargetPlatform == TargetPlatform.macOS
        ? '.AppleSystemUIFont'
        : null;
    final ThemeData base = ThemeData(
      useMaterial3: true,
      brightness: brightness,
      colorScheme: scheme,
      fontFamily: font,
    );
    final TextTheme text = base.textTheme
        .copyWith(
          headlineMedium: TextStyle(
            fontSize: 26,
            height: 1.25,
            fontWeight: FontWeight.w600,
            letterSpacing: -.45,
            color: colors.text,
          ),
          titleLarge: TextStyle(
            fontSize: 20,
            height: 1.3,
            fontWeight: FontWeight.w600,
            color: colors.text,
          ),
          titleMedium: TextStyle(
            fontSize: 15,
            height: 1.4,
            fontWeight: FontWeight.w500,
            color: colors.text,
          ),
          bodyLarge: TextStyle(fontSize: 15, height: 1.5, color: colors.text),
          bodyMedium: TextStyle(fontSize: 14, height: 1.45, color: colors.text),
          bodySmall: TextStyle(fontSize: 12, height: 1.4, color: colors.muted),
          labelLarge: TextStyle(
            fontSize: 13,
            height: 1.3,
            fontWeight: FontWeight.w500,
            color: colors.text,
          ),
          labelMedium: TextStyle(
            fontSize: 12,
            height: 1.35,
            fontWeight: FontWeight.w500,
            color: colors.muted,
          ),
        )
        .apply(fontFamily: font ?? 'Roboto');
    final WidgetStateProperty<BorderSide?> focusBorder =
        WidgetStateProperty.resolveWith(
          (Set<WidgetState> states) => states.contains(WidgetState.focused)
              ? BorderSide(color: colors.accent, width: 2)
              : const BorderSide(color: Colors.transparent, width: 2),
        );
    final RoundedRectangleBorder shape = RoundedRectangleBorder(
      borderRadius: BorderRadius.circular(8),
    );
    final ButtonStyle shared = ButtonStyle(
      textStyle: WidgetStatePropertyAll<TextStyle>(text.labelLarge!),
      minimumSize: const WidgetStatePropertyAll<Size>(Size(0, 42)),
      padding: const WidgetStatePropertyAll<EdgeInsetsGeometry>(
        EdgeInsets.symmetric(horizontal: 14, vertical: 11),
      ),
      shape: WidgetStatePropertyAll<OutlinedBorder>(shape),
      side: focusBorder,
      animationDuration:
          WidgetsBinding
              .instance
              .platformDispatcher
              .accessibilityFeatures
              .disableAnimations
          ? Duration.zero
          : const Duration(milliseconds: 140),
    );
    return base.copyWith(
      extensions: <ThemeExtension<dynamic>>[colors],
      scaffoldBackgroundColor: colors.background,
      textTheme: text,
      dividerTheme: DividerThemeData(
        color: colors.divider,
        thickness: 1,
        space: 1,
      ),
      iconTheme: IconThemeData(size: 20, color: colors.muted),
      filledButtonTheme: FilledButtonThemeData(
        style: shared.copyWith(
          backgroundColor: WidgetStateProperty.resolveWith(
            (Set<WidgetState> states) => states.contains(WidgetState.disabled)
                ? colors.divider
                : colors.accent,
          ),
          foregroundColor: WidgetStateProperty.resolveWith(
            (Set<WidgetState> states) => states.contains(WidgetState.disabled)
                ? colors.muted
                : colors.onAccent,
          ),
        ),
      ),
      textButtonTheme: TextButtonThemeData(
        style: shared.copyWith(
          foregroundColor: WidgetStatePropertyAll<Color>(colors.accent),
        ),
      ),
      outlinedButtonTheme: OutlinedButtonThemeData(
        style: shared.copyWith(
          foregroundColor: WidgetStatePropertyAll<Color>(colors.text),
          side: WidgetStateProperty.resolveWith(
            (Set<WidgetState> states) => BorderSide(
              color: states.contains(WidgetState.focused)
                  ? colors.accent
                  : colors.outline,
              width: states.contains(WidgetState.focused) ? 2 : 1,
            ),
          ),
        ),
      ),
      iconButtonTheme: IconButtonThemeData(
        style: IconButton.styleFrom(
          foregroundColor: colors.muted,
          minimumSize: const Size(40, 40),
          shape: shape,
        ).copyWith(side: focusBorder),
      ),
      checkboxTheme: CheckboxThemeData(
        shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(3)),
        side: WidgetStateBorderSide.resolveWith(
          (Set<WidgetState> states) => BorderSide(
            color: states.contains(WidgetState.focused)
                ? colors.accent
                : colors.outline,
            width: states.contains(WidgetState.focused) ? 2 : 1.3,
          ),
        ),
        checkColor: WidgetStatePropertyAll<Color>(colors.onAccent),
        fillColor: WidgetStateProperty.resolveWith(
          (Set<WidgetState> states) => states.contains(WidgetState.selected)
              ? colors.accent
              : colors.canvas,
        ),
      ),
      tooltipTheme: TooltipThemeData(
        waitDuration: const Duration(milliseconds: 450),
        textStyle: text.bodySmall!.copyWith(color: colors.onAccent),
        decoration: BoxDecoration(
          color: colors.accent,
          borderRadius: BorderRadius.circular(6),
        ),
      ),
      inputDecorationTheme: InputDecorationTheme(
        filled: true,
        fillColor: colors.canvas,
        isDense: true,
        labelStyle: text.bodySmall,
        hintStyle: text.bodyMedium!.copyWith(color: colors.muted),
        border: OutlineInputBorder(
          borderRadius: BorderRadius.circular(8),
          borderSide: BorderSide(color: colors.outline),
        ),
        enabledBorder: OutlineInputBorder(
          borderRadius: BorderRadius.circular(8),
          borderSide: BorderSide(color: colors.outline),
        ),
        focusedBorder: OutlineInputBorder(
          borderRadius: BorderRadius.circular(8),
          borderSide: BorderSide(color: colors.accent, width: 2),
        ),
        contentPadding: const EdgeInsets.symmetric(
          horizontal: 12,
          vertical: 12,
        ),
      ),
      progressIndicatorTheme: ProgressIndicatorThemeData(
        color: colors.accent,
        linearTrackColor: colors.divider,
        linearMinHeight: 3,
        borderRadius: BorderRadius.circular(3),
      ),
      menuTheme: MenuThemeData(
        style: MenuStyle(
          backgroundColor: WidgetStatePropertyAll<Color>(colors.canvas),
          surfaceTintColor: const WidgetStatePropertyAll<Color>(
            Colors.transparent,
          ),
          shape: WidgetStatePropertyAll<OutlinedBorder>(
            RoundedRectangleBorder(
              borderRadius: BorderRadius.circular(10),
              side: BorderSide(color: colors.divider),
            ),
          ),
          elevation: const WidgetStatePropertyAll<double>(6),
          padding: const WidgetStatePropertyAll<EdgeInsetsGeometry>(
            EdgeInsets.symmetric(vertical: 6),
          ),
        ),
      ),
      dialogTheme: DialogThemeData(
        backgroundColor: colors.canvas,
        surfaceTintColor: Colors.transparent,
        shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(14)),
        titleTextStyle: text.titleLarge,
        contentTextStyle: text.bodyMedium,
      ),
      snackBarTheme: SnackBarThemeData(
        backgroundColor: colors.text,
        contentTextStyle: text.bodyMedium!.copyWith(color: colors.canvas),
        behavior: SnackBarBehavior.floating,
      ),
      scrollbarTheme: ScrollbarThemeData(
        radius: const Radius.circular(8),
        thickness: const WidgetStatePropertyAll<double>(5),
        thumbColor: WidgetStatePropertyAll<Color>(
          colors.outline.withValues(alpha: .5),
        ),
      ),
    );
  }
}

Duration quietMotion(BuildContext context, [int milliseconds = 140]) =>
    MediaQuery.disableAnimationsOf(context)
    ? Duration.zero
    : Duration(milliseconds: milliseconds);

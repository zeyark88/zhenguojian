import 'package:flutter/foundation.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';

export 'app_build.dart';

const appVersion = '0.2.18';

/// 平板主题：稍大的字号与更宽松的触控间距（非电视，不启用遥控焦点高亮）。
ThemeData tabletTheme(ThemeData theme) {
  final text = theme.textTheme;
  return theme.copyWith(
    visualDensity: VisualDensity.comfortable,
    textTheme: text.copyWith(
      bodyLarge: text.bodyLarge?.copyWith(fontSize: 17),
      bodyMedium: text.bodyMedium?.copyWith(fontSize: 16),
      bodySmall: text.bodySmall?.copyWith(fontSize: 14),
      labelLarge: text.labelLarge?.copyWith(fontSize: 16),
    ),
    filledButtonTheme: FilledButtonThemeData(
      style: FilledButton.styleFrom(
        minimumSize: const Size(56, 48),
        textStyle: const TextStyle(fontSize: 16),
      ),
    ),
    iconButtonTheme: IconButtonThemeData(
      style: IconButton.styleFrom(
        minimumSize: const Size(46, 46),
      ),
    ),
  );
}

ThemeData televisionTheme(ThemeData theme) {
  final colors = theme.colorScheme;
  final focusSide = WidgetStateProperty.resolveWith<BorderSide?>(
    (states) => states.contains(WidgetState.focused)
        ? BorderSide(color: colors.primary, width: 3)
        : null,
  );
  final focusBackground = WidgetStateProperty.resolveWith<Color?>(
    (states) =>
        states.contains(WidgetState.focused) ? colors.primaryContainer : null,
  );
  final button = ButtonStyle(
    side: focusSide,
    minimumSize: const WidgetStatePropertyAll(Size(52, 48)),
    textStyle: const WidgetStatePropertyAll(TextStyle(fontSize: 17)),
  );
  return theme.copyWith(
    focusColor: colors.primaryContainer,
    iconButtonTheme: IconButtonThemeData(
      style: button.copyWith(backgroundColor: focusBackground),
    ),
    filledButtonTheme: FilledButtonThemeData(style: button),
    outlinedButtonTheme: OutlinedButtonThemeData(style: button),
    textButtonTheme: TextButtonThemeData(
      style: button.copyWith(backgroundColor: focusBackground),
    ),
    inputDecorationTheme: theme.inputDecorationTheme.copyWith(
      focusedBorder: OutlineInputBorder(
        borderRadius: BorderRadius.circular(16),
        borderSide: BorderSide(color: colors.primary, width: 3),
      ),
    ),
  );
}

class AppDevice {
  const AppDevice({this.kind = 'phone', this.version = appVersion});
  /// 设备类型：'phone'（手机）/ 'tablet'（平板）/ 'tv'（电视）。
  final String kind;
  bool get television => kind == 'tv';
  bool get tablet => kind == 'tablet';
  final String version;
  static const channel = MethodChannel('duanju/device');

  static Future<AppDevice> detect() async {
    if (defaultTargetPlatform != TargetPlatform.android) {
      return const AppDevice();
    }
    try {
      final data = await channel.invokeMapMethod<String, dynamic>('deviceInfo');
      final tv = data?['television'] == true;
      final tab = !tv && data?['tablet'] == true;
      return AppDevice(
        kind: tv ? 'tv' : tab ? 'tablet' : 'phone',
        version: data?['version'] as String? ?? appVersion,
      );
    } on PlatformException {
      return const AppDevice();
    } on MissingPluginException {
      return const AppDevice();
    }
  }
}

class AppLayout extends InheritedWidget {
  const AppLayout({
    super.key,
    required this.kind,
    this.version = appVersion,
    required super.child,
  });
  /// 设备类型：phone / tablet / tv。
  final String kind;
  bool get television => kind == 'tv';
  bool get tablet => kind == 'tablet';
  final String version;

  static bool isTelevision(BuildContext context) =>
      context.dependOnInheritedWidgetOfExactType<AppLayout>()?.kind == 'tv';
  static bool isTablet(BuildContext context) =>
      context.dependOnInheritedWidgetOfExactType<AppLayout>()?.kind == 'tablet';
  static String versionOf(BuildContext context) =>
      context.dependOnInheritedWidgetOfExactType<AppLayout>()?.version ??
      appVersion;

  @override
  bool updateShouldNotify(AppLayout oldWidget) =>
      kind != oldWidget.kind || version != oldWidget.version;
}

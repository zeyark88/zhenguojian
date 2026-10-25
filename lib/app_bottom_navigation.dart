import 'package:flutter/material.dart';

class AppBottomNavigation extends StatelessWidget {
  const AppBottomNavigation({
    super.key,
    required this.selectedIndex,
    required this.onDestinationSelected,
    required this.destinations,
    this.onDestinationDoubleTap,
    this.onDestinationDoubleTapIndex,
  });

  final int selectedIndex;
  final ValueChanged<int> onDestinationSelected;
  final List<NavigationDestination> destinations;

  /// 双击某个标签时的回调（例如双击"首页"重新随机）。
  final ValueChanged<int>? onDestinationDoubleTap;

  /// 允许双击生效的标签索引；为空则对所有标签生效。
  /// 限制后其它标签只保留单击，避免连点被双击判定抢占而丢失 onTap。
  final int? onDestinationDoubleTapIndex;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final colors = theme.colorScheme;
    return Material(
      color: theme.scaffoldBackgroundColor,
      child: SafeArea(
        top: false,
        child: DecoratedBox(
          decoration: BoxDecoration(
            border: Border(
              top: BorderSide(color: colors.outlineVariant, width: .5),
            ),
          ),
          child: Row(
            children: [
              for (final (index, destination) in destinations.indexed)
                Expanded(
                  child: Semantics(
                    container: true,
                    button: true,
                    selected: index == selectedIndex,
                    child: Tooltip(
                      message: destination.label,
                      excludeFromSemantics: true,
                      child: InkWell(
                        key: ValueKey('bottom-nav-$index'),
                        onTap: () => onDestinationSelected(index),
                        onDoubleTap: onDestinationDoubleTap == null ||
                                (onDestinationDoubleTapIndex != null &&
                                    index != onDestinationDoubleTapIndex)
                            ? null
                            : () => onDestinationDoubleTap!(index),
                        child: ConstrainedBox(
                          constraints: const BoxConstraints(
                            minHeight: 68,
                            minWidth: 48,
                          ),
                          child: Padding(
                            padding: const EdgeInsets.symmetric(
                              horizontal: 4,
                              vertical: 9,
                            ),
                            child: Column(
                              mainAxisSize: MainAxisSize.min,
                              children: [
                                AnimatedContainer(
                                  duration: const Duration(milliseconds: 160),
                                  curve: Curves.easeOut,
                                  width: 18,
                                  height: 3,
                                  decoration: BoxDecoration(
                                    color: index == selectedIndex
                                        ? colors.primary
                                        : Colors.transparent,
                                    borderRadius: BorderRadius.circular(1.5),
                                  ),
                                ),
                                const SizedBox(height: 6),
                                IconTheme(
                                  data: IconThemeData(
                                    size: 24,
                                    color: index == selectedIndex
                                        ? colors.primary
                                        : colors.onSurfaceVariant,
                                  ),
                                  child: index == selectedIndex
                                      ? destination.selectedIcon ??
                                            destination.icon
                                      : destination.icon,
                                ),
                                const SizedBox(height: 4),
                                Text(
                                  destination.label,
                                  textAlign: TextAlign.center,
                                  maxLines: 2,
                                  overflow: TextOverflow.ellipsis,
                                  style: TextStyle(
                                    fontSize: 12,
                                    height: 1.2,
                                    fontWeight: index == selectedIndex
                                        ? FontWeight.w700
                                        : FontWeight.w500,
                                    color: index == selectedIndex
                                        ? colors.primary
                                        : colors.onSurfaceVariant,
                                  ),
                                ),
                              ],
                            ),
                          ),
                        ),
                      ),
                    ),
                  ),
                ),
            ],
          ),
        ),
      ),
    );
  }
}

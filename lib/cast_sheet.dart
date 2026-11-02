import 'package:flutter/material.dart';

import 'cast_controller.dart';

/// 投屏设备选择弹窗：搜索局域网 DLNA 设备并选择投屏目标。
Future<CastDevice?> showCastSheet(BuildContext context, CastController cast) {
  return showModalBottomSheet<CastDevice>(
    context: context,
    backgroundColor: const Color(0xFF191A20),
    shape: const RoundedRectangleBorder(
      borderRadius: BorderRadius.vertical(top: Radius.circular(16)),
    ),
    builder: (_) => CastSheet(cast: cast),
  );
}

class CastSheet extends StatefulWidget {
  const CastSheet({super.key, required this.cast});

  final CastController cast;

  @override
  State<CastSheet> createState() => _CastSheetState();
}

class _CastSheetState extends State<CastSheet> {
  @override
  void initState() {
    super.initState();
    widget.cast.search();
  }

  @override
  Widget build(BuildContext context) {
    return SafeArea(
      child: ListenableBuilder(
        listenable: widget.cast,
        builder: (context, _) {
          final cast = widget.cast;
          final children = <Widget>[
            const Padding(
              padding: EdgeInsets.fromLTRB(20, 16, 20, 4),
              child: Align(
                child: Text(
                  '投屏到设备',
                  style: TextStyle(
                    color: Colors.white,
                    fontSize: 16,
                    fontWeight: FontWeight.w600,
                  ),
                ),
              ),
            ),
            Padding(
              padding: const EdgeInsets.fromLTRB(20, 0, 20, 12),
              child: Align(
                child: Text(
                  cast.searching ? '正在搜索局域网设备…' : '请选择设备（需与手机同一 Wi-Fi）',
                  style: const TextStyle(color: Color(0xFFB9B6C2), fontSize: 12),
                ),
              ),
            ),
          ];
          if (cast.error != null) {
            children.add(Padding(
              padding: const EdgeInsets.all(20),
              child: Text(
                cast.error!,
                style: const TextStyle(color: Color(0xFFFF8A80), fontSize: 13),
              ),
            ));
          }
          for (final device in cast.devices) {
            children.add(ListTile(
              key: ValueKey('cast-${device.id}'),
              leading: const Icon(Icons.tv_rounded, color: Color(0xFFFF765F)),
              title: Text(device.name,
                  style: const TextStyle(color: Colors.white, fontSize: 15)),
              subtitle: Text(
                device.server.isEmpty ? 'DLNA' : device.server,
                maxLines: 1,
                overflow: TextOverflow.ellipsis,
                style: const TextStyle(color: Color(0xFFB9B6C2), fontSize: 12),
              ),
              onTap: () => Navigator.pop(context, device),
            ));
          }
          if (!cast.searching && cast.devices.isEmpty && cast.error == null) {
            children.add(const Padding(
              padding: EdgeInsets.all(24),
              child: Align(
                child: Text(
                  '未找到设备，请确认电视与手机在同一 Wi-Fi 且支持 DLNA',
                  style: TextStyle(color: Color(0xFFB9B6C2), fontSize: 13),
                ),
              ),
            ));
          }
          children.add(const SizedBox(height: 12));
          return SingleChildScrollView(child: Column(children: children));
        },
      ),
    );
  }
}

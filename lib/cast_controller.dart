import 'dart:async';

import 'package:flutter/foundation.dart';

import 'core_bridge.dart';
import 'models.dart';

/// DLNA 投屏：搜索局域网设备并把当前视频投到电视。
class CastController extends ChangeNotifier {
  CastController(this._repository);

  final AppRepository _repository;

  List<CastDevice> devices = [];
  bool searching = false;
  String? error;

  /// 投屏中的设备名；null = 未投。
  String? castingTo;

  /// 投屏目标设备（换集续投）。
  CastDevice? _target;

  CastDevice? get target => _target;

  Future<void> search() async {
    if (searching) return;
    searching = true;
    error = null;
    devices = [];
    notifyListeners();
    try {
      final result = await _repository.castDiscover();
      devices = result;
    } catch (e) {
      error = e.toString();
    } finally {
      searching = false;
      notifyListeners();
    }
  }

  /// 投当前计划到目标设备；返回是否成功。
  Future<bool> start(CastDevice device, PlaybackPlan plan, String title) async {
    if (plan.url.isEmpty || plan.local) {
      error = '本视频不支持投屏';
      notifyListeners();
      return false;
    }
    try {
      await _repository.castStart(device, plan, title);
      _target = device;
      castingTo = device.name;
      error = null;
      notifyListeners();
      return true;
    } catch (e) {
      error = e.toString();
      notifyListeners();
      return false;
    }
  }

  /// 换集后继续投到同一设备。
  Future<bool> continueCast(PlaybackPlan plan, String title) async {
    final device = _target;
    if (device == null || castingTo == null) return false;
    return start(device, plan, title);
  }

  /// 断开投屏（回到本机播放）。
  void stop() {
    _target = null;
    castingTo = null;
    error = null;
    notifyListeners();
  }
}

/// 一台可投屏设备。
class CastDevice {
  const CastDevice({required this.id, required this.name, this.server = ''});

  factory CastDevice.fromJson(Map<String, dynamic> json) => CastDevice(
        id: json['id'] as String? ?? '',
        name: json['name'] as String? ?? 'DLNA 设备',
        server: json['server'] as String? ?? '',
      );

  final String id;
  final String name;
  final String server;

  Map<String, dynamic> toJson() => {'id': id, 'name': name, 'server': server};
}

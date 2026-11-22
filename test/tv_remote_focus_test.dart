// 0.9.1（TV 遥控修复）回归：
// 1. 详情页异步加载完成后，遥控焦点必须落到底部播放按钮上，
//    且按「上」能进入选集网格（此前焦点丢在路由级 FocusScope 上，
//    选集区 / 底部栏全部无法用遥控选取，见用户真机截图反馈）。
// 2. 竖滑流首页（TV 布局）按「下」能聚焦底部控件条（倍速按钮起），
//    「上」能逐行回到画面焦点（此前上/下键全部被切剧消费、
//    确认键全部被播放/暂停消费，控件条永远拿不到焦点）。
import 'dart:async';

import 'package:duanju_app/app_layout.dart';
import 'package:duanju_app/detail_screen.dart';
import 'package:duanju_app/local_store.dart';
import 'package:duanju_app/models.dart';
import 'package:duanju_app/vertical_feed_screen.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:media_kit/media_kit.dart';
import 'package:shared_preferences/shared_preferences.dart';

import 'fixtures.dart';
import 'player_fixtures.dart';

class _AsyncDetailRepository extends FixtureRepository {
  final completer = Completer<void>();

  @override
  Future<DramaDetail> detail(Drama drama) async {
    await completer.future;
    return DramaDetail(
      drama,
      List.generate(
        60,
        (index) => Episode({'id': '${index + 1}'}, index + 1),
      ),
    );
  }
}

/// 选集弹窗用例需要可跨行的多集数据：RouteRepository 继承的
/// FixtureRepository.detail 只有 2 集，5 列网格里全在第 0 行，
// «下键跨行移动»永远无从验证（0.9.5/0.9.6 的旧断言只要求焦点
// 换个地方，下键把焦点甩出网格外也能蒙混过关）。
class _WideRouteRepository extends RouteRepository {
  @override
  Future<DramaDetail> detail(Drama drama) async => DramaDetail(
        drama,
        List.generate(
          60,
          (index) => Episode({'id': '${index + 1}'}, index + 1),
        ),
      );
}

Widget _tvHost(Widget child) => MaterialApp(
  theme: ThemeData.dark(),
  builder: (_, navigatorChild) =>
      AppLayout(kind: 'tv', child: navigatorChild!),
  home: child,
);

void main() {
  Future<LocalStore> makeStore() async {
    SharedPreferences.setMockInitialValues(const {});
    final store = LocalStore(await SharedPreferences.getInstance());
    addTearDown(store.dispose);
    return store;
  }

  Future<void> press(WidgetTester tester, LogicalKeyboardKey key) async {
    await tester.sendKeyEvent(key);
    await tester.pump(const Duration(milliseconds: 50));
  }

  FocusNode nodeInside(WidgetTester tester, Finder finder) =>
      Focus.of(tester.element(finder));

  testWidgets('TV：详情异步加载完成后焦点落到底部播放按钮，上键可进入选集', (tester) async {
    tester.view.devicePixelRatio = 1;
    tester.view.physicalSize = const Size(1280, 720);
    addTearDown(tester.view.resetDevicePixelRatio);
    addTearDown(tester.view.resetPhysicalSize);

    final store = await makeStore();
    final repository = _AsyncDetailRepository();
    await tester.pumpWidget(
      _tvHost(
        DetailScreen(
          drama: FixtureRepository.free,
          repository: repository,
          store: store,
        ),
      ),
    );
    // 加载中：播放按钮禁用，autofocus 被跳过（这正是真机失焦的根源）。
    await tester.pump();
    expect(find.byKey(const ValueKey('episode-1')), findsNothing);

    // 详情就绪：补落焦点 → primary focus 应在「立即播放」按钮上。
    repository.completer.complete();
    await tester.pumpAndSettle();
    expect(find.byKey(const ValueKey('episode-1')), findsOneWidget);
    final playLabel = find.descendant(
      of: find.byKey(const ValueKey('start-play')),
      matching: find.textContaining('播放'),
    );
    expect(
      nodeInside(tester, playLabel).hasFocus,
      isTrue,
      reason: '详情加载完成后，遥控焦点应落在底部播放按钮上',
    );

    // 上键：焦点进入选集网格（任意一个剧集按钮）。
    await press(tester, LogicalKeyboardKey.arrowUp);
    await tester.pumpAndSettle();
    expect(
      RegExp(r'^remote-\d+$').hasMatch(
        FocusManager.instance.primaryFocus?.debugLabel ?? '',
      ),
      isTrue,
      reason: '从播放按钮按上键，应能聚焦选集网格中的剧集按钮',
    );
    expect(tester.takeException(), isNull);
  });

  testWidgets('TV：首页竖滑流按下键聚焦控件条，上键逐行回到画面', (tester) async {
    tester.view.devicePixelRatio = 1;
    tester.view.physicalSize = const Size(420, 880);
    addTearDown(tester.view.resetDevicePixelRatio);
    addTearDown(tester.view.resetPhysicalSize);

    final store = await makeStore();
    final repository = RouteRepository();
    final player = ScriptedPlayer();

    Future<void> pumpHost() => tester.pumpWidget(
      _tvHost(
        Scaffold(
          body: VerticalFeedScreen(
            key: const ValueKey('feed'),
            repository: repository,
            store: store,
            embedded: true,
            singleDrama: FixtureRepository.free,
            active: true,
            playerFactory: () => Player(platformPlayer: player),
            videoBuilder: (controls) => controls,
          ),
        ),
      ),
    );

    await pumpHost();
    for (var i = 0; i < 20; i++) {
      await tester.pump(const Duration(milliseconds: 20));
    }
    final surface = Focus.of(
      tester.element(find.byKey(const ValueKey('feed-focus-root'))),
    );
    expect(find.byKey(const ValueKey('feed-speed')), findsOneWidget);

    // 确认键：暂停（同时弹出/保持控件条显示）。
    await press(tester, LogicalKeyboardKey.enter);
    await tester.pump(const Duration(milliseconds: 200));

    // 0.9.4：TV 上画面点击（合成 tap）只能拉出控件条、不能翻回隐藏。
    // 模拟电视系统跟随 OK 键派发的合成 tap 验证控件条仍在。
    await tester.tap(find.byKey(const ValueKey('feed-focus-root')));
    await tester.pump(const Duration(milliseconds: 120));
    expect(find.byKey(const ValueKey('feed-speed')), findsOneWidget);

    // 下键：进入控件条，落在首按钮（倍速）上。
    await press(tester, LogicalKeyboardKey.arrowDown);
    await tester.pump(const Duration(milliseconds: 120));
    final speedIcon = find
        .descendant(
          of: find.byKey(const ValueKey('feed-speed')),
          matching: find.byType(Icon),
        )
        .first;
    expect(
      nodeInside(tester, speedIcon).hasFocus,
      isTrue,
      reason: 'TV 布局下，控件条可见时按「下」应聚焦底部控件条首按钮',
    );
    expect(
      FocusManager.instance.primaryFocus,
      isNot(equals(surface)),
      reason: '焦点应已离开画面本身',
    );

    // 上键：从底部功能行移到进度行（焦点仍在控件条内）。
    await press(tester, LogicalKeyboardKey.arrowUp);
    await tester.pump(const Duration(milliseconds: 120));
    expect(
      FocusManager.instance.primaryFocus,
      isNot(equals(surface)),
      reason: '功能行按「上」应先到进度行，而不是直接退出控件条',
    );

    // 再上键：回到画面焦点。
    await press(tester, LogicalKeyboardKey.arrowUp);
    await tester.pump(const Duration(milliseconds: 120));
    expect(
      FocusManager.instance.primaryFocus,
      equals(surface),
      reason: '从进度行再按「上」应回到画面焦点（父级整页节点）',
    );
    expect(tester.takeException(), isNull);
  });

  testWidgets('TV：选集弹窗打开后焦点落在当前集（RemoteGrid），方向键可移动（0.9.7）', (tester) async {
    tester.view.devicePixelRatio = 1;
    tester.view.physicalSize = const Size(420, 880);
    addTearDown(tester.view.resetDevicePixelRatio);
    addTearDown(tester.view.resetPhysicalSize);

    final store = await makeStore();
    final repository = _WideRouteRepository();
    final player = ScriptedPlayer();
    await tester.pumpWidget(
      _tvHost(
        Scaffold(
          body: VerticalFeedScreen(
            key: const ValueKey('feed'),
            repository: repository,
            store: store,
            embedded: true,
            singleDrama: FixtureRepository.free,
            active: true,
            playerFactory: () => Player(platformPlayer: player),
            videoBuilder: (controls) => controls,
          ),
        ),
      ),
    );
    for (var i = 0; i < 20; i++) {
      await tester.pump(const Duration(milliseconds: 20));
    }

    // 下键进入控制条 → 右移到「选集」 → OK 打开弹窗。
    await press(tester, LogicalKeyboardKey.arrowDown);
    await tester.pump(const Duration(milliseconds: 200));
    await press(tester, LogicalKeyboardKey.arrowRight);
    await press(tester, LogicalKeyboardKey.arrowRight);
    await tester.pump(const Duration(milliseconds: 120));
    final episodesIcon = find
        .descendant(
          of: find.byKey(const ValueKey('feed-episodes')),
          matching: find.byType(Icon),
        )
        .first;
    final episodesNode = nodeInside(tester, episodesIcon);
    if (!episodesNode.hasFocus) {
      await press(tester, LogicalKeyboardKey.arrowRight);
      await tester.pump(const Duration(milliseconds: 120));
    }
    expect(
      nodeInside(tester, find
          .descendant(
            of: find.byKey(const ValueKey('feed-episodes')),
            matching: find.byType(Icon),
          )
          .first).hasFocus,
      isTrue,
      reason: '应先聚焦到「选集」按钮（本用例前置步骤）',
    );

    await press(tester, LogicalKeyboardKey.enter);
    await tester.pumpAndSettle();

    // 弹窗已打开：当前集（第 1 集）应持有焦点。
    // 0.9.7：弹窗网格复用详情页 RemoteGrid，焦点节点的 debugLabel
    // 为 remote-{index}（与 0.9.5/0.9.6 手造的 feed-episode-{i} 不同）。
    expect(find.textContaining('选集'), findsWidgets);
    final focusedLabel =
        FocusManager.instance.primaryFocus?.debugLabel ?? '';
    expect(
      RegExp(r'^remote-\d+$').hasMatch(focusedLabel),
      isTrue,
      reason: 'TV 上选集弹窗打开后，焦点应自动落在当前集按钮，实际：$focusedLabel',
    );

    // 右键：焦点移动到第 2 集。
    await press(tester, LogicalKeyboardKey.arrowRight);
    await tester.pump(const Duration(milliseconds: 150));
    expect(
      FocusManager.instance.primaryFocus?.debugLabel,
      'remote-1',
      reason: '右键应把焦点从第 1 集移到第 2 集',
    );

    // 下键：焦点跨行移动到下一行同列（420 逻辑宽 → 5 列，1 + 5 = 6）。
    await press(tester, LogicalKeyboardKey.arrowDown);
    await tester.pump(const Duration(milliseconds: 150));
    expect(
      FocusManager.instance.primaryFocus?.debugLabel,
      'remote-6',
      reason: '下键应在网格内跨行移动焦点',
    );
    expect(tester.takeException(), isNull);
  });

  testWidgets('TV：播放中控制条隐藏时按下键直接拉出并聚焦（0.9.4 加固）', (tester) async {
    tester.view.devicePixelRatio = 1;
    tester.view.physicalSize = const Size(420, 880);
    addTearDown(tester.view.resetDevicePixelRatio);
    addTearDown(tester.view.resetPhysicalSize);

    final store = await makeStore();
    final repository = RouteRepository();
    final player = ScriptedPlayer();

    Future<void> pumpHost() => tester.pumpWidget(
      _tvHost(
        Scaffold(
          body: VerticalFeedScreen(
            key: const ValueKey('feed'),
            repository: repository,
            store: store,
            embedded: true,
            singleDrama: FixtureRepository.free,
            active: true,
            playerFactory: () => Player(platformPlayer: player),
            videoBuilder: (controls) => controls,
          ),
        ),
      ),
    );

    await pumpHost();
    for (var i = 0; i < 20; i++) {
      await tester.pump(const Duration(milliseconds: 20));
    }
    final surface = Focus.of(
      tester.element(find.byKey(const ValueKey('feed-focus-root'))),
    );

    // 等隐藏计时（4 秒）跑完：播放中控制条自动隐藏。
    await tester.pump(const Duration(seconds: 5));

    // 下键（控制条隐藏状态）：应拉出控制条并聚焦倍速，而不是切剧。
    await press(tester, LogicalKeyboardKey.arrowDown);
    await tester.pump(const Duration(milliseconds: 200));
    final speedIcon = find
        .descendant(
          of: find.byKey(const ValueKey('feed-speed')),
          matching: find.byType(Icon),
        )
        .first;
    expect(
      nodeInside(tester, speedIcon).hasFocus,
      isTrue,
      reason: '0.9.4：控制条隐藏时按「下」也应拉出并聚焦首按钮（真机场景加固）',
    );
    expect(
      FocusManager.instance.primaryFocus,
      isNot(equals(surface)),
    );
    expect(tester.takeException(), isNull);
  });
}

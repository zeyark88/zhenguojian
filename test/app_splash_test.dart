import 'package:duanju_app/app_splash.dart';
import 'package:flutter/foundation.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

const _splashMark = '短剧 · 漫剧 · AI 剧';

Widget _wrap(ValueListenable<bool> ready) => MaterialApp(
  home: AppSplash(ready: ready, child: const Text('MAIN')),
);

void main() {
  testWidgets('首页始终未就绪时，超过 timeout 必须强制淡出', (tester) async {
    final ready = ValueNotifier<bool>(false);
    await tester.pumpWidget(_wrap(ready));

    expect(find.text(_splashMark), findsOneWidget);

    await tester.pump(const Duration(milliseconds: 2900));
    expect(find.text(_splashMark), findsOneWidget, reason: '未到 3 秒仍应停留在开屏页');

    await tester.pump(const Duration(milliseconds: 200));
    await tester.pump();
    expect(
      find.text(_splashMark),
      findsNothing,
      reason: '首页一条剧都没拉回来时，也必须在超时后放行，否则用户永远进不去',
    );
    expect(find.text('MAIN'), findsOneWidget);
    ready.dispose();
  });

  testWidgets('首页提前就绪时，最短展示后即淡出，不等满 timeout', (tester) async {
    final ready = ValueNotifier<bool>(false);
    await tester.pumpWidget(_wrap(ready));

    ready.value = true;
    await tester.pump(const Duration(milliseconds: 600));
    await tester.pump();
    expect(find.text(_splashMark), findsNothing);
    ready.dispose();
  });
}

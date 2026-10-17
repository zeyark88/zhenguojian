# 真果鉴 · 个人优化源码（基于上游 0.7.0+60）

> 本仓库是**纯源码仓库**：只托管源码与构建方法，**不提供、不分发任何编译产物（APK 等安装包）**。
> 仅供个人学习、研究与技术交流使用，**不是官方发布仓库**。

## 原项目与出处声明

- **原始项目**：[Topkill/guoapp3](https://github.com/Topkill/guoapp3) —— 原作者 **Topkill**，版权归原作者所有
- **上游源码**：[kuoluoss/zhenguojian](https://github.com/kuoluoss/zhenguojian)（基于 Topkill/guoapp3 构建与维护的发布仓库，维护者 **软萌小正太**）
- 本仓库基于上游 **v0.7.0+60**（main 分支 `b3de92f`，2026-10-17）源码进行修改
- 原上游 README 已完整保留于 [docs/UPSTREAM-README.md](docs/UPSTREAM-README.md)，其中包含上游完整的版本历史说明
- 感谢原作者 Topkill 与上游维护者软萌小正太的付出

## 本仓库相对上游的源码修改

功能裁剪：

1. **移除应用内更新模块** —— 删除启动检查更新、关于页“检测新版本”入口、APK 下载安装逻辑，同时移除 `REQUEST_INSTALL_PACKAGES` 权限与对应 FileProvider。
2. **移除关于页应用内署名** —— 上游在“关于”对话框中署名原作者与维护者；本构建移除了该应用内条目（原作者与出处信息改由本 README 声明，见上文）。
3. **可选双包共存** —— 提供将“红果鉴”（仅红果站源）改为独立包名 `com.duanju.duanju_app.hongguo` 的构建方式，使全站源版与单源版可各自编译、独立安装（构建期决定，见下文构建说明第 3 步）。

交互改动：

4. **启动直达主框架** —— 移除启动开屏页（品牌页 + 转圈遮挡），进入应用立即显示带导航的完整框架（首页/发现/追剧/最近观看/下载均可立即操作），首页数据在框架内以骨架屏占位加载。
5. **骨架屏加载提示** —— 多源加载时显示“正在从 N 个站源加载，可先看其他标签”。

性能优化：

6. **Dart AOT 混淆与符号剥离**（`--obfuscate --split-debug-info`），缩减 libapp.so 并加快加载。
7. **Go 核心软内存上限 512MB**（`native/core/app_memory_limit.go`），约束堆水位，降低与视频解码库共存时被系统查杀的概率。
8. **图片缓存峰值收敛 100MB → 64MB**（封面已按 440 宽下采样）。
9. **启动路径并行化**（SharedPreferences 与设备检测并行执行）。
10. **首页网格 RepaintBoundary 重绘隔离**。
11. **下一集预载触发提前 600ms → 350ms**（保留防抖）。
12. **多源目录并发提升** —— Go 全局目录并发 3 → 6、同源请求间隔 250ms → 150ms（均在原设置项允许范围内），Dart 目录 worker 并发 2 → 6。

构建修正：

13. **锁文件保持上游原版** —— 全程使用 `--enforce-lockfile` 由构建脚本镜像转换，杜绝依赖漂移（曾因手动 `pub get` 导致 `shared_preferences_android` 升到 2.5.0，与混淆构建冲突引发启动崩溃，已修复并回归锁定 2.4.28）。**构建时请勿绕过构建脚本手动执行 `flutter pub get`。**
14. **Release 签名方案说明** —— 自行构建时可用 `apksigner` 附加 `--v1-signing-enabled true --v2-signing-enabled true --v3-signing-enabled true`，对各类安装器有更好的兼容性。

并入的上游版本（保留上游全部官方修复）：

- **0.7.0+60**：开 VPN/代理时站源自动改直连重试；黄果 AI 多域名并行竞速；黄果旧版 VPN 下“登录太频繁”修复；黄果视频更名“黄果剧场”并换域名 `huangguoai.fit`
- **0.6.11+59**：双击暂停后不再自动恢复播放；切下一视频偶发黑屏（约 10 秒内自愈）；源更新提示文案优化；站源选择改为多选弹窗
- **0.6.10+58**：红果源播放接口补 `X-Argus` 请求头（修复“目录正常、播放为空”）

## 从源码构建

克隆后需先用 Go 工具链编译原生核心生成 `libduanju_core.so`（构建脚本会自动完成）。

**环境要求**：

- Flutter（版本与上游 `pubspec.yaml` 约束一致）+ Go + JDK 17 + Android SDK（platform 36 / build-tools / NDK）
- **必须在纯英文路径下执行**（中文路径会导致构建失败）

**步骤**：

1. 配置自己的签名（仓库不含任何密钥）：在 `android/` 下放置你的 `key.properties` 与 keystore 文件；
2. 构建“真果鉴”（全站源）：

   ```bash
   py scripts/build_android.py --abi arm64-v8a --all-sources --cn-mirrors
   ```

3. 构建“红果鉴”独立包名共存版（仅红果源）：先把 `android/app/build.gradle.kts` 中 `applicationId` 改为 `com.duanju.duanju_app.hongguo`，再执行：

   ```bash
   py scripts/build_android.py --abi arm64-v8a --cn-mirrors
   ```

产物位于 `dist/android/`。签名密钥不入库（`*.keystore`、`key.properties`、`local.properties` 均在 `.gitignore` 中），请务必使用**自己的**签名密钥。

## 法律与版权声明

- **本仓库仅托管源码文本与构建脚本，不包含、不分发任何视频内容、预编译库或安装包（APK）**；不为任何第三方分发渠道的产物负责。
- 站源目录由第三方短剧网站提供，本仓库仅收录直接访问这些公开站点的技术与源码；所有内容的著作权归各自权利人所有，本仓库与视频内容无关。
- 本仓库源码基于开源项目 **Topkill/guoapp3** 修改，**版权归原作者所有**，遵循原项目仓库的许可协议。若原作者或权利人认为本仓库内容不妥，请提 Issue 或联系删除，收到后将及时处理。
- 本项目仅供**个人学习、研究与技术开发交流**使用，禁止用于任何商业用途；编译、安装及对站源内容的任何使用行为均由使用者自行承担，请遵守您所在地区的法律法规及平台协议。
- 本仓库按“现状”提供，不提供任何明示或默示的保证；使用者需自行承担使用风险。

## 致谢

- [Topkill](https://github.com/Topkill) —— guoapp3 原作者
- [kuoluoss](https://github.com/kuoluoss)（软萌小正太）—— zhenguojian 上游维护者
- 所有上游依赖的开源项目与库

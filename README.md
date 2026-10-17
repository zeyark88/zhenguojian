# 真果鉴 · 个人优化构建（基于上游 0.6.11+59）

> 本仓库为**个人自用**的定制构建分支，**不是官方发布仓库**。
> 仅供个人学习、研究与技术交流使用。

## 原项目与出处声明

- **原始项目**：[Topkill/guoapp3](https://github.com/Topkill/guoapp3) —— 原作者 **Topkill**，版权归原作者所有
- **上游源码**：[kuoluoss/zhenguojian](https://github.com/kuoluoss/zhenguojian)（基于 Topkill/guoapp3 构建与维护的发布仓库，维护者 **软萌小正太**）
- 本仓库基于上游 **v0.6.11+59**（main 分支 `3d9f8df`，2026-10-17）源码进行修改构建
- 原上游 README 已完整保留于 [docs/UPSTREAM-README.md](docs/UPSTREAM-README.md)，其中包含上游完整的版本历史说明
- 感谢原作者 Topkill 与上游维护者软萌小正太的付出

## 本仓库相对上游的修改

功能裁剪：

1. **移除应用内更新模块** —— 删除启动检查更新、关于页“检测新版本”入口、APK 下载安装逻辑，同时移除 `REQUEST_INSTALL_PACKAGES` 权限与对应 FileProvider。升级需从本仓库 Release 重新下载覆盖安装。
2. **移除关于页应用内署名** —— 上游在“关于”对话框中署名原作者与维护者；本构建移除了该应用内条目（原作者与出处信息改由本 README 声明，见上文）。
3. **双应用共存** —— 提供“红果鉴”（仅红果站源）独立包名版 `com.duanju.duanju_app.hongguo`，与“真果鉴”（全站源，原包名 `com.duanju.duanju_app`）可同时安装、数据互不干扰。

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
12. **多源目录并发提升** —— Go 全局目录并发 3 → 6、同源请求间隔 250ms → 150ms（均在原设置项允许范围内），Dart 目录 worker 并发 2 → 6；多站源全选时首页等待显著缩短。

构建修正：

13. **锁文件保持上游原版** —— 全程使用 `--enforce-lockfile` 由构建脚本镜像转换，杜绝依赖漂移（曾因手动 `pub get` 导致 `shared_preferences_android` 升到 2.5.0，与混淆构建冲突引发启动崩溃，已修复并回归锁定 2.4.28）。
14. **签名方案**：Release 产物为 `v1 + v2 + v3` 完整签名（`-compat` 后缀版本），提升各类安装器（如鸿蒙卓易通）的兼容性。

## 下载

前往 [Releases](../../releases) 页面下载：

| 文件 | 站源 | 包名 | 架构 |
|---|---|---|---|
| `zhenguojian-0.6.11+59-arm64-v8a-final-compat.apk` | 全站源（真果鉴） | `com.duanju.duanju_app` | arm64-v8a |
| `hongguojian-0.6.11+59-arm64-v8a-final-compat.apk` | 仅红果（红果鉴） | `com.duanju.duanju_app.hongguo` | arm64-v8a |

两包使用同一签名密钥，可同时安装、互不覆盖。附带 `SHA256SUMS` 校验文件。

## 从源码构建

克隆后需先用 Go 工具链编译原生核心生成 `libduanju_core.so`（构建器会自动完成），要求：

- Flutter（与上游一致）+ Go + JDK 17 + Android SDK
- **必须在纯英文路径下执行**（中文路径会导致构建失败）

```bash
# 真果鉴（全站源）
py scripts/build_android.py --abi arm64-v8a --all-sources --cn-mirrors

# 红果鉴（仅红果，改用独立包名共存需调整 applicationId）
py scripts/build_android.py --abi arm64-v8a --cn-mirrors
```

签名密钥不入库（`*.keystore`、`key.properties`、`local.properties` 均在 `.gitignore` 中），自行构建请使用自己的签名密钥。

## 法律与版权声明

- 本仓库**不持有、不传播任何视频、音频、图片等内容**。站源目录由第三方短剧网站提供，本仓库仅收录直接访问这些公开站点的技术与源码，所有内容的著作权归各自权利人所有。
- 本仓库源码基于开源项目 Topkill/guoapp3 修改，**版权归原作者所有**，遵循原项目仓库的许可协议。若原作者或权利人认为本仓库内容不妥，请提 Issue 或联系删除，收到后将及时处理。
- 本项目仅供**个人学习、研究与技术开发交流**使用，禁止用于任何商业用途；对站源内容的任何使用行为由使用者自行承担，请遵守您所在地区的法律法规及平台协议。
- 本仓库按“现状”提供，不提供任何明示或默示的保证；使用者需自行承担使用风险。
- 通过本仓库 Release 提供的 APK 为本人在本机从公开源码构建的产物，未做任何闭源改动。

## 致谢

- [Topkill](https://github.com/Topkill) —— guoapp3 原作者
- [kuoluoss](https://github.com/kuoluoss)（软萌小正太）—— zhenguojian 上游维护者
- 所有上游依赖的开源项目与库

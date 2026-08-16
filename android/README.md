# 一念通流 Android App 2.3.3

`android/` 是完全本地运行的原生 Kotlin + Jetpack Compose Chat 客户端。应用包名为 `com.tokenflow.chat`，当前版本为 `2.3.3`（`versionCode 8`），支持 Android 8.0（API 26）及以上版本。

App 不需要 TokenFlow 账号，不访问 `/mobile/v1`，也不依赖 `TOKENFLOW_BASE_URL`。用户在设备上配置模型供应商后，App 直接调用 OpenAI Chat Completions、OpenAI Responses 或 Anthropic Messages API。Go 后端和移动接口继续保留，供网页端及旧 APK 使用。

## 当前功能

- 支持三种供应商协议、连接测试、远端模型列表和手动添加模型；Base URL 必须是 HTTPS API 根地址。
- 支持多轮流式回复、思考和工具过程、停止、重试最后一条回复、自动标题，以及每个会话独立的生成协程。
- 会话支持搜索、重命名、分支、置顶、归档、恢复、多选和批量删除。分支会复制截至目标助手回复的消息和附件，并使用独立 UUID。
- 本地工作区包含收藏、笔记、智能体和知识库。收藏和笔记支持搜索、多选、全选及批量删除；同一回复只能保存为一条笔记，笔记标题可由来源会话模型总结。
- 笔记正文默认使用聊天同款 Markdown/GFM 和安全内联 HTML 阅读器，进入编辑模式后才显示原文。
- 内置 12 种系统提示词模板。智能体可保存模型、系统提示词、思考强度、工具上限及搜索、URL 读取和知识库开关。
- 知识库支持 TXT、Markdown、JSON、CSV 和 PDF，文件保存在应用私有目录，并使用本地 FTS 检索；可手动把片段附加到下一条消息。
- 每条消息最多附加 5 个图片或文档。附件入口支持系统相机、图片选择器和文件选择器；相机照片会修正 EXIF 方向、限制最长边 4096 px，并固定转为 JPEG 质量 75。
- 支持模型视觉检测和全局视觉兜底。当前模型不能直接读取图片时，由已通过视觉测试的兜底模型先生成描述，再交给原会话模型回复。
- Exa Key 用于联网搜索。URL 读取可选择内置读取器或无需 Key 的公开 InfoFlow 接口；目标 URL 会先经过公共 HTTPS/443、DNS 和私网地址校验，内置读取器还会逐次校验重定向，InfoFlow 失败时可回退内置读取器。
- 助手正文可以选择和复制；支持 CommonMark、GFM 表格、安全内联 HTML、代码复制和常见语言的深浅色语法高亮。
- 支持简体中文/英文、系统深浅色、手机抽屉、平板常驻导航和超宽屏三栏布局；头像始终按正方形中心裁剪，Chat 字体大小可用滑块调整。
- Token 用量只显示总数，超过 1000 时使用紧凑的 `K` 格式，例如 `1.2K`。

## MiMo 语音播放

在全局设置中填写 MiMo Key 并选择音色后，可以从已完成的助手回复生成语音。请求固定使用 `mimo-v2.5-tts`，生成的 WAV 只写入应用缓存，不写入 Room；缓存被系统清理后可以再次生成。

语音由 Media3 ExoPlayer 播放，并管理 Android Audio Focus。生成成功后只会在发起生成的当前会话页面实例中自动播放；离开 Chat、进入笔记或收藏，再返回时不会消费或重放旧的自动播放事件。回复下方只保留播放/暂停状态条，不提供“重新生成语音”按钮。

如果模拟器中状态显示正在播放但没有声音，请同时检查 Android 媒体音量、模拟器输出设备以及 Windows 宿主是否存在默认音频输出端点。

## 环境

- JDK 17
- Android SDK Platform 36
- Android Build Tools 36.0.0
- Gradle Wrapper 9.4.1
- Android Gradle Plugin 9.2.1
- Compose BOM 2026.06.01

SDK 不在默认目录时，在不提交的 `android/local.properties` 中设置：

```properties
sdk.dir=C\:\\Users\\your-name\\AppData\\Local\\Android\\Sdk
```

## Debug 构建

```powershell
cd android
.\gradlew.bat testDebugUnitTest lintDebug assembleDebug assembleDebugAndroidTest
```

Debug APK 位于 `app/build/outputs/apk/debug/app-debug.apk`，测试 APK 位于 `app/build/outputs/apk/androidTest/debug/app-debug-androidTest.apk`。Debug 应用 ID 为 `com.tokenflow.chat.debug`，可以和正式版同时安装。

构建不接受也不需要 `TOKENFLOW_BASE_URL`。Manifest 禁止明文网络；供应商 Base URL 必须是无 userinfo、query 或 fragment 的 HTTPS API 根地址，例如 `https://api.openai.com/v1`。

## 覆盖安装与设备测试

需要保留本地 Room 数据时，只能覆盖安装，不要卸载应用或清除应用数据：

```powershell
adb connect 127.0.0.1:5557
adb -s 127.0.0.1:5557 install -r -t app\build\outputs\apk\debug\app-debug.apk
```

普通 USB 设备或标准模拟器可以运行：

```powershell
.\gradlew.bat connectedDebugAndroidTest
```

Windows 上使用 `127.0.0.1:5557` 这类 TCP ADB 序列号时，Gradle UTP 可能因序列号中的冒号创建目录失败。此时安装两个 APK 后直接运行 instrumentation：

```powershell
adb -s 127.0.0.1:5557 install -r -t app\build\outputs\apk\debug\app-debug.apk
adb -s 127.0.0.1:5557 install -r -t app\build\outputs\apk\androidTest\debug\app-debug-androidTest.apk
adb -s 127.0.0.1:5557 shell am instrument -w com.tokenflow.chat.debug.test/androidx.test.runner.AndroidJUnitRunner
```

设备验收至少覆盖 API 26 手机、API 36 手机和一个平板尺寸，并检查三种协议的连续多轮对话、软键盘、横竖屏、长 Markdown/表格、代码高亮、深浅主题、流式停止、相机附件、头像裁剪和会话多选。语音回归需要覆盖“生成语音 -> 打开笔记或收藏 -> 返回 Chat”，确认不会自动重放。

## Release 构建与签名

Release 必须继续使用已发布版本的同一份外部 Android keystore，才能覆盖升级：

```powershell
$env:TOKENFLOW_KEYSTORE_PATH = "C:\secure\tokenflow-release.jks"
$env:TOKENFLOW_KEYSTORE_PASSWORD = "your-store-password"
$env:TOKENFLOW_KEY_ALIAS = "tokenflow"
$env:TOKENFLOW_KEY_PASSWORD = "your-key-password"
.\gradlew.bat assembleRelease
```

也可以使用同名 `-P` Gradle 参数。签名值缺失或 keystore 无效时，Release 构建会失败。正式 APK 位于 `app/build/outputs/apk/release/app-release.apk`。

`local.properties`、`*.jks` 和 `*.keystore` 已由根目录 `.gitignore` 排除。密码不得写入 Gradle 文件或提交到仓库。

## 数据与密钥

- Room 当前 schema 版本为 v4，显式注册 `1->2`、`2->3` 和 `3->4` Migration，不使用 destructive fallback。
- 供应商、模型、会话、消息、附件索引、收藏、笔记、智能体和知识索引位于应用沙箱，并禁止系统备份。生成中的消息在进程重启后会标记为已中断。
- 供应商 API Key、Exa Key 和 MiMo Key 使用 Android Keystore AES-256-GCM 加密保存。2.0 升级时会清除旧移动 Bearer Token；旧 InfoFlow Key 只为导入兼容保留，不再使用、导出或应用。
- 本地头像按全局和会话 UUID 隔离保存，并统一中心裁剪为 384x384 PNG。
- 聊天附件和知识文件保存在应用私有目录；TTS WAV 位于缓存目录。它们都不会进入配置导出。
- `.tfcfg` 使用 PBKDF2-HMAC-SHA256（600,000 次、16 字节随机 salt）派生密钥，再以 AES-256-GCM 和 12 字节随机 IV 加密；密码至少 10 位且不会保存。
- 导出包含供应商及其密钥、已添加模型与视觉状态、默认模型、视觉兜底、Exa Key、MiMo Key/音色、全局系统提示词、URL 读取器选择和智能体。
- 导出不包含会话、消息、附件、收藏、笔记、知识文件、头像、Chat 字体或其他界面偏好。导入会先显示新增、更新和冲突摘要，确认后按稳定 UUID 合并。

## 完整验证

从仓库根目录执行服务端和网页测试：

```powershell
go test ./...
npm test
```

然后执行 Android 验证：

```powershell
cd android
.\gradlew.bat testDebugUnitTest lintDebug assembleDebug assembleDebugAndroidTest
```

设备测试使用上一节的 `connectedDebugAndroidTest` 或直接 instrumentation 命令。覆盖安装前后可用以下命令确认安装时间和版本，避免误清数据：

```powershell
adb -s 127.0.0.1:5557 shell dumpsys package com.tokenflow.chat.debug | Select-String 'versionName|versionCode|firstInstallTime|lastUpdateTime'
```

## 实现结构

- `data/ApiClient.kt`：供应商校验、标准认证、三协议请求、多轮历史和 SSE 事件归一化。
- `data/DirectChatEngine.kt`：无增量重试、思考参数降级和统一本地工具循环。
- `data/LocalDatabase.kt`：Room v4 实体、级联关系、严格 Migration 和中断恢复。
- `data/ChatRepository.kt`：本地配置、会话/分支、消息、收藏、笔记标题、智能体、视觉兜底及加密导入导出。
- `data/AttachmentStore.kt`、`data/CameraCaptureStore.kt`：附件提取、图片归一化、相机 JPEG 75 草稿和私有文件生命周期。
- `data/KnowledgeStore.kt`：本地文件复制、PDF/文本提取、分块和 FTS 搜索。
- `data/SecretStore.kt`：Android Keystore AES-GCM 密钥存储。
- `data/WebTools.kt`：Exa 搜索、公开 InfoFlow/内置 URL 读取和私网地址防护。
- `data/MimoTtsClient.kt`：MiMo WAV 生成与缓存。
- `ui/AppViewModel.kt`：每会话独立生成协程、工作区状态和仅消费一次的定向语音自动播放事件。
- `ui/ChatApp.kt`、`ui/WorkspaceScreens.kt`：青玉自适应 App Shell、Chat 和本地工作区页面。
- `ui/SpeechPlaybackController.kt`：Media3 ExoPlayer、Audio Focus 和播放状态。
- `ui/MarkdownRenderer.kt`：Markdown/GFM、安全内联 HTML、可选择文本、代码复制和语法高亮。

## 服务端部署凭据

原生 Android App 本身不需要部署到 TokenFlow 服务器。维护现有 Go 后端/PWA 时，部署文档约定的 SSH 私钥路径为 `~/.ssh/LotusSSL`；Windows 上通常展开为 `C:\Users\<用户名>\.ssh\LotusSSL`。该 SSH 私钥不是 Android APK 签名 keystore，不得提交到仓库。

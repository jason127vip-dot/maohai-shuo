# 毛孩儿说开发进度

更新日期：2026-10-30。本次新增 Go 后端，已完成本地演示联调。继续工作前阅读本文件、`backend/README.md` 和 `frontend/README.md`。

## 2026-10-30 后端开发进度

### 已完成

- 新增 `backend/` Go + Gin 项目，按 `cmd`、`configs`、`handler`、`service`、`repository`、`model`、`provider`、`worker`、`storage`、`middleware` 和 `migrations` 拆分。完整目录及职责见 `backend/README.md`。
- 参数集中在 `backend/configs/`：公开模板 `config.example.yaml`、忽略的真实配置 `config.local.yaml`、提示词 `prompts.yaml` 和字体说明。真实配置尚未创建，需要按 README 复制模板填写。
- 实现配置严格读取、启动校验、相对路径解析、开发 Cookie 会话、来源校验及统一 JSON 错误。
- 实现 `/api/health`、`/api/me`、创建任务、查询任务及私有成品图片接口，对齐现有 React。
- 实现内存演示数据层、Neon PostgreSQL 数据层及独立建表 SQL；没有自动连接用户的 Neon，也没有执行建表 SQL。
- 实现额度事务设计、用户级幂等、照片指纹冲突校验、并发限制、明确失败只退款一次，以及内部待核对状态。
- 实现后台任务、豆包 Seedream 适配器及原图 DEMO 适配器；统一模型接口保留后续供应商扩展能力。
- 实现后端图片校验、私有原图、方形成品及角标、原图立即删除与过期清理。真实模型模式要求配置含中文字形的字体。
- 前端做了一处必要修复：任务已明确失败后返回制作页，会开始新的提交编号，避免重试一直回到旧失败任务。
- 保持本地开发模式：正式微信身份尚未实现，程序拒绝以 production 身份模式启动；对象存储、公开分享缩略图及管理后台仍待开发。

### 验证结果

| 检查 | 结果 |
| --- | --- |
| `go test -race ./...` | 通过：配置、会话、并发幂等、退款、待核对、图片、worker 和本地 HTTP/假豆包接口 |
| `go vet ./...` | 通过 |
| 实际后端 `go build -mod=readonly` | 通过 |
| 前端 `npm run lint` / `npm run build` | 通过 |
| React 生产构建 → 实际 Go 演示服务 | 通过：签名会话、上传、后台任务、图片查询、1024 方形 JPEG、原图删除 |
| 刷新结果页 | Go 任务与会话仍有效，可恢复显示 |
| 新制作与额度 | 2→1→0，额度耗尽后 UI 禁用且直接 API 请求返回拒绝 |
| 图片归属 | 另一浏览器会话无法读取成品 |
| 本地联调控制台和外部请求 | 无未捕获页面错误，无外部模型请求 |
| 真实 Neon 事务 / 豆包出图 / 微信真机 | 尚未执行；不视为通过 |

联调使用自建测试图片、内存数据库和 DEMO 适配器，没有模型费用。临时测试使用 58080/58173 端口，测试后已停止并确认无监听；没有替用户启动默认 8080/5173 服务，也没有停止用户自己的进程。

### 下一步运行

后端终端：

```bash
cd /Users/jason/Documents/Work/workspace/MaoHaishuo/backend
go mod download
test -f configs/config.local.yaml || cp configs/config.example.yaml configs/config.local.yaml
go run ./cmd/server -config configs/config.local.yaml
```

前端另一个终端：

```bash
cd /Users/jason/Documents/Work/workspace/MaoHaishuo/frontend
nvm use
VITE_USE_MOCK=false npm run dev
```

前端必须使用 false，才会访问 Go。后端模板仍是 `ai.provider: demo` 和 `database.driver: memory`，所以此时只是后端本地演示。配置真实 Neon、模型 ID、API Key 及中文字体后，才进入真实服务联调。配置方法、数据库表和目录结构均写在 `backend/README.md`。

本轮修改未提交或推送 Git；新增真实配置、运行图片和字体文件的忽略规则已经验证。

## 2026-10-29 前端恢复记录（历史）

- 已在实际 `frontend/` 安装锁定依赖，并启动本地模拟预览 `http://127.0.0.1:5173`。服务器只监听本机，需要终端进程保持运行；关闭后按下方说明重启。
- 手机端缩短介绍区，使上传入口在首屏内可见；场景改为两列，增大辅助文字与点击区域。390px 宽度下上传区顶部由约 805px 提前至 483px。
- 新增 `CreationDraftProvider` 与独立的 Context/Hook，阅读隐私后保留照片、场景、画风及同意状态；草稿仍只在内存，不持久化照片。
- 相同请求重试继续使用原幂等键；修改输入或明确“再做一张”时开始新提交。整理照片时暂时禁用生成，避免误用上一张照片。
- 页面导航滚动到顶部；切换任务时不展示旧任务结果；成功但缺少图片地址时显示可重试错误。
- 将业务 CSS 从全局文件拆到应用外壳、制作页、介绍插画、生成组件、结果页、隐私页、按钮和弹窗附近。
- 没有接入付费模型，没有连接 Neon，没有修改 `backend/` 或 `docs/`，没有 Git 提交或推送。

### 本次检查结果

| 检查 | 结果 |
| --- | --- |
| 实际项目 `npm run lint` | 通过 |
| 实际项目 `npm run build` | 通过，含 TypeScript 检查 |
| Chrome 布局：320 / 375 / 390 / 768 / 1440px | 通过，无横向溢出 |
| 图片类型、10 MB 上限、损坏图片、过小图片、压缩与移除 | 通过 |
| 隐私页往返、选项及照片保留 | 通过 |
| 等待、结果、大图、Escape 关闭、关闭按钮、浏览器下载 | 通过 |
| 重复提交、第二次制作、额度用完 | 通过 |
| 无效或刷新失效的模拟任务 | 正确显示可重试错误 |
| 模拟模式网络检查 | 无后端调用、照片上传、外部模型请求或未捕获页面错误 |
| 生产构建加本地模拟 Go HTTP 服务 | 额度、提交、查询错误和重试均通过；幂等键稳定；失败停止轮询；缺图及任务切换处理通过 |
| 微信真机、真实 Go / Neon / 模型联调 | 尚未执行 |

浏览器检查使用已有 Chrome 与临时测试图片；生产构建的接口检查使用本地测试服务，不能视为真实后端或供应商验证。检查脚本和截图位于临时目录，代码及本进度文件已保存到实际项目。

## 项目位置和技术选择

- 本地项目：`/Users/jason/Documents/Work/workspace/MaoHaishuo`
- GitHub：`https://github.com/jason127vip-dot/maohai-shuo.git`
- 分支：`main`。仓库已有提交，本轮新增后端代码尚未提交或推送。
- 前端：React + TypeScript + Vite，使用 React Router。
- 后端：Go + Gin，已实现本地演示及 Neon PostgreSQL 数据层；真实 Neon 连接待配置验证。
- 首个图片模型计划：豆包 Seedream；后端应保留即梦等其他平台的适配能力。
- 代码位于 `frontend/` 和 `backend/`；`docs/` 未修改。

## 已保存内容

`frontend/` 包含应用源代码、SVG 插画、构建与 ESLint 配置、`package.json`、`package-lock.json`、`.nvmrc`、`.env.example`、`.gitignore` 和 README。依赖目录 `node_modules/` 和构建产物 `dist/` 不属于源代码。本次已在实际项目安装和构建；换电脑或清理依赖后按运行说明重装。

项目原有文件保持原样，包括根目录的 `.DS_Store`；保存时对目标文件进行检查，不覆盖不同内容的已有文件。

### 制作页 `/`

- 暖白与橙色的页面设计，包含桌面双栏和手机布局。
- 产品介绍、宠物 SVG 插画示意、剩余免费额度展示。
- 本地选择照片、预览、更换和移除；支持拖拽选择。
- 支持 JPEG、PNG、WebP，文件上限 10 MB；校验图片能否解码及尺寸。
- 在浏览器内压缩至最长边不超过 2048 像素，转换为 JPEG。
- 四种场景：拆家、讨吃、睡姿、门口等你。
- 三种画风：素描、卡通、水彩。
- 隐私同意勾选、必填条件控制、提交中按钮禁用、防重复点击、提交错误提示。

### 任务与结果页 `/generations/:taskId`

- 生成等待界面、任务查询、成功和失败状态。
- 任务每 1.5 秒查询一次，离开页面或成功、失败后停止。
- 等待超过 3 分钟暂停自动查询，提供手动继续查询入口。
- 图片展示、大图弹窗、长按保存说明和浏览器下载入口。
- 当前模拟结果是原图加演示标识，明确提示不是 AI 生成成品。

### 隐私页 `/privacy`

- 说明本地演示中的照片处理和内存保存方式。
- 说明照片授权及插画示意的用途。
- 正式运营隐私指引尚未完成，不能直接据此上线。

### 代码结构与接口边界

- `src/app`：应用外壳及路由。
- `src/pages`：制作、结果、隐私页面。
- `src/features/generation`：场景和画风配置、类型、接口、生成查询 Hook、业务组件。
- `src/features/user`：用户额度读取及 Hook。
- `src/components`：通用按钮、弹窗、SVG 插画。
- `src/lib`：HTTP 请求封装、图片处理、公开配置。
- `src/mocks`：本地模拟接口。
- `src/styles`：设计变量与全局基础样式；业务样式现已拆分至对应页面或组件附近。

前端只调用我们自己的业务 API，不直接引用任何模型供应商的请求格式和密钥。多模型选择与适配留在 Go 后端。

## 当前模拟行为和限制

- 仅当开发模式下配置 `VITE_USE_MOCK=true`，才使用本地模拟。
- 模拟照片、额度和任务只在浏览器内存中保存，刷新后重置；不上传照片，不调用 AI，不产生模型费用。
- 免费额度初始为 2，模拟提交后减少。它只是交互演示，不是真实额度校验。
- 卡片中的 SVG 仅是界面插画，不是实际模型出图样张。
- “再做一张”是回到制作页，不代表已实现免费重做规则。
- 生产构建始终调用真实 Go API，不能将模拟开关作为正式上线方案。
- 当前未接通 Go、Neon、微信 OAuth、JSSDK、真实模型或对象存储。

## 上次暂停时的检查（历史）

检查在临时验证副本 `/private/tmp/maohai-frontend-stage` 中完成，使用现有 Node.js v24.19.0 运行时。

| 检查 | 结果 |
| --- | --- |
| 安装 npm 依赖并生成锁文件 | 通过 |
| `npm run build`：TypeScript 检查与 Vite 生产构建 | 通过，Vite v7.3.7 |
| `npm run lint` | 通过 |
| 启动开发服务器 | 未启动成功，沙箱报 `listen EPERM: operation not permitted 127.0.0.1:5173` |
| 浏览器操作与截图检查 | 未执行，不视为通过 |
| 微信真机测试 | 未执行 |
| 真实后端、数据库、模型联调 | 未执行 |

依赖安装时 npm 提示 ESLint 9 已不在支持期。它不影响本次 lint 通过；恢复工作后评估升级与插件兼容，暂未额外修改依赖方案。

上次暂停时没有成功运行的开发服务器。本次已启动本地开发服务器，详见顶部恢复记录；仍没有真实模型任务或数据库写入。

## 运行方式

本机默认终端原先为 Node 16，不能运行本项目 Vite。先准备 Node.js 22.12 或更高版本。项目 `.nvmrc` 指向 Node 22；如已安装 nvm：

```bash
cd /Users/jason/Documents/Work/workspace/MaoHaishuo/frontend
nvm install
nvm use
npm ci
```

首次创建本地模拟配置时执行以下命令；如果 `.env.local` 已经存在，保留其原内容，不重复覆盖：

```bash
test -f .env.local || cp .env.example .env.local
npm run dev
```

默认浏览器地址通常为 `http://127.0.0.1:5173`，以终端实际输出为准。`npm run dev` 需要在允许监听本地端口的环境运行。

检查命令：

```bash
npm run lint
npm run build
npm run preview
```

注意：`npm run preview` 预览的是生产构建，会请求真实 Go API，而非模拟接口。当前体验模拟流程请使用 `npm run dev`。

## 拟定 Go 接口

- `GET /api/me`：返回 `{ "remainingCredits": 2 }`。
- `POST /api/generations`：multipart 字段 `photo`、`scene`、`style`；`Idempotency-Key` 请求头用于幂等。
- `GET /api/generations/:taskId`：返回 `id`、`scene`、`style`、`status`，成功包含 `imageUrl`，失败包含 `error`。
- 状态：`queued`、`processing`、`succeeded`、`failed`。
- 错误：非 2xx 状态码与 `{ "code": "...", "message": "..." }`。
- 开发代理指向 `http://127.0.0.1:8080`，详见 `vite.config.ts` 和 README。
- 身份、权限、额度扣减、幂等都由后端校验，不能信任浏览器提交的数据。

## 未完成任务

1. 配置专用 Neon，审阅并在目标库执行迁移，验证真实事务、断连、恢复及连接池行为。
2. 配置豆包 API Key、Model ID 和已授权中文字体，用少量真实宠物照片验证保真度、耗时、失败和计费。
3. 微信 OpenID 身份、网页授权、JSSDK、真机保存与分享。当前开发 Cookie 不能用于正式运营。
4. 生产对象存储、公开分享缩略图、原图及成品删除管理、正式隐私指引和 AI 标识核对。
5. 明确免费重做规则；实现待核对任务的管理/结算、额度调整、白名单和数据导出。
6. 评估前端 ESLint 9 升级；当前 lint 通过。

## 恢复工作的下一步

**先按后端 README 运行本地前后端，再配置专用 Neon 和豆包做真实服务联调。** 不需要重新初始化项目，不要使用脚手架覆盖 `frontend/`。保持 React + Go + Neon 的已定方向，以及多模型适配在后端的边界。

本轮新增后端与配套前端修复尚未提交或推送。提交前检查 `git status`，确认不包含真实配置、密钥、运行图片、字体、`node_modules`、`dist` 和 `.DS_Store`；本次没有更改 Git 历史。

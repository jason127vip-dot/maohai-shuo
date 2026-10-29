# 毛孩儿说 H5 前端

React + TypeScript + Vite。当前完成的是制作页，以及用于联调的生成等待页、结果预览页、隐私说明页。

## 本地启动

需要 Node.js 22.12 或更高版本。项目包含 `.nvmrc`，使用 nvm 时可先执行 `nvm install` 和 `nvm use`。Node 16 无法运行此版本 Vite。

```bash
cd frontend
npm install
cp .env.example .env.local
npm run dev
```

打开终端显示的本地地址，通常为 `http://127.0.0.1:5173`。没有启用模拟配置时，前端会尝试调用 Go 后端。

```bash
npm run lint
npm run build
npm run preview
```

生产构建输出在 `dist/`。`preview` 仅用于检查构建结果，不作为生产服务器。

## 已完成的页面

| 地址 | 功能 |
| --- | --- |
| `/` | 上传与更换照片、四个场景、三种画风、额度、隐私同意、提交 |
| `/generations/:taskId` | 查询任务、等待、失败提示、成品预览、大图与保存引导 |
| `/privacy` | 当前预览模式的照片处理说明；正式隐私指引仍需补全 |

支持 JPEG、PNG、WebP，原始文件上限 10 MB。照片在浏览器端解码、缩放至最长边 2048 像素、转换为 JPEG，点击生成后才提交。

## 目录怎么找

```text
src/
  app/                       应用外壳、路由
  pages/create/              制作页、左侧插画介绍
  pages/result/              等待与成品预览
  pages/privacy/             隐私说明
  features/generation/       生成接口、类型、任务查询、场景画风组件
  features/user/             当前用户与免费额度
  components/                通用按钮、弹窗、SVG 插画
  lib/                       HTTP 请求、图片处理、公开配置
  mocks/                     与真实 API 同一接口的本地模拟
  styles/                    全局基础样式、颜色及字号变量
```

开发约定：页面组织功能，业务逻辑放在对应 `features`；网络请求统一经过 `lib/request.ts`；业务组件样式与业务文件放在一起。`App.css` 管应用外壳，`Showcase.css` 管介绍插画，`CreateForm.css` 管制作表单，`generation.css` 管选图及场景画风组件，其余结果页、隐私页、按钮和弹窗各自维护 CSS。全局颜色优先从 `styles/tokens.css` 调整。

## 模拟模式与实际能力

`.env.local` 中的 `VITE_USE_MOCK=true` 仅在 `npm run dev` 下生效。生产构建无论此变量为何值都使用真实 API，页面也不会把接口故障伪装成生成成功。

模拟模式只在浏览器内工作，不上传照片，不调用 AI，也不产生模型费用。模拟结果是上传原图加预览标识，不代表真实画风转换效果。插画卡片为 SVG 界面示意，也不是模型效果样张。

模拟额度初始为 2 次，任务和额度只保留在内存，刷新后重置。真实环境的额度、幂等、任务状态和授权均由 Go 后端管理。

当前制作草稿由 `CreationDraftProvider` 在内存中管理，进入隐私页再返回、从结果页返回时，照片和选项不会丢失。照片不会写入 localStorage 或 sessionStorage；更换或移除照片、关闭页面时释放旧预览地址。相同提交重试沿用幂等键；用户修改照片/场景/画风或在结果页点击“再做一张”时，开始新的提交。

当前保存方式为大图长按保存，并提供普通浏览器下载入口。微信 JSSDK 分享、OAuth、免费重做规则尚未接通；不会显示假的分享成功或把普通再次制作标为免费重做。

## Go API 对接约定

开发环境下 `/api` 代理至 `http://127.0.0.1:8080`。设置 `VITE_USE_MOCK=false` 后可开始联调。以下为拟定接口，Go 尚未实现：

### 当前额度

`GET /api/me`

```json
{ "remainingCredits": 2 }
```

身份由后端通过微信授权与会话确定，前端不传 OpenID 作为可信身份。

### 创建图片任务

`POST /api/generations`

- Body：`multipart/form-data`，字段为 `photo`、`scene`、`style`。
- Header：`Idempotency-Key`，同一次提交重试时保持一致。后端应据此防止重复创建任务和扣额度。
- 场景值：`mischief`、`snack`、`sleep`、`door`。
- 画风值：`sketch`、`cartoon`、`watercolor`。

```json
{
  "id": "task-id",
  "status": "queued",
  "scene": "mischief",
  "style": "cartoon"
}
```

### 查询图片任务

`GET /api/generations/:taskId`

状态为 `queued`、`processing`、`succeeded`、`failed`。成功返回 `imageUrl`，失败返回 `error`。查询接口必须验证任务属于当前用户。

```json
{
  "id": "task-id",
  "status": "succeeded",
  "scene": "mischief",
  "style": "cartoon",
  "imageUrl": "https://your-image-host/result.jpg"
}
```

前端每 1.5 秒查询一次；离开页面或成功、失败后停止。连续等待 3 分钟后暂停自动查询，允许用户手动继续查，不重新创建任务。真实任务可根据 URL 的编号恢复，模拟任务刷新后不可恢复。

错误使用非 2xx HTTP 状态码及统一结构：

```json
{ "code": "QUOTA_EXHAUSTED", "message": "免费额度已用完" }
```

前端只依赖上述业务接口。豆包、即梦、提示词和模型切换都由 Go 后端适配，不在 React 中维护供应商请求逻辑。

## 部署与后续工作

- Nginx 托管 `dist/`，页面路由配置 `try_files $uri $uri/ /index.html;`，并将 `/api/` 转发到 Go 服务。
- 前后端同域部署，通过 HTTPS Cookie 维持后端会话。
- `VITE_` 环境变量会进入浏览器代码，只放公开配置；不要放模型密钥、微信 AppSecret、Neon 数据库连接串。
- 正式上线前补齐真实授权、任务与额度规则、AI 成品标识、图片保留和删除机制、微信分享配置及运营主体隐私指引。
- 验证微信真机上的选图、保存、分享与跨页面授权。浏览器桌面预览不代表微信真机验收。

## 最近验证

2026-10-29 已在本地 Chrome 检查 320、375、390、768、1440 像素宽度，未发现横向溢出。已通过非法/过大/损坏/过小图片校验、压缩、更换/移除、隐私往返草稿、生成等待及结果、弹窗与下载、重复提交、额度不足的浏览器流程检查。

也使用实际生产构建加本地测试 HTTP 服务验证了 `/me`、创建任务、查询任务的错误重试、提交幂等键、任务切换、成功却缺少图片地址和失败后停止查询。这个测试服务仅模拟 Go 接口，不代表真实 Go、Neon、微信或模型联调通过。完整进度见仓库根目录 `PROGRESS.md`。

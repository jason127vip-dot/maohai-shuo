# 毛孩儿说 Go 后端

后端使用 Go、Gin 和 pgx，负责用户额度、照片校验、图片任务、豆包 Seedream 调用及结果保存。前端只访问自己的业务接口，模型选择与密钥留在 Go 后端。

当前版本可在本机完整运行“React → Go → 后台任务 → 图片预览”。默认使用内存数据和本地演示图片，便于不填任何云服务密钥就先检查流程。Neon 数据层和豆包真实调用代码已实现，真实连接及出图需配置后再联调。

## 目录结构

```text
backend/
├── cmd/
│   └── server/
│       └── main.go                 启动入口与退出处理
├── configs/                        所有运行配置集中在这里
│   ├── config.example.yaml         可提交的配置模板
│   ├── config.local.yaml           自己填写的真实配置，Git 忽略
│   ├── prompts.yaml                场景、画风和提示词
│   └── fonts/
│       └── README.md               中文角标字体说明
├── internal/
│   ├── config/                     读取配置、校验参数、拼接提示词
│   ├── handler/                    接收 HTTP 请求，返回前端格式
│   ├── service/                    照片处理、幂等提交及业务编排
│   ├── repository/                 数据读写与额度事务
│   │   ├── repository.go           统一数据接口
│   │   ├── memory.go               本地演示，重启后清空
│   │   └── postgres.go             Neon PostgreSQL 实现
│   ├── model/                      任务、状态和响应结构
│   ├── provider/
│   │   ├── provider.go             统一图片生成接口
│   │   ├── demo/                   原图演示，无模型费用
│   │   └── doubao/                 豆包 Seedream 实现
│   ├── worker/                     后台执行、超时处理与原图清理
│   ├── storage/                    文件存储、缩放和角标
│   └── middleware/                 开发会话、来源校验、请求日志
├── migrations/
│   └── 001_initial.sql             建表 SQL，手动审阅执行
├── data/                           运行时图片，Git 忽略
├── go.mod
├── go.sum
├── .gitignore
└── README.md
```

`handler` 负责输入输出，`service` 处理业务，`repository` 管数据，`provider` 调模型，`worker` 在后台执行。新增供应商时实现 `provider.Generator` 并在启动入口注册，不改前端和额度事务。

## 本地启动

需要 Go 1.25 或以上。开发机当前检查到 Go 1.27.0。

```bash
cd /Users/jason/Documents/Work/workspace/MaoHaishuo/backend
go mod download
test -f configs/config.local.yaml || cp configs/config.example.yaml configs/config.local.yaml
go run ./cmd/server -config configs/config.local.yaml
```

默认监听 `127.0.0.1:8080`。查看状态：

```bash
curl http://127.0.0.1:8080/api/health
```

Ctrl+C 停止服务。健康接口表示进程正在运行；它不是数据库可用性的持续监控。

联调 React 时，在另一个终端运行：

```bash
cd /Users/jason/Documents/Work/workspace/MaoHaishuo/frontend
nvm use
VITE_USE_MOCK=false npm run dev
```

前端已有 `/api` 到 `127.0.0.1:8080` 的代理。这里必须关闭前端模拟模式，否则页面仍会绕过 Go。后端模板中的 `ai.provider: demo` 与前端模拟是两个独立开关：关闭前端模拟后，可以先访问真正的 Go 服务，同时保持后端不调用付费 AI。

后端演示额度初始为 2。浏览器通过签名 Cookie 保持开发身份；内存数据库随 Go 进程重启清空。演示结果仍是原图加工，标注为 DEMO，前端返回 `isMock: true`。

## 配置集中管理

日常只改 `configs/config.local.yaml` 和同目录的 `prompts.yaml`。文件路径相对配置文件所在目录解析，换工作目录也不会把照片写到意外位置。配置在启动时读取，修改后重启生效。

| 配置组 | 主要内容 |
| --- | --- |
| `server` | 环境、地址、端口、允许的网页来源、超时 |
| `database` | memory/postgres、Neon 连接串、连接池上限 |
| `auth` | 开发 Cookie 名称、签名密钥和有效期 |
| `wechat` | 预留 AppID、AppSecret、回调地址；尚未接入 OAuth |
| `ai` | 当前供应商、生成超时和演示延迟 |
| `ai.providers.doubao` | API 地址、API Key、模型 ID、尺寸和单图设置 |
| `worker` | 并发数、查询间隔、中断任务的判定时间 |
| `storage` | 文件目录、上传及像素限制、成品尺寸、字体、原图保留时间 |
| `quota` | 初始额度、每个用户同时生成的任务上限 |
| `prompts_file` | 提示词文件位置 |

真实密钥仅放在本地配置文件。`config.local.yaml` 被本目录 `.gitignore` 忽略；不要把填好密钥的内容复制回 `config.example.yaml`。本版运行配置没有分散到多个 `.env` 文件，也不会打印配置全文、照片、提示词或模型响应。

### 调整每人免费次数

在 `configs/config.local.yaml` 中修改下面的参数，默认每人 2 次，需要 5 次或 10 次时直接修改数字：

```yaml
quota:
  initial_credits: 2 # 可改为 5 或 10
  max_active_tasks_per_user: 1
```

保存后重启 Go 服务。该参数用于新用户首次初始化，已有用户的剩余次数不会被重置；使用 `memory` 演示数据库时，重启会清空内存用户，重新按新参数初始化。前端需使用 `VITE_USE_MOCK=false`，才能读取 Go 返回的额度。

## 接入 Neon

1. 新建毛孩儿说专用 Neon 项目或数据库。
2. 审阅 `migrations/001_initial.sql`，在目标数据库执行一次。
3. 将 `database.driver` 改为 `postgres`，`database.url` 填 Neon 的 PostgreSQL 连接串，保留 SSL 参数。
4. 重启 Go 服务。

启动只检查连接和表是否存在，不自动执行迁移。表名统一带 `maohai_` 前缀：

| 表 | 用途 |
| --- | --- |
| `maohai_users` | 当前额度与用户标识 |
| `maohai_generation_tasks` | 任务、幂等键、照片指纹、场景、画风、提示词快照和状态 |
| `maohai_quota_ledger` | 预占、退款流水，防止重复处理 |
| `maohai_generation_attempts` | 实际供应商、模型、请求编号、返回张数和耗时 |

额度扣减、任务创建和预占流水放在同一个事务中。额度流水用于核对业务次数，不能直接视为供应商账单；真实费用仍需与供应商计费核对。

## 切换真实豆包图片生成

在本地配置中填写：

```yaml
ai:
  provider: doubao
  timeout_seconds: 120
  demo_delay_milliseconds: 1500
  providers:
    doubao:
      base_url: https://ark.cn-beijing.volces.com/api/v3
      api_key: "填写你自己的 API Key"
      model: "填写控制台已开通的 Seedream 模型 ID"
      size: 2K
      disable_group_generation: true
```

这是配置局部示例，保留模板中的其他配置组。`disable_group_generation: true` 适用于官方支持该参数的 Seedream 5.0 lite / 4.5 / 4.0；使用其他型号时先核对接口能力，可能需要设为 false。

还需要在 `storage.font_path` 配置包含“毛孩儿说”和“AI 生成”的中文 TTF/OTF 字体。字体使用方法见 `configs/fonts/README.md`。程序在接收生成任务前检查字体，真实模型模式不会使用缺少中文字形的字体替代。

豆包请求使用参考照片 Base64、后端提示词、单张输出、`b64_json` 返回和 AI 水印。图片返回后，由本服务转换为方形成品并添加品牌及 AI 角标。本服务不依赖供应商会过期的临时图片 URL。

配置真实豆包后，点击生成会实际发起付费请求。当前开发身份只适合本机测试，真实账户额度及价格以你开通的服务为准。

参考：[豆包图片接口](https://console.volcengine.com/ark/region%3Acn-beijing/docs/image-generation-api)、[Neon Go 连接](https://neon.com/docs/guides/go)。

## 接口约定

| 方法和路径 | 作用 |
| --- | --- |
| `GET /api/health` | 服务进程状态，不需要会话 |
| `GET /api/me` | 当前用户剩余额度，建立开发 Cookie |
| `POST /api/generations` | 上传照片并创建后台任务 |
| `GET /api/generations/:taskId` | 查询自己的任务 |
| `GET /api/images/:taskId` | 读取自己的成品图片 |

提交使用 `multipart/form-data`，字段为 `photo`、`scene`、`style`，并带 `Idempotency-Key` 请求头。场景与画风 ID 与 React 已有值一致。

首次创建返回 HTTP 202；相同提交返回 HTTP 200 及原任务。前端查询使用 `queued`、`processing`、`succeeded`、`failed` 四种状态。响应不包含提示词、数据库地址或密钥。

```json
{
  "id": "随机任务编号",
  "status": "succeeded",
  "scene": "mischief",
  "style": "cartoon",
  "imageUrl": "/api/images/随机任务编号"
}
```

错误使用非 2xx 状态及统一结构：

```json
{ "code": "QUOTA_EXHAUSTED", "message": "免费额度已用完" }
```

## 后台任务和额度

```text
照片校验 → 暂存原图 → 事务预占额度并创建 queued 任务
    → 立即返回任务编号 → worker 领取 processing 任务
    → 调模型 → 加工并保存图片 → succeeded
```

- 相同用户、相同幂等键返回同一任务；如果换了照片或选项却复用幂等键，返回冲突。
- 明确生成失败时退回额度，退款只发生一次。
- 网络中断、模型 5xx、超时或已返回却无法加工保存时，标为内部 `uncertain`，保留额度，不自动重调模型。
- `uncertain` 对前端显示 `failed`，同时明确说明“结果暂未确认、额度仍保留、不要重复生成”。当前管理端核对/结算入口尚未开发。
- 只有数据库状态保存可以自动重试，重试不会再次调用模型。
- PostgreSQL 模式中的排队任务在重启后继续领取；中断的 processing 任务超过阈值后转为待核对，不重复调用。
- 更换 `ai.provider` 后，已有排队任务不会偷偷切换供应商。不再可用的供应商任务会明确失败并退款。

## 照片处理和存储

- 后端独立校验图片内容、大小和像素，不信任前端已经压缩过。
- 原图只保存在私有目录，不开放原图静态路径。
- 成品通过会话和任务归属验证后读取，不直接公开整个 data 目录。
- 成功或明确失败后尝试立即删除原图；删除失败、孤立上传或待核对任务按配置的保留时间清理。
- 清理前检查任务状态，仍排队或执行中的任务不会因文件时间较早而丢失原图。
- 当前实现为本地文件存储。生产对象存储、公开分享缩略图及完整的数据删除入口属于后续工作。

## 检查方式

```bash
go test -race ./...
go vet ./...
go build ./cmd/server
```

HTTP 与豆包适配器测试使用本地假服务；额度、并发、退款和任务归属使用内存数据验证。测试不会向真实豆包发送照片，也不会连接你的 Neon。

## 当前边界和下一步

本版采用签名 Cookie 的开发身份，主动限制监听在回环地址，且拒绝以 production 环境启动。正式微信 OpenID 授权尚未接入；配置中的 `wechat` 组是预留位置，不代表已经支持公众号登录。

接下来依次完成：配置专用 Neon 并验证事务 → 配置模型和中文字体后做真实照片试出图 → 微信身份与 JSSDK → 生产对象存储、隐私指引及 AI 显式/隐式标识核对 → 明确并实现免费重做及管理后台。

这版的普通新制作会占用一次额度，尚未将未确定的免费重做规则写死。正式上线前还需要完成微信真机与真实服务验收。

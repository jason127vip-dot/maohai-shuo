# 微信支付 APIv3 首版（2026-11-20）

已实现普通商户 JSAPI 支付、公钥验签模式，使用微信官方 Go SDK v0.2.21。当前是开发完成后的本地验证版本，支付开关仍关闭，尚未部署或进行真实付款验收。

## 业务规则

用户已确认：月卡480分／4次／1个自然月，年卡5800分／48次／12个自然月。每次购买独立保存次数和有效期，不延长旧套餐。以微信确认的支付成功时间起算，按北京时间计算自然月／年，月末取目标月最后一天，同一时刻到期；到期时刻及以后不能再用于新任务。

优先扣有效且有剩余次数、最早到期的套餐，最后扣原有不设到期日的免费余额。任务预占时记录实际扣次来源；失败、结果不确定或任务中断时沿用现有退次政策，幂等退回原套餐或免费余额。退款到已到期套餐不会延长有效期，也不会变成免费永久次数。这里的“退次”不是微信资金退款。

## 配置与证书

- `configs/config.example.yaml`、`config.wechat.example.yaml`：公开模板，新增 `payment` 和可选 `payment_file`，不含真实商户参数。
- `configs/payment.example.yaml`：独立支付文件模板。
- 本地 `configs/config.local.yaml` 已引用 `payment_file: "payment.local.yaml"`。
- 本地 `configs/payment.local.yaml`：已按用户提供资料填写商户号、商户证书序列号、APIv3密钥、微信支付公钥ID、三个PEM路径和两档套餐。`enabled: false`，`notify_url` 留空，待正式HTTPS域名确定后填写。
- 本地 `configs/payment/`：`apiclient_cert.pem`、`apiclient_key.pem`、`pub_key.pem`。私有配置与这些文件已加入Git忽略，文件权限为0600。不保存商户操作密码或APIv2密钥，支付代码不需要它们。
- 主配置中的公众号 `wechat.app_id` 是支付下单的AppID，不另复制一份；OpenID只从该AppID下已登录账号的身份表读取。

`payment_file` 相对主配置所在目录；三个证书路径相对支付文件所在目录（内联配置则相对主配置）。指定独立文件时，不要同时填一份启用的内联支付配置。可使用 `MAOHAI_PAYMENT_API_V3_KEY` 环境变量覆盖密钥。APIv3密钥需32个ASCII字母／数字，不能用APIv2密钥替代。

可配置项：`enabled`、`mch_id`、`certificate_serial`、`merchant_certificate_path`、`private_key_path`、`api_v3_key`、`public_key_id`、`public_key_path`、`notify_url`、`order_minutes`（5～60分钟）、`http_timeout_seconds`（1～15秒）、`plans`（月卡与年卡的名称、金额分、次数、自然月数）。金额由后端配置决定，前端只传套餐ID，页面从后端读取价格和次数。配置变更重启后生效，已有订单保存的价格与权益不被改写。

商户证书不是网站HTTPS证书。启动仅从本地加载证书、公钥与私钥，核对商户号、序列号、有效期和私钥匹配，不自动下载证书、不主动请求微信。公钥ID和公钥是否确实配套，需以同一商户平台下载页面及后续签名联调确认，不能仅靠本地解析证明。

离线检查（不会访问数据库或微信）：

```sh
go run ./cmd/check-payment-config -config configs/config.local.yaml
```

用户曾在聊天中明文提供密钥与操作密码，正式启用前由管理员更换，并同步本地／服务器私有配置；不要发到聊天、提交Git、放入前端或通用补丁包。

## 数据库及部署

1. 保留数据库、运行配置、data、字体和网站代理配置的备份。
2. 目标库已执行001～004后，管理员审阅并手动执行 `migrations/005_wechat_pay.sql`。它新增订单表、付费次数包和额度流水来源字段，旧免费余额、账号、作品保持。程序只读检查，不自动执行DDL。新版本即使暂不开支付，也需要005，因为余额和退次查询已支持套餐。
3. 新程序与前端一起更新。把本地私有配置和三个PEM文件通过安全方式部署到服务器配置目录，不能放在公开的 `www` 或 `assets` 中，不覆盖原公众号密钥／数据库配置／字体。
4. 配置标准HTTPS域名（不使用8088等端口），将公众号登录回调、`wechat.frontend_url`、`allowed_origins`调整一致；支付 `notify_url` 为同站 `https://正式域名/api/payments/wechat/notify`。
5. 商户管理员确认JSAPI权限、当前公众号AppID绑定及支付授权目录。回调地址应由IIS／Nginx转发到Go，允许外部POST，不经过网页登录跳转、不缓存、不改写通知正文。
6. 先核验数据库事务与HTTPS登录，再将 `enabled` 改为true、重启，进行一次授权的微信真机付款验收。启用检查会拒绝非微信身份、非PostgreSQL、HTTP、不匹配的回调站点或不完整配置。
7. 上线后保留后台对账进程：它随Go服务启动，定期查询待确认订单，漏通知也可补发权益；关闭开关会暂停下单、通知和主动查询，但不删除已购套餐或阻止使用未到期次数。

请勿在已发生真实付款后直接恢复不认识付费套餐的旧程序或删除005表；旧程序只知道免费余额，可能误处理正在执行的付费任务。遇到故障先关闭支付入口并保留新版本的数据处理能力，核对订单／套餐／额度流水后决定回退方案。

## 接口与确认到账

| 接口 | 行为 |
| --- | --- |
| `GET /api/payments/catalog` | 当前开关和后端两档套餐 |
| `GET /api/payments/entitlements` | 本人最近100份套餐、剩余次数、到期时间、服务器时间 |
| `GET /api/payments/orders` | 本人最近20笔订单，页面可继续支付／查询 |
| `POST /api/payments/orders` | JSON `planId`，`Idempotency-Key`；持久化订单，不接受金额、用户或OpenID |
| `POST /api/payments/orders/:orderId/pay` | 先查原单；未支付才下单／复用prepay_id并签发调起参数 |
| `POST /api/payments/orders/:orderId/refresh` | 主动查微信，确认成功后发放套餐 |
| `POST /api/payments/wechat/notify` | 无用户Cookie要求，必须公钥验签及APIv3解密，提交数据库事务后才返回204 |

`GET /api/me` 的 `remainingCredits` 是免费余额加当前有效套餐的剩余次数；到期判断在后端每次查询和预占时执行。

订单创建、重复通知、主动查单和后台补偿均保留原订单号。订单快照包括AppID、商户号、付款OpenID、原价、次数与有效期；成功时核对这些字段、币种、交易类型、交易号及成功时间。使用订单总金额校验，优惠导致的 `payer_total` 不能替代总金额。订单与套餐在同一事务提交，交易号及订单ID唯一；并发重试不重复发放。

浏览器收银台的成功回调只触发查单，不能单独发次数；取消可继续同一订单，网络未知只提示查询原订单。创建响应丢失时前端保留提交编号，重试不新建订单；刷新后从本人订单历史恢复。后台查询NOTPAY且超过截止时间后先远程关单，成功后才本地关闭；USERPAYING或未知结果不提前作废。签名、解密、金额或身份校验失败时，不发放套餐。

API响应和日志不返回密钥、私钥、OpenID、微信交易号或完整上游报文。收银台调起参数只返回当前账号的待支付订单，订单列表不返回prepay_id等内部字段。

## 验证与限制

本地已验证：官方SDK请求与调起签名、应答验签、通知验签／AES-GCM解密、改金额／付款人拒绝、重复并发通知只发一次、自然月／闰日／北京时间边界、先到期先扣、已到期原套餐退次、取消与未知结果复用订单、会话与账号隔离。Go race、vet、前端test/lint/TypeScript/生产构建及浏览器结果以本轮PROGRESS记录为准。

PostgreSQL合约扩展到005重复迁移与支付／额度事务，只有显式指定可丢弃的 `MAOHAI_TEST_POSTGRES_URL` 才运行，绝不读取真实配置偷偷连接Neon。未配置该测试库时会skip，不代表数据库验收通过。

尚未完成：真实商户权限核验、目标库迁移、微信真机付款／真实重复通知、线上部署。本首版没有资金退款接口、自助退款、管理后台、账单下载或自动退款后扣回权益；管理员不能在不核对和处理套餐权益的情况下直接从商户平台退款。不能将本地模拟通过称为已正式开通收款。

官方参考：[Go SDK](https://github.com/wechatpay-apiv3/wechatpay-go)、[JSAPI接入准备](https://pay.wechatpay.cn/doc/v3/merchant/4015423216)、[JSAPI下单](https://pay.wechatpay.cn/doc/v3/merchant/4012791856)、[商户订单号查单](https://pay.wechatpay.cn/doc/v3/merchant/4012791859)。

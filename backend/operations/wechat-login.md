# 微信服务号登录部署与验收

登录使用 `snsapi_base` 网页授权获取 OpenID；作品页另有可跳过的 `snsapi_userinfo` 昵称头像同步。后端用 AppID + OpenID 定位内部账号。Cookie 只保存内部用户 ID、到期时间和签名，微信 access_token/AppSecret 不传给前端、不写入业务日志或数据库。资料更新说明见 [history-profile-update.md](history-profile-update.md)。

## 当前地址与代理

当前 H5：`http://maohai.aidistance.cn:8088`。

已验证的域名入口：`http://maohai.aidistance.cn`。该入口的校验文件已能访问；网页授权域名已由管理员保存。

新增回调：`http://maohai.aidistance.cn/api/auth/wechat/callback`。

链路：微信 → Nginx 80 的 `/api/auth/wechat/` → IIS 8088 的 `/api/*` → 回环地址上的 Go → 微信 HTTPS 接口换 OpenID → PostgreSQL 定位账号 → 设置会话 Cookie → 303 返回 H5 8088。两个入口使用同一主机名，Cookie 不区分端口，因此登录后 H5 的 `/api` 请求可以携带回调设置的 Cookie。访问网站必须使用该域名，不能用公网 IP 替代。

IIS ARR 必须保留 Go 返回的外部跳转地址：在 IIS 管理器选择服务器节点 → Application Request Routing Cache → Server Proxy Settings，取消 **Reverse rewrite host in response headers**，保留 Enable proxy，并点击 Apply。否则 ARR 会把微信的 `https://open.weixin.qq.com/connect/oauth2/authorize` 改写成本网站的 `/connect/oauth2/authorize`，SPA 再将该路径提交为 return_to，触发 INVALID_RETURN_PATH。该问题已通过线上登录入口响应头确认。参考：[微软 ARR 配置说明](https://learn.microsoft.com/en-us/iis/extensions/configuring-application-request-routing-arr/arr-as-generic-proxy-in-hotmail-and-skydrive)。

部署时把 `maohai-wechat.nginx.conf.example` 的授权路径合并到现有 `D:\BtSoft\nginx\conf\vhost\maohai-wechat.conf`，不要为同一域名重复创建 server。保留已能访问的校验文件 location。不修改其他站点。`nginx.exe -t` 通过后，用宝塔 Nginx 管理的「重载配置」生效；该服务以 SYSTEM 运行，不沿用此前失败的管理员命令行 reload。

IIS 需保留已有 `/api` 到 Go 的反向代理，且保留完整路径；若该代理只列出了旧接口，补充 `/api/auth/wechat/start`、`/api/auth/wechat/callback`。授权路径的 Nginx/IIS 访问日志不要记录查询参数或 Cookie，避免记录一次性的授权 code；Go 日志只记录路由模板，不记录查询串。服务端微信请求中的 AppSecret 不打印。

当前 HTTP 接入按现有地址支持，Cookie 为 HttpOnly、SameSite=Lax、host-only、Path=/api；HTTP 下不会错误设置 Secure 而导致无法登录。HTTP 链路不加密，正式开放给用户前应安排 HTTPS。迁移 HTTPS 时同时更新 callback_url、frontend_url、allowed_origins 和相应代理；HTTPS 配置会自动启用 Secure Cookie。不要只修改其中一个 URL。

## 执行顺序

1. 备份服务器运行配置、数据库及当前前后端产物。保留已有模型、存储路径、字体、提示词和额度参数。
2. 由管理员审阅后，在项目数据库顺序执行 `migrations/002_wechat_identity.sql`、`003_history_wechat_profile.sql`。先前未建表的环境先执行 `001_initial.sql`。已执行 002 的环境只需补 003；这两份迁移可重复执行，不会删除旧用户或改动余额。应用启动只检查结构，绝不自动执行 DDL。
3. 合并 `configs/config.wechat.example.yaml` 中的 server/auth/wechat 设置到现有配置。保留真实数据库连接串，将 `database.driver` 设为 postgres。前端源地址必须同时列在 `allowed_origins` 中；Go 仍绑定回环地址，不直接暴露到公网。
4. 在服务器填写或通过运行服务的环境变量提供 AppSecret：`MAOHAI_WECHAT_APP_SECRET`。签名密钥可用 `MAOHAI_AUTH_SIGNING_KEY` 覆盖 YAML。环境变量必须属于实际运行 Go 的服务进程；在另一个终端里临时设置不会影响已有服务。不要把 AppSecret 放进任何 `VITE_` 变量、前端文件或 Git。
5. 更新 Go 后端和前端生产构建，前端禁用 mock，并重启 Go 服务加载新配置。只切换前端不能启用持久微信身份。先保持 AI 付费参数不变；验证登录和 `/api/me` 无需生成图片。
6. 合并并重载上述 Nginx 授权路径代理，然后按下方步骤在微信内验收。

示例只列出需合并的核心配置；完整字段见配置模板：

```yaml
server:
  environment: production
  host: 127.0.0.1
  # 保留现有 Go 端口，不是 Nginx 的 80 或 IIS 的 8088。
  allowed_origins:
    - http://maohai.aidistance.cn:8088
auth:
  mode: wechat
  cookie_name: maohai_session
  signing_key: REPLACE_WITH_UNIQUE_RANDOM_SECRET_AT_LEAST_32_CHARS
  session_hours: 24
wechat:
  app_id: wxac7ef35276f43ad7
  app_secret: "" # 在服务器填写，或设置 MAOHAI_WECHAT_APP_SECRET。
  callback_url: http://maohai.aidistance.cn/api/auth/wechat/callback
  frontend_url: http://maohai.aidistance.cn:8088
```

不要把片段当成完整配置直接覆盖。`session_hours` 只控制多久重新微信授权，不影响账号、任务或积分的保留。

## 旧账号直接废弃，微信账号重新开始

不迁移、不绑定旧匿名账号。即便浏览器仍持有有效开发 Cookie，首次微信授权也会创建独立的新账号，并领取配置中的初始积分；之后同一 OpenID 再次授权只恢复这个微信账号，不重复发积分。旧积分和旧任务不继承。

旧匿名账号的物理清理由管理员单独运行 `operations/delete_old_anonymous_accounts.sql`：先备份数据库、停止 Go 服务，执行 002 建表，再运行清理脚本，最后切换微信配置并启动新版本。脚本以事务删除所有没有微信身份映射的旧用户、对应任务、调用记录和额度流水；如已存在微信账号则保留它们。可以重复运行。它不是启动迁移，不会在程序启动时自动删除数据。

清理脚本只处理数据库记录，不删除服务器上的图片文件。数据库任务删除后，旧图片接口会返回不存在；如需回收磁盘，可在确认实际存储路径后另行清理旧任务对应图片，不要直接删除整个存储目录。

会话签名密钥直接使用新的独立随机密钥，无需保留旧开发密钥。已删除 `migrate_legacy_session` 配置项；如果使用了上一版待部署模板，请移除该行。

## 接口与持久化

- `GET /api/auth/wechat/start?return_to=/`：仅微信内发起，创建 10 分钟、浏览器绑定、服务端单次消费的 state，再跳转微信。只接受制作页、隐私页或合法任务结果路径作为返回地址。
- `GET /api/auth/wechat/callback?code=...&state=...`：先验证和消费 state，再向微信换 OpenID，然后定位或创建账号。异常时显示可返回重试的中文页面，不把微信原始错误或凭据返回给浏览器。
- 微信模式的业务接口没有有效会话时返回 `401 WECHAT_LOGIN_REQUIRED`，不创建账号、不发积分。前端在选图前完成登录，并处理会话中途过期。
- 新微信会话的签名用途与旧开发 Cookie 分离，并绑定 AppID；AppID 或密钥变更后必须重新登录。
- `maohai_wechat_identities` 的主键 `(app_id, open_id)` 和唯一 `user_id` 防止重复或多重绑定；首次登录用数据库事务和身份锁处理并发。只有真正插入用户时才写初始积分，后续登录只更新登录时间。
- `maohai_users.last_seen_at` 在已授权业务请求时更新，记录最近活跃时间；不能据此还原过去每一天的 DAU。无需额外开发统计页面即可运行 `operations/wechat_user_stats.sql` 查看当前汇总。

## 验收

1. 微信内打开 H5，跳转授权后回到原页面。普通浏览器应提示在微信打开；隐私页仍可直接阅读。
2. 查看 `/api/me` 可读取余额，Cookie 为 `maohai_session` 的 `wx.` 会话；前端和响应中不得出现 AppSecret、OpenID 或微信 access_token。
3. 如已有新微信账号的任务，验证再次登录后仍可读取；旧匿名任务不继承。登录本身不会调用图片模型，创建图片的付费联调单独安排。
4. 删除会话 Cookie 或缩短有效期并等待到期，再以同一微信授权，核对内部用户 ID、余额、旧任务均保持不变。更换另一微信账号不能读取前一账号的任务。
5. 并发首次登录同一个 OpenID，只能有一条身份映射和一个用户；总初始积分只能发一次。
6. 不带 state Cookie、错 state、过期 state、重复回调都不得登录或写入账号。微信拒绝/网络失败应能返回重试，不应无限跳转。

本地自动化：`go test -race ./...`、`go vet ./...`，前端 `npm test`、`npm run lint`、`npm run build`。微信交换测试使用假的 HTTPS transport，不请求真实微信。可额外指定明确可丢弃的测试数据库 `MAOHAI_TEST_POSTGRES_URL` 运行 PostgreSQL 合约测试；该测试在唯一隔离 schema 内安装迁移，结束后删除该 schema，绝不读取运行配置中的 Neon 连接串。未设置时会明确 skip。

参考：[腾讯云开发服务号网页授权文档](https://docs.cloudbase.net/en/integration/wechat-official-oauth)。本项目直接由 Go 调微信，不使用 CloudBase 登录或云函数。

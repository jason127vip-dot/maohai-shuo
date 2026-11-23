// Offline diagnostics only: never starts HTTP, opens the database, or calls WeChat.
package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/jason127vip-dot/maohai-shuo/backend/internal/config"
	"github.com/jason127vip-dot/maohai-shuo/backend/internal/wechatpay"
	"os"
)

func main() {
	path := flag.String("config", "configs/config.local.yaml", "配置路径")
	flag.Parse()
	c, _, err := config.Load(*path)
	if err == nil {
		_, err = wechatpay.New(context.Background(), c.Payment)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "离线支付配置检查失败：", err.Error())
		os.Exit(1)
	}
	fmt.Printf("证书、公私钥格式、证书匹配和有效期检查通过。支付启用：%t；通知地址已填写：%t。未验证公钥ID对应关系、商户权限、数据库或真实支付。\n", c.Payment.Enabled, c.Payment.NotifyURL != "")
}

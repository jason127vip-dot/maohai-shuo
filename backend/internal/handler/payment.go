package handler

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/gin-gonic/gin"
	"github.com/jason127vip-dot/maohai-shuo/backend/internal/model"
	"github.com/jason127vip-dot/maohai-shuo/backend/internal/service"
	"io"
	"net/http"
	"time"
)

func registerPayments(router *gin.Engine, api *gin.RouterGroup, p *service.Payment) {
	api.GET("/payments/catalog", func(c *gin.Context) {
		c.Header("Cache-Control", "no-store")
		c.JSON(200, gin.H{"enabled": p.Enabled(), "plans": p.Config.Payment.Plans})
	})
	api.GET("/payments/entitlements", func(c *gin.Context) {
		items, err := p.Store.Entitlements(c.Request.Context(), c.GetString("userID"))
		if err != nil {
			respondError(c, err)
			return
		}
		c.JSON(200, gin.H{"items": items, "serverTime": time.Now().UTC()})
	})
	api.GET("/payments/orders", func(c *gin.Context) {
		items, err := p.Store.ListOrders(c.Request.Context(), c.GetString("userID"), 20)
		if err != nil {
			respondError(c, err)
			return
		}
		c.JSON(200, gin.H{"items": items})
	})
	api.POST("/payments/orders", func(c *gin.Context) {
		var body struct {
			PlanID string `json:"planId"`
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 4096)
		decoder := json.NewDecoder(c.Request.Body)
		decoder.DisallowUnknownFields()
		if decoder.Decode(&body) != nil || decoder.Decode(new(any)) != io.EOF {
			c.JSON(400, gin.H{"code": "INVALID_INPUT", "message": "订单只需要提交套餐标识"})
			return
		}
		order, err := p.Create(c.Request.Context(), c.GetString("userID"), body.PlanID, c.GetHeader("Idempotency-Key"))
		if err != nil {
			if errors.Is(err, model.ErrConflict) {
				c.JSON(409, gin.H{"code": "PAYMENT_REQUEST_CONFLICT", "message": "同一订单提交编号不能用于不同套餐，请重新选择"})
				return
			}
			respondError(c, err)
			return
		}
		c.JSON(200, order)
	})
	api.POST("/payments/orders/:orderId/pay", func(c *gin.Context) {
		order, params, err := p.Pay(c.Request.Context(), c.Param("orderId"), c.GetString("userID"))
		if err != nil {
			respondError(c, err)
			return
		}
		c.JSON(200, gin.H{"order": order, "payParams": params})
	})
	api.POST("/payments/orders/:orderId/refresh", func(c *gin.Context) {
		order, err := p.Refresh(c.Request.Context(), c.Param("orderId"), c.GetString("userID"))
		if err != nil {
			respondError(c, err)
			return
		}
		c.JSON(200, order)
	})
	router.POST("/api/payments/wechat/notify", func(c *gin.Context) {
		if !p.Enabled() {
			c.JSON(503, gin.H{"code": "FAIL", "message": "支付暂不可用"})
			return
		}
		ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
		defer cancel()
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 65536)
		trade, err := p.Gateway.ParseNotify(ctx, c.Request)
		if err != nil {
			c.JSON(400, gin.H{"code": "FAIL", "message": "通知验证失败"})
			return
		}
		if err = p.Store.SettleOrder(ctx, trade); err != nil {
			c.JSON(503, gin.H{"code": "FAIL", "message": "通知尚未处理成功"})
			return
		}
		// Success only after durable order + entitlement transaction has committed.
		c.Status(http.StatusNoContent)
	})
}

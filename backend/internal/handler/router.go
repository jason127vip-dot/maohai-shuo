package handler

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jason127vip-dot/maohai-shuo/backend/internal/config"
	"github.com/jason127vip-dot/maohai-shuo/backend/internal/middleware"
	"github.com/jason127vip-dot/maohai-shuo/backend/internal/model"
	"github.com/jason127vip-dot/maohai-shuo/backend/internal/service"
	"github.com/jason127vip-dot/maohai-shuo/backend/internal/wechat"
	"github.com/jason127vip-dot/maohai-shuo/backend/internal/wechatpay"
)

func NewRouter(cfg config.Config, generation *service.Generation, log *slog.Logger, payment ...*service.Payment) *gin.Engine {
	return newRouter(cfg, generation, log, wechat.NewClient(cfg.Wechat.AppID, cfg.Wechat.AppSecret), payment...)
}

func newRouter(cfg config.Config, generation *service.Generation, log *slog.Logger, exchange wechat.Exchanger, payment ...*service.Payment) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	_ = router.SetTrustedProxies(nil)
	router.Use(middleware.RequestLog(log), middleware.Recovery(log), middleware.OriginCheck(cfg.Server.AllowedOrigins))
	router.GET("/api/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok", "environment": cfg.Server.Environment, "provider": cfg.AI.Provider, "database": cfg.Database.Driver})
	})
	session := middleware.DevelopmentSession(cfg, generation.Store)
	if cfg.Auth.Mode == "wechat" {
		registerWechatAuth(router, cfg, generation.Store, exchange, log)
		session = middleware.WechatSession(cfg, generation.Store)
	}
	api := router.Group("/api", session)
	api.Use(func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), time.Duration(cfg.Server.ReadTimeoutSeconds)*time.Second)
		defer cancel()
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	})
	api.GET("/me", func(c *gin.Context) {
		credits, err := generation.Store.Credits(c.Request.Context(), c.GetString("userID"))
		if err != nil {
			respondError(c, err)
			return
		}
		c.Header("Cache-Control", "no-store")
		profile, err := generation.Store.UserProfile(c.Request.Context(), c.GetString("userID"))
		if err != nil {
			respondError(c, err)
			return
		}
		c.JSON(200, gin.H{"remainingCredits": credits, "nickname": profile.Nickname, "avatarUrl": profile.AvatarURL, "canSyncWechatProfile": cfg.Auth.Mode == "wechat"})
	})
	api.GET("/generations", historyHandler(generation.Store))
	pay := &service.Payment{Store: generation.Store, Config: cfg}
	if len(payment) > 0 && payment[0] != nil {
		pay = payment[0]
	}
	registerPayments(router, api, pay)
	registerPets(api, generation.Store)
	api.POST("/generations", func(c *gin.Context) {
		limit := cfg.Storage.MaxUploadMB * 1024 * 1024
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, limit+(1<<20))
		err := c.Request.ParseMultipartForm(1 << 20)
		if c.Request.MultipartForm != nil {
			defer c.Request.MultipartForm.RemoveAll()
		}
		if err != nil {
			c.JSON(400, gin.H{"code": "INVALID_UPLOAD", "message": "图片上传格式不正确或超过大小上限"})
			return
		}
		file, header, err := c.Request.FormFile("photo")
		if err != nil {
			c.JSON(400, gin.H{"code": "PHOTO_REQUIRED", "message": "请选择宠物照片"})
			return
		}
		defer file.Close()
		if header.Size > limit {
			c.JSON(413, gin.H{"code": "PHOTO_TOO_LARGE", "message": "照片超过允许的大小"})
			return
		}
		data, err := io.ReadAll(io.LimitReader(file, limit+1))
		if err != nil || int64(len(data)) > limit {
			c.JSON(413, gin.H{"code": "PHOTO_TOO_LARGE", "message": "照片无法读取或超过允许的大小"})
			return
		}
		showLabel := c.Request.FormValue("showPetLabel")
		if showLabel != "" && showLabel != "true" && showLabel != "false" {
			c.JSON(400, gin.H{"code": "INVALID_INPUT", "message": "名字和年龄显示选项不正确"})
			return
		}
		task, created, err := generation.Create(c.Request.Context(), c.GetString("userID"), service.Input{Photo: data, Scene: c.Request.FormValue("scene"), Style: c.Request.FormValue("style"), IdempotencyKey: c.GetHeader("Idempotency-Key"), PetID: c.Request.FormValue("petId"), ShowPetLabel: showLabel == "true"})
		if err != nil {
			respondError(c, err)
			return
		}
		status := http.StatusOK
		if created {
			status = http.StatusAccepted
		}
		c.Header("Cache-Control", "no-store")
		c.JSON(status, task.Response())
	})
	api.GET("/generations/:taskId", func(c *gin.Context) {
		task, err := generation.Get(c.Request.Context(), c.Param("taskId"), c.GetString("userID"))
		if err != nil {
			respondError(c, err)
			return
		}
		c.Header("Cache-Control", "no-store")
		c.JSON(200, task.Response())
	})
	api.GET("/images/:taskId", func(c *gin.Context) {
		path, err := generation.ImagePath(c.Request.Context(), c.Param("taskId"), c.GetString("userID"))
		if err != nil {
			respondError(c, err)
			return
		}
		c.Header("Cache-Control", "private, no-store")
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("Content-Disposition", `inline; filename="maohai-`+c.Param("taskId")+`.jpg"`)
		c.File(path)
	})
	router.NoRoute(func(c *gin.Context) { c.JSON(404, gin.H{"code": "NOT_FOUND", "message": "接口不存在"}) })
	return router
}

func respondError(c *gin.Context, err error) {
	var input *service.ValidationError
	var paymentRequest *wechatpay.RequestError
	switch {
	case errors.As(err, &input):
		c.JSON(400, gin.H{"code": "INVALID_INPUT", "message": input.Message})
	case errors.Is(err, model.ErrNotFound):
		c.JSON(404, gin.H{"code": "NOT_FOUND", "message": "记录不存在或无权访问"})
	case errors.Is(err, model.ErrQuota):
		c.JSON(409, gin.H{"code": "QUOTA_EXHAUSTED", "message": "可用次数已用完或套餐已到期"})
	case errors.Is(err, model.ErrPaymentDisabled):
		c.JSON(503, gin.H{"code": "PAYMENT_DISABLED", "message": "微信支付暂未开放"})
	case errors.As(err, &paymentRequest):
		c.JSON(503, gin.H{"code": "WECHATPAY_REQUEST_FAILED", "message": paymentRequest.Error()})
	case errors.Is(err, model.ErrTooManyOrders):
		c.JSON(409, gin.H{"code": "TOO_MANY_ORDERS", "message": "还有多笔待支付订单，请先查询或支付原订单"})
	case errors.Is(err, model.ErrPaymentMismatch):
		c.JSON(409, gin.H{"code": "PAYMENT_MISMATCH", "message": "支付信息尚未核对成功，请稍后查询订单或联系管理员"})
	case errors.Is(err, model.ErrPetAssigned):
		c.JSON(409, gin.H{"code": "PET_ALREADY_ASSIGNED", "message": "作品已经关联宠物，不能重复归属给另一只宠物"})
	case errors.Is(err, model.ErrConflict):
		c.JSON(409, gin.H{"code": "IDEMPOTENCY_CONFLICT", "message": "同一提交编号不能用于不同照片或选项"})
	case errors.Is(err, model.ErrActiveTask):
		c.JSON(409, gin.H{"code": "TASK_IN_PROGRESS", "message": "已有任务正在排队或生成，请稍后查看原任务结果"})
	default:
		c.JSON(503, gin.H{"code": "SERVICE_UNAVAILABLE", "message": "服务暂时不可用，请稍后重试；重试时保留原提交编号"})
	}
}

package wechatpay

import (
	"bytes"
	"context"
	"crypto/rsa"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jason127vip-dot/maohai-shuo/backend/internal/config"
	"github.com/jason127vip-dot/maohai-shuo/backend/internal/model"
	"github.com/wechatpay-apiv3/wechatpay-go/core"
	"github.com/wechatpay-apiv3/wechatpay-go/core/auth/verifiers"
	"github.com/wechatpay-apiv3/wechatpay-go/core/notify"
	"github.com/wechatpay-apiv3/wechatpay-go/core/option"
	"github.com/wechatpay-apiv3/wechatpay-go/services/payments"
	"github.com/wechatpay-apiv3/wechatpay-go/services/payments/jsapi"
	"github.com/wechatpay-apiv3/wechatpay-go/utils"
)

var ErrUnavailable = errors.New("微信支付暂时无法确认，请稍后查询原订单")
var ErrInvalidNotify = errors.New("支付通知验签或解密失败")

type PayParams struct {
	AppID     string `json:"appId"`
	TimeStamp string `json:"timeStamp"`
	NonceStr  string `json:"nonceStr"`
	Package   string `json:"package"`
	SignType  string `json:"signType"`
	PaySign   string `json:"paySign"`
}
type Gateway interface {
	Prepay(context.Context, model.Order) (string, error)
	Parameters(context.Context, string, string) (PayParams, error)
	Query(context.Context, model.Order) (model.PaymentTrade, error)
	Close(context.Context, model.Order) error
	ParseNotify(context.Context, *http.Request) (model.PaymentTrade, error)
}
type Client struct {
	api    *jsapi.JsapiApiService
	signer *core.Client
	notify *notify.Handler
	cfg    config.Payment
}

// Loads keys offline: no certificate downloader or network request on startup.
func New(ctx context.Context, c config.Payment) (*Client, error) {
	return newClient(ctx, c, &http.Client{Timeout: time.Duration(c.HTTPTimeoutSeconds) * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }})
}
func newClient(ctx context.Context, c config.Payment, httpClient *http.Client) (*Client, error) {
	if len(c.APIv3Key) != 32 {
		return nil, errors.New("APIv3密钥必须为32字节")
	}
	key, err := utils.LoadPrivateKeyWithPath(c.PrivateKeyPath)
	if err != nil {
		return nil, errors.New("支付商户私钥无法加载")
	}
	cert, err := utils.LoadCertificateWithPath(c.MerchantCertificatePath)
	if err != nil {
		return nil, errors.New("支付商户证书无法加载")
	}
	public, err := utils.LoadPublicKeyWithPath(c.PublicKeyPath)
	if err != nil {
		return nil, errors.New("微信支付公钥无法加载")
	}
	cp, ok := cert.PublicKey.(*rsa.PublicKey)
	if !ok || key.N.BitLen() < 2048 || public.N.BitLen() < 2048 || cp.N.Cmp(key.N) != 0 || cp.E != key.E || !strings.EqualFold(cert.SerialNumber.Text(16), strings.TrimLeft(c.CertificateSerial, "0")) || cert.Subject.CommonName != c.MchID || time.Now().Before(cert.NotBefore) || !time.Now().Before(cert.NotAfter) {
		return nil, errors.New("商户证书、序列号、商户号、私钥不匹配或证书已过期")
	}
	client, err := core.NewClient(ctx, option.WithWechatPayPublicKeyAuthCipher(c.MchID, c.CertificateSerial, key, c.PublicKeyID, public), option.WithHTTPClient(httpClient))
	if err != nil {
		return nil, errors.New("支付客户端初始化失败")
	}
	handler, err := notify.NewRSANotifyHandler(c.APIv3Key, verifiers.NewSHA256WithRSAPubkeyVerifier(c.PublicKeyID, *public))
	if err != nil {
		return nil, errors.New("支付通知解密配置不正确")
	}
	return &Client{api: &jsapi.JsapiApiService{Client: client}, signer: client, notify: handler, cfg: c}, nil
}
func (c *Client) Prepay(ctx context.Context, o model.Order) (string, error) {
	resp, result, err := c.api.Prepay(ctx, jsapi.PrepayRequest{Appid: core.String(o.AppID), Mchid: core.String(o.MchID), Description: core.String("毛孩儿说-" + o.PlanName), OutTradeNo: core.String(o.ID), TimeExpire: &o.ExpiresAt, NotifyUrl: core.String(c.cfg.NotifyURL), Amount: &jsapi.Amount{Total: core.Int64(int64(o.AmountFen)), Currency: core.String("CNY")}, Payer: &jsapi.Payer{Openid: core.String(o.OpenID)}})
	if err != nil || resp == nil || resp.PrepayId == nil || *resp.PrepayId == "" {
		return "", requestFailure("prepay", result, err)
	}
	return *resp.PrepayId, nil
}
func (c *Client) Parameters(ctx context.Context, app, prepay string) (PayParams, error) {
	nonce, err := utils.GenerateNonce()
	if err != nil {
		return PayParams{}, requestFailure("sign", nil, err)
	}
	p := PayParams{AppID: app, TimeStamp: strconv.FormatInt(time.Now().Unix(), 10), NonceStr: nonce, Package: "prepay_id=" + prepay, SignType: "RSA"}
	signature, err := c.signer.Sign(ctx, fmt.Sprintf("%s\n%s\n%s\n%s\n", p.AppID, p.TimeStamp, p.NonceStr, p.Package))
	if err != nil {
		return p, requestFailure("sign", nil, err)
	}
	p.PaySign = signature.Signature
	return p, nil
}
func (c *Client) Query(ctx context.Context, o model.Order) (model.PaymentTrade, error) {
	resp, result, err := c.api.QueryOrderByOutTradeNo(ctx, jsapi.QueryOrderByOutTradeNoRequest{OutTradeNo: core.String(o.ID), Mchid: core.String(o.MchID)})
	if core.IsAPIError(err, "ORDER_NOT_EXIST") {
		return model.PaymentTrade{OrderID: o.ID, State: "NOTFOUND"}, nil
	}
	if err != nil || resp == nil {
		return model.PaymentTrade{}, requestFailure("query", result, err)
	}
	return trade(resp), nil
}
func (c *Client) Close(ctx context.Context, o model.Order) error {
	result, err := c.api.CloseOrder(ctx, jsapi.CloseOrderRequest{OutTradeNo: core.String(o.ID), Mchid: core.String(o.MchID)})
	if err != nil && !core.IsAPIError(err, "ORDER_CLOSED") && !core.IsAPIError(err, "ORDER_NOT_EXIST") {
		return requestFailure("close", result, err)
	}
	return nil
}
func (c *Client) ParseNotify(ctx context.Context, r *http.Request) (model.PaymentTrade, error) {
	// Bound memory and validate nonce size before the SDK calls AES-GCM Open.
	raw, err := io.ReadAll(io.LimitReader(r.Body, 65537))
	if err != nil || len(raw) > 65536 {
		return model.PaymentTrade{}, ErrInvalidNotify
	}
	var envelope struct {
		EventType    string `json:"event_type"`
		ResourceType string `json:"resource_type"`
		Resource     struct {
			Nonce        string `json:"nonce"`
			OriginalType string `json:"original_type"`
		} `json:"resource"`
	}
	if json.Unmarshal(raw, &envelope) != nil || len(envelope.Resource.Nonce) != 12 || envelope.EventType != "TRANSACTION.SUCCESS" || envelope.ResourceType != "encrypt-resource" || envelope.Resource.OriginalType != "transaction" {
		return model.PaymentTrade{}, ErrInvalidNotify
	}
	r.Body = io.NopCloser(bytes.NewReader(raw))
	var result payments.Transaction
	if _, err = c.notify.ParseNotifyRequest(ctx, r, &result); err != nil {
		return model.PaymentTrade{}, ErrInvalidNotify
	}
	return trade(&result), nil
}
func str(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}
func trade(t *payments.Transaction) model.PaymentTrade {
	out := model.PaymentTrade{OrderID: str(t.OutTradeNo), AppID: str(t.Appid), MchID: str(t.Mchid), TransactionID: str(t.TransactionId), State: str(t.TradeState), TradeType: str(t.TradeType)}
	if t.Payer != nil {
		out.OpenID = str(t.Payer.Openid)
	}
	if t.Amount != nil {
		out.Currency = str(t.Amount.Currency)
		if t.Amount.Total != nil {
			out.AmountFen = int(*t.Amount.Total)
		}
	}
	out.PaidAt, _ = time.Parse(time.RFC3339, str(t.SuccessTime))
	return out
}

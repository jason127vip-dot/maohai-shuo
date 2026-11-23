package wechatpay

import (
	"bytes"
	"context"
	"crypto"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"io"
	"math/big"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jason127vip-dot/maohai-shuo/backend/internal/config"
	"github.com/jason127vip-dot/maohai-shuo/backend/internal/model"
)

type roundTrip func(*http.Request) (*http.Response, error)

func (f roundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func testKeys(t *testing.T) (config.Payment, *rsa.PrivateKey, *rsa.PrivateKey) {
	t.Helper()
	merchant, e := rsa.GenerateKey(rand.Reader, 2048)
	if e != nil {
		t.Fatal(e)
	}
	platform, e := rsa.GenerateKey(rand.Reader, 2048)
	if e != nil {
		t.Fatal(e)
	}
	dir := t.TempDir()
	c := config.Payment{MchID: "merchant-test", CertificateSerial: "1234", PublicKeyID: "PUB_KEY_ID_TEST", APIv3Key: "0123456789abcdef0123456789abcdef", PrivateKeyPath: filepath.Join(dir, "key.pem"), PublicKeyPath: filepath.Join(dir, "pub.pem"), MerchantCertificatePath: filepath.Join(dir, "cert.pem"), NotifyURL: "https://example.test/api/payments/wechat/notify", HTTPTimeoutSeconds: 5}
	template := &x509.Certificate{SerialNumber: big.NewInt(0x1234), Subject: pkix.Name{CommonName: c.MchID}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature}
	cert, e := x509.CreateCertificate(rand.Reader, template, template, &merchant.PublicKey, merchant)
	if e != nil {
		t.Fatal(e)
	}
	private, e := x509.MarshalPKCS8PrivateKey(merchant)
	if e != nil {
		t.Fatal(e)
	}
	pub, e := x509.MarshalPKIXPublicKey(&platform.PublicKey)
	if e != nil {
		t.Fatal(e)
	}
	for path, block := range map[string]*pem.Block{c.PrivateKeyPath: {Type: "PRIVATE KEY", Bytes: private}, c.MerchantCertificatePath: {Type: "CERTIFICATE", Bytes: cert}, c.PublicKeyPath: {Type: "PUBLIC KEY", Bytes: pub}} {
		if e = os.WriteFile(path, pem.EncodeToMemory(block), 0600); e != nil {
			t.Fatal(e)
		}
	}
	return c, merchant, platform
}
func signature(t *testing.T, key *rsa.PrivateKey, message string) string {
	t.Helper()
	sum := sha256.Sum256([]byte(message))
	out, e := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, sum[:])
	if e != nil {
		t.Fatal(e)
	}
	return base64.StdEncoding.EncodeToString(out)
}
func signedHeaders(t *testing.T, key *rsa.PrivateKey, id string, body []byte) http.Header {
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	nonce := "test-header-nonce"
	h := http.Header{}
	h.Set("Wechatpay-Timestamp", timestamp)
	h.Set("Wechatpay-Nonce", nonce)
	h.Set("Wechatpay-Serial", id)
	h.Set("Wechatpay-Signature", signature(t, key, timestamp+"\n"+nonce+"\n"+string(body)+"\n"))
	h.Set("Content-Type", "application/json")
	return h
}
func TestOfficialSDKResponseAndInvocationSignatures(t *testing.T) {
	cfg, merchant, platform := testKeys(t)
	tamper := false
	httpClient := &http.Client{Transport: roundTrip(func(r *http.Request) (*http.Response, error) {
		if r.URL.Host != "api.mch.weixin.qq.com" {
			t.Fatal("unexpected upstream")
		}
		raw, e := io.ReadAll(r.Body)
		if e != nil {
			t.Fatal(e)
		}
		var input struct {
			Amount struct {
				Total int `json:"total"`
			} `json:"amount"`
			Payer struct {
				OpenID string `json:"openid"`
			} `json:"payer"`
		}
		if json.Unmarshal(raw, &input) != nil || input.Amount.Total != 480 || input.Payer.OpenID != "openid-test" {
			t.Fatal("incorrect server controlled amount/payer")
		}
		auth := map[string]string{}
		for _, part := range strings.Split(strings.TrimPrefix(r.Header.Get("Authorization"), "WECHATPAY2-SHA256-RSA2048 "), ",") {
			k, v, ok := strings.Cut(part, "=")
			if ok {
				auth[strings.TrimSpace(k)] = strings.Trim(v, "\"")
			}
		}
		message := r.Method + "\n" + r.URL.RequestURI() + "\n" + auth["timestamp"] + "\n" + auth["nonce_str"] + "\n" + string(raw) + "\n"
		sum := sha256.Sum256([]byte(message))
		sig, e := base64.StdEncoding.DecodeString(auth["signature"])
		if e != nil || rsa.VerifyPKCS1v15(&merchant.PublicKey, crypto.SHA256, sum[:], sig) != nil {
			t.Fatal("request not signed by merchant private key")
		}
		body := []byte(`{"prepay_id":"test-prepay"}`)
		h := signedHeaders(t, platform, cfg.PublicKeyID, body)
		if tamper {
			body = []byte(`{"prepay_id":"tampered"}`)
		}
		return &http.Response{StatusCode: 200, Header: h, Body: io.NopCloser(bytes.NewReader(body)), Request: r}, nil
	})}
	c, e := newClient(context.Background(), cfg, httpClient)
	if e != nil {
		t.Fatal(e)
	}
	order := model.Order{ID: "order-test", AppID: "wx-test", MchID: cfg.MchID, PlanName: "月卡", AmountFen: 480, OpenID: "openid-test", ExpiresAt: time.Now().Add(15 * time.Minute)}
	prepay, e := c.Prepay(context.Background(), order)
	if e != nil || prepay != "test-prepay" {
		t.Fatal("prepay response not verified", e)
	}
	params, e := c.Parameters(context.Background(), order.AppID, prepay)
	if e != nil {
		t.Fatal(e)
	}
	sum := sha256.Sum256([]byte(params.AppID + "\n" + params.TimeStamp + "\n" + params.NonceStr + "\n" + params.Package + "\n"))
	sig, _ := base64.StdEncoding.DecodeString(params.PaySign)
	if params.SignType != "RSA" || rsa.VerifyPKCS1v15(&merchant.PublicKey, crypto.SHA256, sum[:], sig) != nil {
		t.Fatal("browser invocation signature invalid")
	}
	tamper = true
	if _, e = c.Prepay(context.Background(), order); e == nil {
		t.Fatal("tampered response accepted")
	}
	cfg.CertificateSerial = "ABCD"
	if _, e = newClient(context.Background(), cfg, httpClient); e == nil {
		t.Fatal("wrong serial accepted")
	}
}
func TestNotifyVerifiesSignatureAndDecryptsBeforeTrust(t *testing.T) {
	cfg, _, platform := testKeys(t)
	c, e := New(context.Background(), cfg)
	if e != nil {
		t.Fatal(e)
	}
	plain := []byte(`{"out_trade_no":"order-test","appid":"wx-test","mchid":"merchant-test","trade_state":"SUCCESS","trade_type":"JSAPI","transaction_id":"transaction-test","success_time":"2026-11-20T10:30:00+08:00","payer":{"openid":"openid-test"},"amount":{"total":480,"payer_total":1,"currency":"CNY"}}`)
	block, _ := aes.NewCipher([]byte(cfg.APIv3Key))
	aead, _ := cipher.NewGCM(block)
	nonce := "012345678901"
	aad := "transaction"
	encrypted := aead.Seal(nil, []byte(nonce), plain, []byte(aad))
	envelope := map[string]any{"id": "notification-test", "event_type": "TRANSACTION.SUCCESS", "resource_type": "encrypt-resource", "resource": map[string]any{"algorithm": "AEAD_AES_256_GCM", "nonce": nonce, "associated_data": aad, "original_type": "transaction", "ciphertext": base64.StdEncoding.EncodeToString(encrypted)}}
	body, _ := json.Marshal(envelope)
	request := func(b []byte) *http.Request {
		r, _ := http.NewRequest("POST", "https://example.test/api/payments/wechat/notify", bytes.NewReader(b))
		r.Header = signedHeaders(t, platform, cfg.PublicKeyID, b)
		return r
	}
	out, e := c.ParseNotify(context.Background(), request(body))
	if e != nil || out.AmountFen != 480 || out.OpenID != "openid-test" || out.TransactionID != "transaction-test" {
		t.Fatal("valid encrypted notification failed", e)
	}
	for _, mutate := range []func(*http.Request){
		func(r *http.Request) { r.Header.Set("Wechatpay-Signature", "WECHATPAY/SIGNTEST/invalid") },
		func(r *http.Request) { r.Header.Set("Wechatpay-Serial", "PUB_KEY_ID_OTHER") },
		func(r *http.Request) { r.Header.Set("Wechatpay-Timestamp", "1") },
		func(r *http.Request) {
			r.Body = io.NopCloser(bytes.NewReader(bytes.ReplaceAll(body, []byte("transaction"), []byte("altered"))))
		},
	} {
		r := request(body)
		mutate(r)
		if _, e = c.ParseNotify(context.Background(), r); e == nil {
			t.Fatal("invalid signature/header/body trusted")
		}
	}
	resource := envelope["resource"].(map[string]any)
	resource["nonce"] = "short"
	bad, _ := json.Marshal(envelope)
	if _, e = c.ParseNotify(context.Background(), request(bad)); e == nil {
		t.Fatal("invalid nonce accepted")
	}
	if _, e = c.ParseNotify(context.Background(), request(bytes.Repeat([]byte("x"), 65537))); e == nil {
		t.Fatal("oversized notification accepted")
	}
	// A different, valid-length decryption key must fail.
	cfg.APIv3Key = strings.Repeat("b", 32)
	wrong, e := New(context.Background(), cfg)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = wrong.ParseNotify(context.Background(), request(body)); e == nil {
		t.Fatal("incorrect decryption key accepted")
	}
}

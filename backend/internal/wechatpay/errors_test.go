package wechatpay

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/jason127vip-dot/maohai-shuo/backend/internal/model"
	"github.com/wechatpay-apiv3/wechatpay-go/core"
)

func TestPaymentDiagnosticExcludesRawUpstreamData(t *testing.T) {
	const secret = "PRIVATE_PAYER_AND_AUTHORIZATION"
	var logs bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })
	apiError := &core.APIError{StatusCode: 403, Code: "NO_AUTH", Message: secret, Body: secret,
		Header: http.Header{"Authorization": {secret}}, Detail: map[string]string{"openid": secret}}
	failure := requestFailure("prepay", nil, fmt.Errorf("SDK wrapper: %w", apiError))
	var diagnostic *RequestError
	if !errors.As(failure, &diagnostic) || !errors.Is(failure, ErrUnavailable) || diagnostic.Code != "NO_AUTH" || diagnostic.HTTPStatus != 403 {
		t.Fatal("diagnostic lost stage/code/status or retry classification")
	}
	if !strings.Contains(failure.Error(), "微信预支付") || !strings.Contains(failure.Error(), "NO_AUTH") {
		t.Fatal("failure does not identify the failing phase")
	}
	if strings.Contains(failure.Error()+logs.String(), secret) {
		t.Fatal("private SDK data escaped into message or logs")
	}
	logs.Reset()
	apiError.Code = secret
	failure = requestFailure(secret, nil, apiError)
	if strings.Contains(failure.Error()+logs.String(), secret) || !strings.Contains(failure.Error(), "UPSTREAM_ERROR") {
		t.Fatal("unrecognized upstream code/operation was not bounded")
	}
	forged := &RequestError{Operation: secret, Code: secret}
	if strings.Contains(forged.Error(), secret) {
		t.Fatal("diagnostic message trusts raw fields")
	}

	for _, test := range []struct {
		err    error
		result *core.APIResult
		code   string
	}{
		{context.DeadlineExceeded, nil, "TIMEOUT"},
		{errors.New(secret), &core.APIResult{Response: &http.Response{StatusCode: 200}}, "RESPONSE_CHECK_FAILED"},
		{errors.New(secret), nil, "SDK_ERROR"},
	} {
		logs.Reset()
		failure = requestFailure("query", test.result, test.err)
		if !errors.As(failure, &diagnostic) || diagnostic.Code != test.code || strings.Contains(logs.String()+failure.Error(), secret) {
			t.Fatal("non-API failure classification or redaction failed")
		}
	}
}

func TestSDKPaymentFailureStagesAndExpectedOrderStates(t *testing.T) {
	cfg, _, _ := testKeys(t)
	code, status := "NO_AUTH", 403
	httpClient := &http.Client{Transport: roundTrip(func(r *http.Request) (*http.Response, error) {
		body := `{"code":"` + code + `","message":"private upstream detail"}`
		return &http.Response{StatusCode: status, Header: http.Header{"Content-Type": {"application/json"}}, Body: io.NopCloser(strings.NewReader(body)), Request: r}, nil
	})}
	client, err := newClient(context.Background(), cfg, httpClient)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	order := model.Order{ID: "test-order", AppID: "wx-test", MchID: cfg.MchID, OpenID: "private-payer", AmountFen: 480, PlanName: "月卡", ExpiresAt: time.Now().Add(15 * time.Minute)}
	for _, operation := range []string{"prepay", "query", "close"} {
		t.Run(operation, func(t *testing.T) {
			var failure error
			switch operation {
			case "prepay":
				_, failure = client.Prepay(ctx, order)
			case "query":
				_, failure = client.Query(ctx, order)
			case "close":
				failure = client.Close(ctx, order)
			}
			var diagnostic *RequestError
			if !errors.As(failure, &diagnostic) || diagnostic.Operation != operation || diagnostic.Code != "NO_AUTH" || diagnostic.HTTPStatus != 403 {
				t.Fatal("SDK failure did not retain safe stage/code/status", failure)
			}
		})
	}
	code, status = "ORDER_NOT_EXIST", 404
	trade, err := client.Query(ctx, order)
	if err != nil || trade.State != "NOTFOUND" || trade.OrderID != order.ID {
		t.Fatal("expected missing order was treated as a failure")
	}
	if err = client.Close(ctx, order); err != nil {
		t.Fatal("closing a missing order changed behavior")
	}
	code, status = "ORDER_CLOSED", 400
	if err = client.Close(ctx, order); err != nil {
		t.Fatal("already-closed order changed behavior")
	}
}

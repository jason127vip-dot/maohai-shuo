package wechatpay

import (
	"context"
	"crypto/x509"
	"errors"
	"fmt"
	"log/slog"
	"net"

	"github.com/wechatpay-apiv3/wechatpay-go/core"
)

// RequestError retains only bounded diagnostic fields, never the SDK error's
// body, headers, message, payer identifiers or authorization material.
type RequestError struct {
	Operation  string
	Code       string
	HTTPStatus int
}

func safeOperation(value string) (string, string) {
	switch value {
	case "query":
		return value, "微信查单"
	case "prepay":
		return value, "微信预支付"
	case "close":
		return value, "微信关单"
	case "sign":
		return value, "支付参数签名"
	default:
		return "payment", "微信支付"
	}
}

func safeCode(value string) string {
	switch value {
	case "PARAM_ERROR", "INVALID_REQUEST", "SIGN_ERROR", "SYSTEM_ERROR",
		"APPID_MCHID_NOT_MATCH", "MCH_NOT_EXISTS", "NO_AUTH", "OUT_TRADE_NO_USED",
		"FREQUENCY_LIMITED", "ORDER_NOT_EXIST", "ORDER_CLOSED", "ORDERPAID",
		"NETWORK_ERROR", "TIMEOUT", "TLS_ERROR", "RESPONSE_CHECK_FAILED", "SDK_ERROR":
		return value
	default:
		return "UPSTREAM_ERROR"
	}
}

func (e *RequestError) Error() string {
	_, label := safeOperation(e.Operation)
	return fmt.Sprintf("%s未成功（%s），请联系管理员并保留原订单", label, safeCode(e.Code))
}

func (e *RequestError) Unwrap() error { return ErrUnavailable }

func requestFailure(operation string, result *core.APIResult, err error) error {
	operation, _ = safeOperation(operation)
	failure := &RequestError{Operation: operation, Code: "SDK_ERROR"}
	if result != nil && result.Response != nil {
		failure.HTTPStatus = result.Response.StatusCode
		if failure.HTTPStatus >= 200 && failure.HTTPStatus < 300 {
			failure.Code = "RESPONSE_CHECK_FAILED"
		}
	}
	var apiError *core.APIError
	var networkError net.Error
	var unknownCA x509.UnknownAuthorityError
	var invalidCertificate x509.CertificateInvalidError
	var wrongHostname x509.HostnameError
	switch {
	case errors.As(err, &apiError):
		failure.Code = safeCode(apiError.Code)
		failure.HTTPStatus = apiError.StatusCode
	case errors.Is(err, context.DeadlineExceeded):
		failure.Code = "TIMEOUT"
	case errors.As(err, &unknownCA), errors.As(err, &invalidCertificate), errors.As(err, &wrongHostname):
		failure.Code = "TLS_ERROR"
	case errors.As(err, &networkError):
		failure.Code = "NETWORK_ERROR"
		if networkError.Timeout() {
			failure.Code = "TIMEOUT"
		}
	}
	if failure.HTTPStatus < 100 || failure.HTTPStatus > 599 {
		failure.HTTPStatus = 0
	}
	slog.Warn("wechat_pay_request_failed", "stage", failure.Operation, "upstream_code", failure.Code, "upstream_status", failure.HTTPStatus)
	return failure
}

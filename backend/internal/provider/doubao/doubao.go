package doubao

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"regexp"
	"strings"

	"github.com/jason127vip-dot/maohai-shuo/backend/internal/config"
	"github.com/jason127vip-dot/maohai-shuo/backend/internal/provider"
)

type Generator struct {
	config config.Doubao
	client *http.Client
}

func New(c config.Doubao, client *http.Client) *Generator {
	return &Generator{config: c, client: client}
}

func (g *Generator) Generate(ctx context.Context, r provider.Request) (provider.Result, error) {
	result := provider.Result{}
	payload := map[string]any{
		"model": r.Model, "prompt": r.Prompt,
		"image": "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(r.Image),
		"size":  g.config.Size, "response_format": "b64_json", "watermark": true, "stream": false,
	}
	if g.config.DisableGroupGeneration {
		payload["sequential_image_generation"] = "disabled"
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return result, &provider.Failure{Code: "REQUEST_ENCODING"}
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(g.config.BaseURL, "/")+"/images/generations", bytes.NewReader(body))
	if err != nil {
		return result, &provider.Failure{Code: "REQUEST_CONFIG"}
	}
	request.Header.Set("Authorization", "Bearer "+g.config.APIKey)
	request.Header.Set("Content-Type", "application/json")
	response, err := g.client.Do(request)
	if err != nil {
		return result, &provider.Failure{Code: "PROVIDER_CONNECTION_UNCERTAIN", Uncertain: true}
	}
	defer response.Body.Close()
	result.RequestID = response.Header.Get("X-Request-Id")
	if result.RequestID == "" {
		result.RequestID = response.Header.Get("X-Tt-Logid")
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		// Retain only bounded diagnostic identifiers, never the upstream message:
		// it may echo the prompt, photo data, or credentials.
		var details struct {
			Error struct {
				Code string `json:"code"`
			} `json:"error"`
		}
		providerCode := ""
		if json.NewDecoder(io.LimitReader(response.Body, 8192)).Decode(&details) == nil {
			providerCode = diagnosticToken(details.Error.Code, g.config.APIKey)
		}
		result.RequestID = diagnosticToken(result.RequestID, g.config.APIKey)
		uncertain := response.StatusCode >= 500 || response.StatusCode == http.StatusRequestTimeout
		code := "PROVIDER_HTTP_ERROR"
		if response.StatusCode == http.StatusForbidden {
			switch providerCode {
			case "AccountOverdueError", "OperationDenied.ServiceOverdue":
				code = provider.CodeInsufficientBalance
			}
		}
		return result, &provider.Failure{Code: code, Uncertain: uncertain, HTTPStatus: response.StatusCode, ProviderCode: providerCode}
	}
	const maxResponse = 40 << 20
	data, err := io.ReadAll(io.LimitReader(response.Body, maxResponse+1))
	if err != nil || len(data) > maxResponse {
		return result, &provider.Failure{Code: "PROVIDER_RESPONSE_UNCERTAIN", Uncertain: true}
	}
	var decoded struct {
		Data []struct {
			Base64 string `json:"b64_json"`
		} `json:"data"`
	}
	if json.Unmarshal(data, &decoded) != nil {
		return result, &provider.Failure{Code: "PROVIDER_RESPONSE_UNCERTAIN", Uncertain: true}
	}
	result.OutputCount = len(decoded.Data)
	// Exactly one image is requested. Unexpected paid output requires review, not retry.
	if len(decoded.Data) != 1 || decoded.Data[0].Base64 == "" {
		return result, &provider.Failure{Code: "PROVIDER_OUTPUT_UNCERTAIN", Uncertain: true}
	}
	result.Image, err = base64.StdEncoding.DecodeString(decoded.Data[0].Base64)
	if err != nil || len(result.Image) == 0 {
		return result, &provider.Failure{Code: "PROVIDER_IMAGE_UNCERTAIN", Uncertain: true}
	}
	return result, nil
}

var diagnosticPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.:-]{0,127}$`)

func diagnosticToken(value, apiKey string) string {
	if !diagnosticPattern.MatchString(value) || (apiKey != "" && strings.Contains(value, apiKey)) {
		return ""
	}
	return value
}

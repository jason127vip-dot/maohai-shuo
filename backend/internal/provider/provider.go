package provider

import "context"

// CodeInsufficientBalance identifies an explicit provider-account billing failure.
// Providers map their own error codes here; quota and rate limits are separate.
const CodeInsufficientBalance = "PROVIDER_INSUFFICIENT_BALANCE"

type Request struct {
	Image  []byte
	Prompt string
	Model  string
}
type Result struct {
	Image       []byte
	RequestID   string
	OutputCount int
}
type Generator interface {
	Generate(context.Context, Request) (Result, error)
}

// Failure only contains sanitized messages. Uncertain means a paid inference may have run.
type Failure struct {
	Code         string
	Uncertain    bool
	HTTPStatus   int
	ProviderCode string
}

func (e *Failure) Error() string { return e.Code }

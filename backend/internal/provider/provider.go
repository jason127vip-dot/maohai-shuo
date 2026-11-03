package provider

import "context"

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
	Code      string
	Uncertain bool
}

func (e *Failure) Error() string { return e.Code }

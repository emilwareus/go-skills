// generic_decorator.go shows an application-level decorator.
package examples

import (
	"context"
	"log"
	"time"
)

type CommandHandler[C any] interface {
	Handle(ctx context.Context, cmd C) error
}

type CommandHandlerFunc[C any] func(ctx context.Context, cmd C) error

func (f CommandHandlerFunc[C]) Handle(ctx context.Context, cmd C) error {
	return f(ctx, cmd)
}

type LoggingCommandHandler[C any] struct {
	base CommandHandler[C]
	log  *log.Logger
	name string
}

func NewLoggingCommandHandler[C any](
	name string,
	base CommandHandler[C],
	logger *log.Logger,
) LoggingCommandHandler[C] {
	if base == nil {
		panic("missing command handler")
	}
	if logger == nil {
		logger = log.Default()
	}
	return LoggingCommandHandler[C]{base: base, log: logger, name: name}
}

func (h LoggingCommandHandler[C]) Handle(ctx context.Context, cmd C) (err error) {
	started := time.Now()
	defer func() {
		h.log.Printf("command=%s duration=%s err=%v", h.name, time.Since(started), err)
	}()
	return h.base.Handle(ctx, cmd)
}

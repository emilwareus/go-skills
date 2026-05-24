// query_handler.go is the read-side counterpart to command_handler.go.
//
// What CQRS buys you here:
//
//   - Different shapes for reads vs writes. The write side loads a
//     full Training aggregate to enforce invariants; the read side
//     returns a flat list of Date DTOs the UI can render directly,
//     with no aggregate loading or invariant code on the read path.
//
//   - Different ports. The query handler depends on
//     AvailableHoursReadModel, which lives in this package. The
//     adapter that implements it can be a separate database, a
//     denormalized table, a cache, or a search index - it doesn't
//     have to be the same store the write side uses.
//
//   - Different change pressure. Read shapes evolve as the UI
//     evolves. Write shapes evolve as business rules evolve. CQRS
//     keeps those changes from blocking each other.
//
// What CQRS does NOT require: two databases, event sourcing, or
// elaborate machinery. The simplest CQRS split is "command
// handlers go through aggregates; query handlers go through
// read-model interfaces that return DTOs". Start there.
package examples

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// The query
//
// Queries are inputs only. Like commands, they're flat data. The
// handler decides what shape to return.
type AvailableHours struct {
	From time.Time
	To   time.Time
}

// Date is the read DTO returned to the caller. Notice it has
// public fields and JSON tags - it's optimized for serialization
// to the UI, not for enforcing invariants. That's the opposite of
// the Hour aggregate from the domain-modeling skill, and that's
// the whole point of CQRS: writes and reads optimize for different
// things.
type Date struct {
	Date  time.Time `json:"date"`
	Hours []Hour    `json:"hours"`
}

type Hour struct {
	Hour      time.Time `json:"hour"`
	Available bool      `json:"available"`
}

// The port
//
// One interface, one method, scoped exactly to what the handler
// needs. The adapter implementing this is free to use whatever
// query strategy fits - a single SELECT, a join across two tables,
// a call to a search service. The handler does not care.
type AvailableHoursReadModel interface {
	AvailableHours(ctx context.Context, from, to time.Time) ([]Date, error)
}

// The handler

type AvailableHoursHandler struct {
	readModel AvailableHoursReadModel
}

func NewAvailableHoursHandler(readModel AvailableHoursReadModel) AvailableHoursHandler {
	if readModel == nil {
		panic("missing AvailableHoursReadModel")
	}
	return AvailableHoursHandler{readModel: readModel}
}

// Handle validates the query shape, then delegates to the read
// model. Notice we DO validate here - "from after to" is a query
// error, and rejecting it before the database call is cheaper and
// easier to test than rejecting it via a SQL constraint.
//
// What we do NOT do: enforce domain invariants. The read path
// returns whatever the read model returns. If a row in the read
// model is stale or denormalized differently from the aggregate,
// that's a read-model consistency concern, not a query-handler
// concern.
func (h AvailableHoursHandler) Handle(ctx context.Context, query AvailableHours) ([]Date, error) {
	if query.From.After(query.To) {
		return nil, ErrInvalidDateRange
	}
	dates, err := h.readModel.AvailableHours(ctx, query.From, query.To)
	if err != nil {
		return nil, fmt.Errorf("read available hours: %w", err)
	}
	return dates, nil
}

// ErrInvalidDateRange is exported so transport adapters can map it
// to a 400 (Bad Request) without string-matching the error message.
// This is the pattern recommended in the errors-observability skill.
var ErrInvalidDateRange = errors.New("invalid date range: from must be on or before to")

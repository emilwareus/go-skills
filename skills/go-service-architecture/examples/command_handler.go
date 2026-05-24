// Package examples mirrors the Training/Trainer domain from the
// Three Dots Labs Wild Workouts posts.
//
// command_handler.go shows the one-handler-per-command shape from
// the "Basic CQRS in Go" post. The "Introducing Clean Architecture"
// post uses a multi-method TrainingService instead; both are shown
// in the articles, but this file is the CQRS-shaped variant.
//
//   - One handler struct per command.
//   - Dependencies are narrow interfaces, named for what the
//     handler needs, defined in this package (NOT in the
//     adapter packages that implement them).
//   - The handler body orchestrates: it loads, calls domain
//     behavior, persists, and translates errors. It does not
//     contain SQL, HTTP, or business rules.
//
// The "dependencies pointing inward" rule shows up concretely here:
// trainingRepository, trainerService, and userService are
// interfaces in *this* package. The adapter that calls a real gRPC
// service implements trainerService; the adapter that talks to
// Firestore implements trainingRepository. Neither is imported
// here — they only have to satisfy the local interface.
package examples

import (
	"context"
	"fmt"
	"time"
)

// ── The command ──────────────────────────────────────────────────
//
// Commands are flat data structures. No methods, no validation
// beyond shape. The handler is responsible for invoking domain
// rules; the command is just the message.
type CancelTraining struct {
	User         AuthUser
	TrainingUUID string
}

// AuthUser stands in for the Wild Workouts auth.User type. It is
// kept local only so this example compiles as a standalone file.
// A request without an authenticated user should never reach the
// handler; that check belongs in the transport layer (middleware)
// which produced this command in the first place.
type AuthUser struct {
	UUID string
	Role string // "trainer" or "attendee"
}

// ── The ports the handler depends on ──────────────────────────────
//
// Three narrow interfaces, each describing exactly what THIS
// handler needs. trainerService and userService are likely also
// used by other handlers — they live alongside this file because
// this package is where they're consumed, but in a larger codebase
// they would each live in the file of the use case that
// established them.

type trainingRepository interface {
	// CancelTraining uses the UpdateFn pattern (see
	// go-persistence-transactions/examples/update_fn.go for the
	// underlying mechanic): the repo loads the Training, hands it
	// to the closure, and saves the result inside one tx. The
	// handler stays unaware of transactions and SQL.
	CancelTraining(
		ctx context.Context,
		trainingUUID string,
		updateFn func(ctx context.Context, t *Training) error,
	) error
}

type userService interface {
	UpdateTrainingBalance(ctx context.Context, userID string, amountChange int) error
}

type trainerService interface {
	CancelTraining(ctx context.Context, trainingTime time.Time) error
}

// Training is the application-layer view of the aggregate. (In a
// strict DDD layout this would be in a domain package and the app
// layer would import it; in the Wild Workouts article the model
// lives in app/.)
type Training struct {
	UUID     string
	UserUUID string
	Time     time.Time
}

func (t Training) CanBeCancelled() bool {
	return t.Time.Sub(time.Now()) > 24*time.Hour
}

// ── The handler ──────────────────────────────────────────────────

type CancelTrainingHandler struct {
	repo           trainingRepository
	userService    userService
	trainerService trainerService
}

// NewCancelTrainingHandler panics on nil dependencies. This is the
// Wild Workouts style: a missing dependency at startup is a wiring
// bug, and a startup panic with a clear message is better than a
// nil-pointer deref later under load. Use this for constructors
// called by main.go; do not panic in business code.
func NewCancelTrainingHandler(
	repo trainingRepository,
	userService userService,
	trainerService trainerService,
) CancelTrainingHandler {
	if repo == nil {
		panic("missing trainingRepository")
	}
	if userService == nil {
		panic("missing userService")
	}
	if trainerService == nil {
		panic("missing trainerService")
	}
	return CancelTrainingHandler{
		repo:           repo,
		userService:    userService,
		trainerService: trainerService,
	}
}

// Handle is the heart of the use case. Read it top-to-bottom and
// you'll see exactly:
//
//   - Authorization: "is this user allowed to cancel this training?"
//   - Domain decision: "what's the refund policy?" via CanBeCancelled
//   - Side-effects through ports: user balance adjustment, trainer
//     schedule update.
//
// What's NOT here: SQL, HTTP, transaction handling, logging
// boilerplate, retry loops. Each of those belongs at a different
// layer.
func (h CancelTrainingHandler) Handle(ctx context.Context, cmd CancelTraining) error {
	return h.repo.CancelTraining(ctx, cmd.TrainingUUID, func(ctx context.Context, training *Training) error {
		// Authorization: trainers can cancel anyone's training;
		// attendees can only cancel their own. The check uses
		// already-loaded data (training.UserUUID) so it doesn't
		// have to make another query.
		if cmd.User.Role != "trainer" && training.UserUUID != cmd.User.UUID {
			return fmt.Errorf("user %q cannot cancel training of user %q", cmd.User.UUID, training.UserUUID)
		}

		// Refund policy. The "right" amount depends on who's
		// cancelling and how close to the start. Notice this is
		// readable as business intent — not buried in a SQL CASE.
		var balanceDelta int
		switch {
		case training.CanBeCancelled():
			balanceDelta = 1 // refund the credit
		case cmd.User.Role == "trainer":
			balanceDelta = 2 // trainer-initiated late cancel: bonus credit
		default:
			balanceDelta = 0 // late cancel by attendee: no refund
		}

		if balanceDelta != 0 {
			if err := h.userService.UpdateTrainingBalance(ctx, training.UserUUID, balanceDelta); err != nil {
				return fmt.Errorf("update training balance: %w", err)
			}
		}

		if err := h.trainerService.CancelTraining(ctx, training.Time); err != nil {
			return fmt.Errorf("cancel trainer schedule: %w", err)
		}
		return nil
	})
}

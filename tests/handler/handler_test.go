// Package handler demonstrates testing an application command
// handler with fakes — the "fakes over mocks" rule from the
// go-testing skill. The handler under test mirrors
// CancelTrainingHandler from
// skills/go-service-architecture/examples/command_handler.go.
//
// Why a fake repository here instead of a generated mock:
//
//   - The repository interface is narrow (one method). A
//     hand-written fake is 15 lines.
//
//   - We want to assert observable outcomes (was the training
//     cancelled? was the user's balance updated?), not "method
//     X was called with arg Y". A fake records state; a mock
//     records calls. State assertions break less often on
//     refactor.
//
//   - Fakes can implement behavior (e.g. "return NotFound for
//     this UUID"), which is exactly what real-ish testing needs.
//     Mocks force every behavior to be expressed as expected
//     calls and configured returns, which gets verbose fast.
//
// Lives in tests/ at repo root rather than skills/ so it is not
// shipped with any skill — see top-level README for the rationale.
package handler

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCancelTrainingHandler(t *testing.T) {
	t.Run("attendee cancels their own training in time", func(t *testing.T) {
		fakes := newFakes()
		fakes.repo.training = &Training{
			UUID:     "training-1",
			UserUUID: "user-1",
			Time:     time.Now().Add(48 * time.Hour),
		}
		h := NewCancelTrainingHandler(fakes.repo, fakes.users, fakes.trainer)

		err := h.Handle(context.Background(), CancelTraining{
			User:         User{UUID: "user-1", Role: "attendee"},
			TrainingUUID: "training-1",
		})
		require.NoError(t, err)

		// Observable outcomes: training cancelled in repo,
		// user refunded one credit, trainer schedule updated.
		assert.True(t, fakes.repo.cancelled)
		assert.Equal(t, map[string]int{"user-1": 1}, fakes.users.balanceChanges)
		assert.Len(t, fakes.trainer.cancellations, 1)
	})

	t.Run("attendee cannot cancel someone else's training", func(t *testing.T) {
		fakes := newFakes()
		fakes.repo.training = &Training{
			UUID:     "training-2",
			UserUUID: "user-1",
			Time:     time.Now().Add(48 * time.Hour),
		}
		h := NewCancelTrainingHandler(fakes.repo, fakes.users, fakes.trainer)

		err := h.Handle(context.Background(), CancelTraining{
			User:         User{UUID: "user-2", Role: "attendee"},
			TrainingUUID: "training-2",
		})
		require.Error(t, err)

		// Side-effect assertion: nothing happened.
		assert.False(t, fakes.repo.cancelled)
		assert.Empty(t, fakes.users.balanceChanges)
		assert.Empty(t, fakes.trainer.cancellations)
	})

	t.Run("late cancellation by trainer gives bonus credit", func(t *testing.T) {
		fakes := newFakes()
		fakes.repo.training = &Training{
			UUID:     "training-3",
			UserUUID: "user-1",
			Time:     time.Now().Add(2 * time.Hour), // < 24h
		}
		h := NewCancelTrainingHandler(fakes.repo, fakes.users, fakes.trainer)

		err := h.Handle(context.Background(), CancelTraining{
			User:         User{UUID: "trainer-1", Role: "trainer"},
			TrainingUUID: "training-3",
		})
		require.NoError(t, err)
		assert.Equal(t, 2, fakes.users.balanceChanges["user-1"])
	})
}

// ── Fakes ─────────────────────────────────────────────────────────
//
// Each fake satisfies one of the handler's narrow interfaces and
// records what happened in plain Go fields. Tests assert on those
// fields. No expectations to configure; no order to declare.

type fakeRepo struct {
	training  *Training
	cancelled bool
	err       error
}

// CancelTraining implements the UpdateFn-style port. The closure
// is the handler's actual logic; we run it against our in-memory
// training and record the result.
func (f *fakeRepo) CancelTraining(
	ctx context.Context,
	trainingUUID string,
	updateFn func(ctx context.Context, t *Training) error,
) error {
	if f.err != nil {
		return f.err
	}
	if f.training == nil || f.training.UUID != trainingUUID {
		return errors.New("training not found")
	}
	if err := updateFn(ctx, f.training); err != nil {
		return err
	}
	f.cancelled = true
	return nil
}

type fakeUsers struct {
	balanceChanges map[string]int
}

func (f *fakeUsers) UpdateTrainingBalance(_ context.Context, userID string, change int) error {
	f.balanceChanges[userID] += change
	return nil
}

type fakeTrainer struct {
	cancellations []time.Time
}

func (f *fakeTrainer) CancelTraining(_ context.Context, at time.Time) error {
	f.cancellations = append(f.cancellations, at)
	return nil
}

type fakeBundle struct {
	repo    *fakeRepo
	users   *fakeUsers
	trainer *fakeTrainer
}

func newFakes() fakeBundle {
	return fakeBundle{
		repo:    &fakeRepo{},
		users:   &fakeUsers{balanceChanges: map[string]int{}},
		trainer: &fakeTrainer{},
	}
}

// ── Handler under test (stubs of the real example) ────────────────
//
// In a real test file these would come from the handler package
// under test. See
// skills/go-service-architecture/examples/command_handler.go for
// the annotated production version.

type User struct{ UUID, Role string }

type Training struct {
	UUID     string
	UserUUID string
	Time     time.Time
}

func (t Training) CanBeCancelled() bool { return t.Time.Sub(time.Now()) > 24*time.Hour }

type CancelTraining struct {
	User         User
	TrainingUUID string
}

type trainingRepository interface {
	CancelTraining(ctx context.Context, trainingUUID string, updateFn func(ctx context.Context, t *Training) error) error
}
type userService interface {
	UpdateTrainingBalance(ctx context.Context, userID string, change int) error
}
type trainerService interface {
	CancelTraining(ctx context.Context, at time.Time) error
}

type CancelTrainingHandler struct {
	repo    trainingRepository
	users   userService
	trainer trainerService
}

func NewCancelTrainingHandler(r trainingRepository, u userService, t trainerService) CancelTrainingHandler {
	return CancelTrainingHandler{repo: r, users: u, trainer: t}
}

func (h CancelTrainingHandler) Handle(ctx context.Context, cmd CancelTraining) error {
	return h.repo.CancelTraining(ctx, cmd.TrainingUUID, func(ctx context.Context, tr *Training) error {
		if cmd.User.Role != "trainer" && tr.UserUUID != cmd.User.UUID {
			return errors.New("unauthorized")
		}
		var delta int
		switch {
		case tr.CanBeCancelled():
			delta = 1
		case cmd.User.Role == "trainer":
			delta = 2
		}
		if delta != 0 {
			if err := h.users.UpdateTrainingBalance(ctx, tr.UserUUID, delta); err != nil {
				return err
			}
		}
		return h.trainer.CancelTraining(ctx, tr.Time)
	})
}

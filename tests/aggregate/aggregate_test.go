// Package aggregate demonstrates table-test and fixture-helper
// patterns from the go-testing skill, applied to a domain aggregate
// similar to the Hour aggregate in skills/go-domain-modeling/examples/.
//
// These tests live at the repository root in tests/ rather than in
// skills/ so they are NOT shipped when an agent installs the skill
// via `npx skills add`. They exist to:
//
//  1. Demonstrate the testing patterns the go-testing skill
//     describes, with real code an engineer can copy.
//  2. Get continuously vetted by `go test ./...` in CI so the
//     patterns don't drift.
//
// aggregate_test.go shows the most important testing principle:
// pure domain code is the easiest code in the codebase to test.
// Take advantage of it.
package aggregate

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── Table tests: the workhorse pattern ─────────────────────────────
//
// Use table tests when the cases share setup and assertions but
// vary on input/output. Each case gets a name that reads as a
// sentence about the behavior — so a failed test name alone tells
// you what regressed.
func TestHour_ScheduleTraining(t *testing.T) {
	now := mustParse("2026-06-01T10:00:00Z")

	tests := []struct {
		name          string
		startingHour  *Hour
		wantErr       error
		wantScheduled bool
	}{
		{
			name:          "available hour becomes scheduled",
			startingHour:  availableHour(t, now),
			wantScheduled: true,
		},
		{
			name:         "already scheduled hour is rejected",
			startingHour: scheduledHour(t, now),
			wantErr:      ErrHourNotAvailable,
		},
		{
			name:         "not-available hour is rejected",
			startingHour: notAvailableHour(t, now),
			wantErr:      ErrHourNotAvailable,
		},
	}

	for i := range tests {
		// Capturing the loop variable into a local is the
		// idiomatic shape for parallel subtests across Go
		// versions. It avoids the classic "all subtests see the
		// last case" bug on older Go.
		tc := tests[i]
		t.Run(tc.name, func(t *testing.T) {
			err := tc.startingHour.ScheduleTraining()

			// Use ErrorIs for sentinel comparison — string
			// matching the error message is fragile and breaks
			// the moment someone reformats the wrap.
			if tc.wantErr != nil {
				assert.ErrorIs(t, err, tc.wantErr)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tc.wantScheduled, tc.startingHour.HasTrainingScheduled())
		})
	}
}

// ── Fixture helpers ───────────────────────────────────────────────
//
// The helpers below take *testing.T so they can call t.Helper()
// (which makes failure lines point at the test, not the helper)
// and t.Fatal on construction errors. They construct fresh objects
// so each case gets independent state.

func availableHour(t *testing.T, at time.Time) *Hour {
	t.Helper()
	h, err := NewAvailableHour(at)
	require.NoError(t, err)
	return h
}

func notAvailableHour(t *testing.T, at time.Time) *Hour {
	t.Helper()
	h, err := NewNotAvailableHour(at)
	require.NoError(t, err)
	return h
}

func scheduledHour(t *testing.T, at time.Time) *Hour {
	t.Helper()
	h := availableHour(t, at)
	require.NoError(t, h.ScheduleTraining())
	return h
}

func mustParse(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic(err)
	}
	return t
}

// ── Aggregate under test ──────────────────────────────────────────
//
// Inlined here so the test file reads standalone. In a real codebase
// this code would come from the domain package being tested (see
// skills/go-domain-modeling/examples/hour_aggregate.go for the
// annotated production version).

var (
	ErrHourNotAvailable    = errors.New("hour not available")
	ErrNoTrainingScheduled = errors.New("no training scheduled")
)

type Availability int

const (
	Available Availability = iota + 1
	TrainingScheduled
	NotAvailable
)

type Hour struct {
	hour         time.Time
	availability Availability
}

func NewAvailableHour(h time.Time) (*Hour, error) {
	if h.Minute() != 0 {
		return nil, errors.New("must be on the hour")
	}
	return &Hour{hour: h, availability: Available}, nil
}

func NewNotAvailableHour(h time.Time) (*Hour, error) {
	if h.Minute() != 0 {
		return nil, errors.New("must be on the hour")
	}
	return &Hour{hour: h, availability: NotAvailable}, nil
}

func (h *Hour) ScheduleTraining() error {
	if h.availability != Available {
		return ErrHourNotAvailable
	}
	h.availability = TrainingScheduled
	return nil
}

func (h *Hour) HasTrainingScheduled() bool { return h.availability == TrainingScheduled }
func (h *Hour) IsAvailable() bool          { return h.availability == Available }

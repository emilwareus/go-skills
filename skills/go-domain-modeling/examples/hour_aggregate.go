// hour_aggregate.go demonstrates the Hour aggregate pattern.
//
// The pattern in three sentences:
//
//  1. All fields are unexported. Outside callers can't reach in
//     and break invariants by direct assignment.
//  2. State transitions go through methods named after business
//     actions (ScheduleTraining, CancelTraining, MakeAvailable),
//     each of which checks that the transition is currently legal.
//  3. Queries (IsAvailable, HasTrainingScheduled) read state
//     without changing it, so they can be called freely.
//
// What this prevents: the "anemic domain" trap where Hour would be
// a public struct with public fields, and the rules about *when*
// you can schedule a training would live in three different
// handlers (HTTP, gRPC, worker) - none of which agree.
package examples

import (
	"errors"
	"time"
)

// Sentinel errors for state transition failures. Callers compare
// with errors.Is and map to HTTP 409 / gRPC FailedPrecondition.
var (
	ErrHourNotAvailable    = errors.New("hour not available")
	ErrNoTrainingScheduled = errors.New("no training scheduled")
)

// Availability is a small enum that lives next to the aggregate
// that owns it. Use a defined type (not a bare int or string) so
// the compiler stops you from passing the wrong value.
type Availability int

const (
	Available Availability = iota + 1
	TrainingScheduled
	NotAvailable
)

// Hour is the aggregate. Two fields, both unexported.
type Hour struct {
	hour         time.Time
	availability Availability
}

// NewAvailableHour is the constructor for the common case: a freshly
// minted bookable hour. Validation lives here (validateTime, defined
// below), so a Hour value handed back from this function is always
// safe to use.
//
// Multiple small named constructors read better than one constructor
// with an Availability parameter the caller has to remember the
// meaning of.
func NewAvailableHour(hour time.Time) (*Hour, error) {
	if err := validateTime(hour); err != nil {
		return nil, err
	}
	return &Hour{
		hour:         hour,
		availability: Available,
	}, nil
}

func NewNotAvailableHour(hour time.Time) (*Hour, error) {
	if err := validateTime(hour); err != nil {
		return nil, err
	}
	return &Hour{
		hour:         hour,
		availability: NotAvailable,
	}, nil
}

// ScheduleTraining is the kind of method that makes this pattern
// pay off. It encodes a business rule - "you can only schedule a
// training on an available hour" - at the *only* point where a
// transition to TrainingScheduled is possible. There is no way to
// bypass it short of editing this file.
func (h *Hour) ScheduleTraining() error {
	if !h.IsAvailable() {
		return ErrHourNotAvailable
	}
	h.availability = TrainingScheduled
	return nil
}

func (h *Hour) CancelTraining() error {
	if !h.HasTrainingScheduled() {
		return ErrNoTrainingScheduled
	}
	h.availability = Available
	return nil
}

func (h *Hour) MakeAvailable() error {
	if h.HasTrainingScheduled() {
		// Refuse to silently cancel someone's training as a side
		// effect of an administrative status change. Force the
		// caller to use CancelTraining explicitly first.
		return ErrHourNotAvailable
	}
	h.availability = Available
	return nil
}

// Queries: read-only, safe to call any number of times. Naming them
// IsX / HasX (not Get + bool) reads as English at the call site.
func (h *Hour) IsAvailable() bool          { return h.availability == Available }
func (h *Hour) HasTrainingScheduled() bool { return h.availability == TrainingScheduled }

// Accessors for fields callers genuinely need. Don't auto-generate
// a getter for every field - only expose what callers actually use.
// If a getter has no caller, deleting it makes the API smaller.
func (h *Hour) Time() time.Time            { return h.hour }
func (h *Hour) Availability() Availability { return h.availability }

// validateTime is the domain rule for "what counts as a valid
// scheduling slot". Kept private so the constructors are the only
// callers - there is no need to expose it.
func validateTime(t time.Time) error {
	if t.IsZero() {
		return errors.New("zero time is not a valid hour")
	}
	if t.Minute() != 0 || t.Second() != 0 || t.Nanosecond() != 0 {
		return errors.New("hour must be aligned to an exact hour")
	}
	return nil
}

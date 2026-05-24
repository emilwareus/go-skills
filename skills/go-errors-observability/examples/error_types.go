// error_types.go shows a small slug-error contract based on the
// Wild Workouts codebase. It is intentionally narrow: stable slugs,
// human messages, and a coarse error type for boundary mapping.
package examples

import (
	"errors"
	"fmt"
)

// Sentinels are still useful when the caller only needs a stable
// category and no extra fields.
var ErrTrainingNotFound = errors.New("training not found")

type ErrorType struct {
	t string
}

var (
	ErrorTypeUnknown        = ErrorType{"unknown"}
	ErrorTypeAuthorization  = ErrorType{"authorization"}
	ErrorTypeIncorrectInput = ErrorType{"incorrect-input"}
)

// SlugError is the typed error carried from application/domain code
// to the transport boundary. The slug is stable API surface; the
// message is for logs and operators.
type SlugError struct {
	message   string
	slug      string
	errorType ErrorType
}

func (s SlugError) Error() string {
	return s.message
}

func (s SlugError) Slug() string {
	return s.slug
}

func (s SlugError) ErrorType() ErrorType {
	return s.errorType
}

func NewSlugError(slug string, message string) SlugError {
	return SlugError{
		message:   message,
		slug:      slug,
		errorType: ErrorTypeUnknown,
	}
}

func NewAuthorizationError(slug string, message string) SlugError {
	return SlugError{
		message:   message,
		slug:      slug,
		errorType: ErrorTypeAuthorization,
	}
}

func NewIncorrectInputError(slug string, message string) SlugError {
	return SlugError{
		message:   message,
		slug:      slug,
		errorType: ErrorTypeIncorrectInput,
	}
}

func validateAvailableHours(fromAfterTo bool) error {
	if fromAfterTo {
		return NewIncorrectInputError("date-from-after-date-to", "Date from after date to")
	}
	return nil
}

func wrapApplicationError(trainingUUID string, err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, ErrTrainingNotFound) {
		return err
	}
	return fmt.Errorf("cancel training %s: %w", trainingUUID, err)
}

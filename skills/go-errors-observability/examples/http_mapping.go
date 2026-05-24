// http_mapping.go mirrors the Wild Workouts httperr helper shape:
// ports call small helpers such as Unauthorised, BadRequest,
// InternalError, or RespondWithSlugError instead of spreading
// status-code decisions across handlers.
package examples

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
)

func InternalError(slug string, err error, w http.ResponseWriter, r *http.Request) {
	httpRespondWithError(err, slug, w, r, "Internal server error", http.StatusInternalServerError)
}

func Unauthorised(slug string, err error, w http.ResponseWriter, r *http.Request) {
	httpRespondWithError(err, slug, w, r, "Unauthorised", http.StatusUnauthorized)
}

func BadRequest(slug string, err error, w http.ResponseWriter, r *http.Request) {
	httpRespondWithError(err, slug, w, r, "Bad request", http.StatusBadRequest)
}

func RespondWithSlugError(err error, w http.ResponseWriter, r *http.Request) {
	var slugError SlugError
	if !errors.As(err, &slugError) {
		InternalError("internal-server-error", err, w, r)
		return
	}

	switch slugError.ErrorType() {
	case ErrorTypeAuthorization:
		Unauthorised(slugError.Slug(), err, w, r)
	case ErrorTypeIncorrectInput:
		BadRequest(slugError.Slug(), err, w, r)
	default:
		InternalError(slugError.Slug(), err, w, r)
	}
}

func httpRespondWithError(err error, slug string, w http.ResponseWriter, r *http.Request, logMsg string, status int) {
	if status >= http.StatusInternalServerError {
		log.Printf("%s: slug=%s method=%s path=%s err=%v", logMsg, slug, r.Method, r.URL.Path, err)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(ErrorResponse{Slug: slug})
}

type ErrorResponse struct {
	Slug string `json:"slug"`
}

// Example handler shape:
//
//	func getTrainings(w http.ResponseWriter, r *http.Request) {
//	    trainings, err := app.TrainingsForUser.Handle(r.Context(), query)
//	    if err != nil {
//	        RespondWithSlugError(err, w, r)
//	        return
//	    }
//	    _ = json.NewEncoder(w).Encode(trainings)
//	}

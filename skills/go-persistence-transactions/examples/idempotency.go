// idempotency.go is a supplementary example: the Three Dots Labs
// article on database transactions mentions idempotency in passing
// but does not show code for it. The skill recommends the pattern
// for retried commands (clients, queues, webhooks), so this file
// gives a reference implementation in the same domain as the rest of
// the folder — UsePointsAsDiscount, which is exactly the kind of
// command you do not want to apply twice if a client retries.
//
// The pattern: store the idempotency key, the request hash, and the
// final outcome. On retry:
//
//   - Same key, same payload → return the stored outcome, do nothing.
//   - Same key, different payload → reject; that's a client bug.
//   - New key → claim it, do the work, persist the outcome.
//
// The race-free claim relies on a unique constraint and
// INSERT ... ON CONFLICT DO NOTHING, so two concurrent callers
// cannot both "win".
package examples

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
)

// Schema:
//
//	CREATE TABLE idempotency_keys (
//	    key          TEXT PRIMARY KEY,
//	    scope        TEXT NOT NULL,         -- e.g. "use_points_as_discount"
//	    request_hash BYTEA NOT NULL,        -- sha256 of the request body
//	    response     JSONB NOT NULL,        -- stored outcome to replay on retry
//	    status_code  INT   NOT NULL,        -- 0 = in-flight; >0 = finished
//	    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
//	);
//
// The key being the PRIMARY KEY is what gives us atomic first-writer-wins
// on INSERT. The scope column lets the same key live in two different
// flows without colliding (refund vs. charge, etc.).

// IdempotencyResult is what we replay to retrying callers. For the
// UsePointsAsDiscount command this is typically just an HTTP status
// and a small JSON body, but the shape is general.
type IdempotencyResult struct {
	StatusCode int
	Body       json.RawMessage
}

var ErrKeyConflict = errors.New("idempotency key reused with different request payload")

type IdempotencyStore struct {
	db *sql.DB
}

func NewIdempotencyStore(db *sql.DB) *IdempotencyStore { return &IdempotencyStore{db: db} }

// Begin attempts to claim the key.
//
// Returns:
//
//   - (nil, false, nil): the key is fresh. The caller owns it and
//     must run the real command (e.g. UsePointsAsDiscountHandler.Handle)
//     and then call Finish exactly once.
//   - (result, true, nil): the key has been used before with the
//     same payload. Skip the command and reply with result.
//   - (_, _, ErrKeyConflict): the key was used before with a
//     different payload. Reject with 4xx — this is a client bug.
func (s *IdempotencyStore) Begin(ctx context.Context, key, scope string, requestBody []byte) (*IdempotencyResult, bool, error) {
	hash := sha256.Sum256(requestBody)
	hashHex := hex.EncodeToString(hash[:])

	// Atomic claim. status_code = 0 marks the row as in-flight;
	// Finish will write the real status.
	res, err := s.db.ExecContext(ctx, `
		INSERT INTO idempotency_keys (key, scope, request_hash, response, status_code)
		VALUES ($1, $2, decode($3, 'hex'), '{}'::jsonb, 0)
		ON CONFLICT (key) DO NOTHING
	`, key, scope, hashHex)
	if err != nil {
		return nil, false, fmt.Errorf("claim idempotency key: %w", err)
	}
	rows, _ := res.RowsAffected()
	if rows == 1 {
		return nil, false, nil // we got the key
	}

	// Someone else owns the key. Inspect the existing row.
	var (
		storedHashHex string
		response      json.RawMessage
		status        int
	)
	err = s.db.QueryRowContext(ctx, `
		SELECT encode(request_hash, 'hex'), response, status_code
		FROM idempotency_keys
		WHERE key = $1 AND scope = $2
	`, key, scope).Scan(&storedHashHex, &response, &status)
	if err != nil {
		return nil, false, fmt.Errorf("read idempotency key: %w", err)
	}

	if storedHashHex != hashHex {
		return nil, true, ErrKeyConflict
	}
	if status == 0 {
		// Another caller is mid-flight. Two reasonable choices:
		// (a) tell the client to retry shortly (simple, what we do here);
		// (b) block on a row lock until the in-flight call finishes.
		return nil, true, errors.New("idempotency key in flight; retry later")
	}

	return &IdempotencyResult{StatusCode: status, Body: response}, true, nil
}

// Finish records the outcome so future callers with the same key
// replay it. Must be called exactly once per successful Begin.
func (s *IdempotencyStore) Finish(ctx context.Context, key string, result IdempotencyResult) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE idempotency_keys
		SET response = $1, status_code = $2
		WHERE key = $3
	`, result.Body, result.StatusCode, key)
	if err != nil {
		return fmt.Errorf("finish idempotency key: %w", err)
	}
	return nil
}

// HTTP wrapper around the article's UsePointsAsDiscountHandler:
//
//	func (h *Endpoint) ServeHTTP(w http.ResponseWriter, r *http.Request) {
//	    key := r.Header.Get("Idempotency-Key")
//	    body, _ := io.ReadAll(r.Body)
//
//	    prior, hit, err := h.keys.Begin(r.Context(), key, "use_points_as_discount", body)
//	    if errors.Is(err, ErrKeyConflict) { http.Error(w, "key reused", 422); return }
//	    if err != nil { http.Error(w, err.Error(), 500); return }
//	    if hit { w.WriteHeader(prior.StatusCode); w.Write(prior.Body); return }
//
//	    var cmd UsePointsAsDiscount
//	    _ = json.Unmarshal(body, &cmd)
//	    if err := h.handler.Handle(r.Context(), cmd); err != nil {
//	        _ = h.keys.Finish(r.Context(), key, IdempotencyResult{StatusCode: 500, Body: errorBody(err)})
//	        http.Error(w, err.Error(), 500)
//	        return
//	    }
//	    _ = h.keys.Finish(r.Context(), key, IdempotencyResult{StatusCode: 200, Body: []byte(`{"ok":true}`)})
//	    w.WriteHeader(200)
//	}

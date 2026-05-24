// handler_returns_events.go mirrors the outbox pattern shown in the
// Three Dots Labs "Distributed Transactions in Go" article.
//
// The article evolves the UsePointsAsDiscount handler through three
// versions:
//
//  1. Distributed monolith: handler calls UserRepository.UpdateByID,
//     then synchronously calls ordersService.AddDiscount over HTTP.
//     Two services, no shared transaction, partial-failure window.
//
//  2. Event-driven: handler updates the user, then publishes a
//     PointsUsedForDiscount event. Better, but the "publish to
//     broker" step happens AFTER the DB commit — a crash in between
//     loses the event.
//
//  3. Outbox: the UpdateByID closure now returns ([]any) of events.
//     The repository inserts those events into the outbox table in
//     the same transaction as the user/discount update. Either
//     everything commits or nothing does.
//
// This file shows the article's exact version 3 shape. The repository
// side lives in outbox/repository.go.
package examples

import (
	"context"
	"fmt"
)

// PointsUsedForDiscount is the event the article publishes. The
// article explicitly calls this name poorly designed: it leaks the
// orders-service discount use case into the users domain instead of
// naming only the fact that points were used. The code keeps the
// name to match the article verbatim.
type PointsUsedForDiscount struct {
	UserID int `json:"user_id"`
	Points int `json:"points"`
}

// EventPublisher is the narrow interface the handler depends on. In
// the version-2 (non-outbox) flow this is wired to a Watermill
// publisher directly; in the version-3 (outbox) flow the
// "publisher" the repository uses internally is a Watermill SQL
// Pub/Sub bound to the active *sql.Tx.
type EventPublisher interface {
	Publish(ctx context.Context, event any) error
}

// ── Version 1 (anti-pattern): distributed monolith ─────────────────
//
// The article calls this out as the wrong starting point: two
// services, two transactions, no atomicity. Reproduced here in a
// comment so you can see what we're moving away from.
//
//	func (h UsePointsAsDiscountHandler) Handle(ctx context.Context, cmd UsePointsAsDiscount) error {
//	    err := h.userRepository.UpdateByID(ctx, cmd.UserID, func(user *User) (bool, error) {
//	        return true, user.UsePoints(cmd.Points)
//	    })
//	    if err != nil {
//	        return fmt.Errorf("could not update user: %w", err)
//	    }
//	    err = h.ordersService.AddDiscount(ctx, cmd.UserID, cmd.Points)
//	    if err != nil {
//	        // user was already debited; the discount call failed.
//	        // Now what? There is no clean recovery here.
//	        return fmt.Errorf("could not add discount: %w", err)
//	    }
//	    return nil
//	}

// ── Version 2: event-driven, publish-after-commit ──────────────────
//
// Better than version 1: the orders service is decoupled and can be
// down without breaking the points command. But the publish happens
// after the DB commit, so a crash between commit and publish loses
// the event silently. The article uses this as the bridge to
// version 3.
type UsePointsAsDiscountHandlerV2 struct {
	userRepository UserRepository
	eventPublisher EventPublisher
}

func (h UsePointsAsDiscountHandlerV2) Handle(ctx context.Context, cmd UsePointsAsDiscount) error {
	err := h.userRepository.UpdateByID(ctx, cmd.UserID, func(user *User) (bool, error) {
		err := user.UsePoints(cmd.Points)
		if err != nil {
			return false, err
		}
		return true, nil
	})
	if err != nil {
		return fmt.Errorf("could not update user: %w", err)
	}

	event := PointsUsedForDiscount{
		UserID: cmd.UserID,
		Points: cmd.Points,
	}
	err = h.eventPublisher.Publish(ctx, event)
	if err != nil {
		return fmt.Errorf("could not publish event: %w", err)
	}
	return nil
}

// ── Version 3 (recommended): outbox via UpdateFn that returns events ─
//
// The closure now returns (bool, []any, error). The bool still means
// "did anything change?"; the []any is the list of events the
// aggregate decided occurred during this command. The repository
// persists the aggregate AND inserts each event into the outbox
// table — both inside the same transaction.
//
// The handler is back to a single call. It does not talk to the
// broker, does not import Watermill, and cannot publish an event
// that didn't commit.
type UsePointsAsDiscountHandler struct {
	userRepository OutboxUserRepository
}

// OutboxUserRepository is the UpdateFn variant used by the outbox
// pattern. The only difference from the article's plain
// UserRepository is the extra []any return value.
type OutboxUserRepository interface {
	UpdateByID(
		ctx context.Context,
		userID int,
		updateFn func(user *User) (bool, []any, error),
	) error
}

func (h UsePointsAsDiscountHandler) Handle(ctx context.Context, cmd UsePointsAsDiscount) error {
	return h.userRepository.UpdateByID(ctx, cmd.UserID, func(user *User) (bool, []any, error) {
		err := user.UsePoints(cmd.Points)
		if err != nil {
			return false, nil, err
		}
		event := PointsUsedForDiscount{
			UserID: cmd.UserID,
			Points: cmd.Points,
		}
		return true, []any{event}, nil
	})
}

// ── Receiving side (orders service) ────────────────────────────────
//
// The orders service subscribes to PointsUsedForDiscount and
// translates it into its own internal AddDiscount command. Mapping
// events to commands keeps domain logic in the command handler and
// the Watermill-touching code thin.

type AddDiscount struct {
	UserID   int
	Discount int
}

type AddDiscountHandler interface {
	Handle(ctx context.Context, cmd AddDiscount) error
}

type OnPointsUsedForDiscountHandler struct {
	addDiscountHandler AddDiscountHandler
}

func (h OnPointsUsedForDiscountHandler) Handle(ctx context.Context, event *PointsUsedForDiscount) error {
	cmd := AddDiscount{
		UserID:   event.UserID,
		Discount: event.Points,
	}
	return h.addDiscountHandler.Handle(ctx, cmd)
}

// User is the same aggregate as the persistence skill's examples;
// the import would normally come from that package.
type User struct {
	id        int
	email     string
	points    int
	discounts *Discounts
}

func (u *User) ID() int               { return u.id }
func (u *User) Email() string         { return u.email }
func (u *User) Points() int           { return u.points }
func (u *User) Discounts() *Discounts { return u.discounts }

type Discounts struct{ nextOrderDiscount int }

func (c *Discounts) NextOrderDiscount() int { return c.nextOrderDiscount }

func (u *User) UsePoints(points int) error {
	if points <= 0 {
		return fmt.Errorf("points must be greater than 0")
	}
	if u.points < points {
		return fmt.Errorf("not enough points")
	}
	u.points -= points
	u.discounts.nextOrderDiscount += points
	return nil
}

// UserRepository (non-outbox variant), kept here so version 2 above
// compiles in isolation.
type UserRepository interface {
	UpdateByID(ctx context.Context, userID int, updateFn func(user *User) (bool, error)) error
}

// UsePointsAsDiscount is the command shared with the persistence
// skill's examples.
type UsePointsAsDiscount struct {
	UserID int
	Points int
}

// handler_returns_events.go shows the recommended outbox handler
// shape: the command mutates the aggregate and returns the domain
// events that occurred. The repository stores both the aggregate
// changes and the event rows in one database transaction.
package examples

import (
	"context"
	"fmt"
)

// PointsUsed names the publishing-domain fact. The users domain says
// that points were used; the orders domain decides what that means
// for discounts.
type PointsUsed struct {
	UserID int `json:"user_id"`
	Points int `json:"points"`
}

type UsePointsAsDiscountHandler struct {
	userRepository OutboxUserRepository
}

// OutboxUserRepository is the UpdateFn variant used by the outbox
// pattern. The callback returns the list of events that should be
// stored in the same transaction as the aggregate update.
type OutboxUserRepository interface {
	UpdateByID(
		ctx context.Context,
		userID int,
		updateFn func(user *User) (bool, []any, error),
	) error
}

func (h UsePointsAsDiscountHandler) Handle(ctx context.Context, cmd UsePointsAsDiscount) error {
	return h.userRepository.UpdateByID(ctx, cmd.UserID, func(user *User) (bool, []any, error) {
		if err := user.UsePoints(cmd.Points); err != nil {
			return false, nil, err
		}
		event := PointsUsed{
			UserID: cmd.UserID,
			Points: cmd.Points,
		}
		return true, []any{event}, nil
	})
}

// Receiving side: the orders service subscribes to PointsUsed and
// translates it into its own internal command. Domain behavior stays
// in command handlers; Watermill-facing code stays thin.

type AddDiscount struct {
	UserID   int
	Discount int
}

type AddDiscountHandler interface {
	Handle(ctx context.Context, cmd AddDiscount) error
}

type OnPointsUsedHandler struct {
	addDiscountHandler AddDiscountHandler
}

func (h OnPointsUsedHandler) Handle(ctx context.Context, event *PointsUsed) error {
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

// UsePointsAsDiscount is the command that spends points in the users
// domain. The event name remains the domain fact: PointsUsed.
type UsePointsAsDiscount struct {
	UserID int
	Points int
}

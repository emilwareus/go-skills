// repository_security.go shows the secure-by-design repository shape:
// protected reads and updates receive the domain user and enforce the
// visibility rule before returning or mutating an aggregate.
package examples

import (
	"context"
	"errors"
	"fmt"
	"time"
)

type TrainingUserType string

const (
	TrainingUserAttendee TrainingUserType = "attendee"
	TrainingUserTrainer  TrainingUserType = "trainer"
)

type TrainingUser struct {
	uuid string
	typ  TrainingUserType
}

func NewTrainingUser(uuid string, typ TrainingUserType) TrainingUser {
	return TrainingUser{uuid: uuid, typ: typ}
}

func (u TrainingUser) UUID() string {
	return u.uuid
}

func (u TrainingUser) Type() TrainingUserType {
	return u.typ
}

type Training struct {
	uuid     string
	userUUID string
	time     time.Time
}

func (t Training) UUID() string {
	return t.uuid
}

func (t Training) UserUUID() string {
	return t.userUUID
}

func (t Training) Time() time.Time {
	return t.time
}

type ForbiddenToSeeTrainingError struct {
	RequestingUserUUID string
	TrainingOwnerUUID  string
}

func (e ForbiddenToSeeTrainingError) Error() string {
	return fmt.Sprintf(
		"user %q can't see user %q training",
		e.RequestingUserUUID,
		e.TrainingOwnerUUID,
	)
}

func CanUserSeeTraining(user TrainingUser, training Training) error {
	if user.Type() == TrainingUserTrainer {
		return nil
	}
	if user.UUID() == training.UserUUID() {
		return nil
	}
	return ForbiddenToSeeTrainingError{user.UUID(), training.UserUUID()}
}

type SecureTrainingRepository struct {
	rows map[string]Training
}

func NewSecureTrainingRepository(rows map[string]Training) SecureTrainingRepository {
	return SecureTrainingRepository{rows: rows}
}

func (r SecureTrainingRepository) GetTraining(
	ctx context.Context,
	trainingUUID string,
	user TrainingUser,
) (*Training, error) {
	training, err := r.loadTraining(ctx, trainingUUID)
	if err != nil {
		return nil, err
	}
	if err := CanUserSeeTraining(user, training); err != nil {
		return nil, err
	}
	return &training, nil
}

func (r SecureTrainingRepository) UpdateTraining(
	ctx context.Context,
	trainingUUID string,
	user TrainingUser,
	updateFn func(ctx context.Context, tr *Training) (*Training, error),
) error {
	training, err := r.loadTraining(ctx, trainingUUID)
	if err != nil {
		return err
	}
	if err := CanUserSeeTraining(user, training); err != nil {
		return err
	}
	updated, err := updateFn(ctx, &training)
	if err != nil {
		return err
	}
	r.rows[trainingUUID] = *updated
	return nil
}

func (r SecureTrainingRepository) loadTraining(ctx context.Context, trainingUUID string) (Training, error) {
	_ = ctx
	training, ok := r.rows[trainingUUID]
	if !ok {
		return Training{}, errors.New("training not found")
	}
	return training, nil
}

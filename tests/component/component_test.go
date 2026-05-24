// Package component shows a component test for an event-driven
// flow: publish a command, observe the resulting event on the
// other side. This is the test shape the go-testing skill
// recommends for outbox/Watermill flows from the event-driven
// skill.
//
// Two patterns to notice:
//
//  1. Bounded eventual assertion (`assert.EventuallyWithT`). Async
//     systems take SOME time to converge; tests that sleep for a
//     fixed N seconds are both slow and flaky. EventuallyWithT
//     polls until the assertion passes or a deadline expires —
//     fast on the happy path, deterministic on failure.
//
//  2. Per-test filtering by correlation ID. Multiple parallel
//     tests can publish into the same broker; each one filters
//     received events to its own correlation ID so they don't
//     race.
//
// In a real codebase this test would spin up a real Postgres
// + Watermill SQL Pub/Sub via docker-compose or the project's
// existing local harness and run the actual CancelTrainingHandler.
// The structure below is the pattern; the SetupTestService function
// is a placeholder for that bootstrap.
//
// Lives in tests/ at repo root — see top-level README.
package component

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCancelTraining_EmitsEvent(t *testing.T) {
	t.Parallel()

	svc := SetupTestService(t)
	correlationID := uniqueCorrelationID(t)

	// Run the command.
	err := svc.Cancel(context.Background(), CancelCommand{
		TrainingUUID:  "training-component-1",
		CorrelationID: correlationID,
	})
	require.NoError(t, err)

	// Eventual assertion: the consumer should have received and
	// processed a TrainingCancelled event tagged with our
	// correlation ID. We poll for up to 2 seconds, checking
	// every 100ms. If the event arrives in 50ms, the test takes
	// 50ms. If it never arrives, the test fails at 2s with a
	// clear message.
	assert.EventuallyWithT(t, func(c *assert.CollectT) {
		events := svc.ReceivedEvents(correlationID)
		require.Len(c, events, 1)
		assert.Equal(c, "training-component-1", events[0].TrainingUUID)
	}, 2*time.Second, 100*time.Millisecond)
}

// uniqueCorrelationID gives each parallel test its own filter
// key. Using t.Name() works because Go test names are unique per
// run, including subtests.
func uniqueCorrelationID(t *testing.T) string {
	t.Helper()
	return "ct-" + t.Name() + "-" + time.Now().Format("150405.000000000")
}

// ── Stubs for the example to compile ──────────────────────────────
//
// In a real codebase, SetupTestService would build the real
// application graph (Postgres via docker-compose or the existing
// local harness, Watermill SQL Pub/Sub on top of it, the actual
// CancelTrainingHandler from the service-architecture skill, etc.)
// and return handles to invoke commands and observe consumed events.
// The ReceivedEvents method is the filtering hook tests use to
// scope assertions.

type CancelCommand struct {
	TrainingUUID  string
	CorrelationID string
}

type TrainingCancelled struct {
	TrainingUUID  string
	CorrelationID string
}

type TestService struct {
	mu       sync.Mutex
	consumed map[string][]TrainingCancelled
}

// SetupTestService is a placeholder. In real life this would:
//
//   - Start a Postgres testcontainer (or use a per-test schema).
//   - Run migrations.
//   - Build the Watermill router with real handlers.
//   - Start the outbox forwarder.
//   - Subscribe a test-only consumer that records what arrives.
//   - Register t.Cleanup() to tear everything down.
func SetupTestService(t *testing.T) *TestService {
	t.Helper()
	svc := &TestService{consumed: map[string][]TrainingCancelled{}}
	// real setup goes here
	return svc
}

// Cancel simulates publishing the command and the consumer
// receiving the resulting event. In a real test the publish would
// go through the real HTTP/gRPC entry point and the consumer
// would receive through the real broker.
func (s *TestService) Cancel(_ context.Context, cmd CancelCommand) error {
	go func() {
		// Simulate the outbox + broker delay.
		time.Sleep(20 * time.Millisecond)
		s.mu.Lock()
		defer s.mu.Unlock()
		s.consumed[cmd.CorrelationID] = append(s.consumed[cmd.CorrelationID], TrainingCancelled{
			TrainingUUID:  cmd.TrainingUUID,
			CorrelationID: cmd.CorrelationID,
		})
	}()
	return nil
}

func (s *TestService) ReceivedEvents(correlationID string) []TrainingCancelled {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]TrainingCancelled(nil), s.consumed[correlationID]...)
}

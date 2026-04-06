package workers_test

import (
	"context"
	"testing"
	"time"

	"github.com/amirhnajafiz/bedrock-api/internal/components/sessions"
	"github.com/amirhnajafiz/bedrock-api/internal/logger"
	"github.com/amirhnajafiz/bedrock-api/internal/scheduler"
	"github.com/amirhnajafiz/bedrock-api/internal/storage"
	"github.com/amirhnajafiz/bedrock-api/internal/workers"
	"github.com/amirhnajafiz/bedrock-api/pkg/models"
)

func TestWorkerCheckExpiredSessions(t *testing.T) {
	// create a context
	ctx, cancel := context.WithCancel(context.Background())

	// create a new logger instance
	logr := logger.New("info")

	// create a session store and add some sessions with different TTLs
	ss := sessions.NewSessionStore(storage.NewGoCache())
	ss.SaveSession("1", "d1", &models.Session{
		CreatedAt: time.Now(),
		Spec: models.Spec{
			TTL: 2 * time.Second,
		},
	})

	// start the WorkerCheckExpiredSessions in a separate goroutine
	go workers.WorkerCheckExpiredSessions(ctx, logr.Named("session-worker"), 1*time.Second)

	// session must exist now
	if _, err := ss.GetSession("1", "d1"); err != nil {
		t.Errorf("Expected session to exist, got error: %v", err)
	}

	// wait for a short period to allow the worker to run
	time.Sleep(5 * time.Second)

	// session must not exist after TTL has expired
	if _, err := ss.GetSession("1", "d1"); err == nil {
		t.Errorf("Expected session to be expired and removed, but it still exists")
	}

	// cancel the context to stop the worker
	cancel()
}

// TestWorkerDockerDHealthCheck tests the WorkerDockerDHealthCheck function to ensure it correctly updates
// the scheduler with healthy Docker daemons and removes stale entries after the specified interval.
func TestWorkerDockerDHealthCheck(t *testing.T) {
	// create a context
	ctx, cancel := context.WithCancel(context.Background())

	// create a new logger instance
	logr := logger.New("info")

	// create a channel to simulate Docker daemon health updates
	input := make(chan string)

	// get a reference to the scheduler instance
	sc := scheduler.NewRoundRobin()

	// start the WorkerDockerDHealthCheck in a separate goroutine
	go workers.WorkerDockerDHealthCheck(ctx, input, logr.Named("dockerd-health"), 3*time.Second)

	// simulate sending health updates for Docker daemons
	input <- "dockerd1"
	input <- "dockerd2"

	time.Sleep(1 * time.Second)

	// check if the scheduler has the expected Docker daemons
	if !sc.Exists("dockerd1") {
		t.Errorf("Expected dockerd1 to be in the scheduler")
	}
	if !sc.Exists("dockerd2") {
		t.Errorf("Expected dockerd2 to be in the scheduler")
	}

	// wait for a short period to allow the worker to process the updates
	time.Sleep(5 * time.Second)

	// check if the scheduler has removed the stale Docker daemons
	if sc.Exists("dockerd1") {
		t.Errorf("Expected dockerd1 to be removed from the scheduler")
	}
	if sc.Exists("dockerd2") {
		t.Errorf("Expected dockerd2 to be removed from the scheduler")
	}

	// cancel the context to stop the worker
	cancel()
}

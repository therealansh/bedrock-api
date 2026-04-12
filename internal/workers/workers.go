package workers

import (
	"context"
	"time"

	"github.com/amirhnajafiz/bedrock-api/internal/components/sessions"
	"github.com/amirhnajafiz/bedrock-api/internal/scheduler"
	"github.com/amirhnajafiz/bedrock-api/internal/storage"
	"github.com/amirhnajafiz/bedrock-api/pkg/enums"

	"go.uber.org/zap"
)

// WorkerCheckExpiredSessions continuously checks for expired sessions in the session store and removes them at regular intervals.
func WorkerCheckExpiredSessions(ctx context.Context, logr *zap.Logger, interval time.Duration) {
	// get a reference to the session store instance
	ss := sessions.NewSessionStore(storage.NewGoCache())

	// ticker is used to periodically check for expired sessions
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	logr.Info("worker started", zap.Duration("interval", interval))

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			timeSnapshot := time.Now()

			records, err := ss.ListSessions()
			if err != nil {
				logr.Error("failed to list sessions", zap.Error(err))
				continue
			}

			// loop through the sessions in the session store mark any that have expired
			for _, session := range records {
				// only consider running sessions
				if session.Status != enums.SessionStatusRunning {
					continue
				}

				// check with the TTL
				if session.CreatedAt.Add(session.Spec.TTL).Before(timeSnapshot) {
					session.Status = enums.SessionStatusStopped

					logr.Debug("session expired", zap.String("session_id", session.Id))

					// update the session
					if err := ss.SaveSession(session); err != nil {
						logr.Error("failed to update session", zap.Error(err))
					}
				}
			}
		}
	}
}

// WorkerDockerDHealthCheck continuously checks the health status of Docker daemons by listening to an input channel
// for updates and using a ticker to periodically remove stale entries from the health map.
func WorkerDockerDHealthCheck(ctx context.Context, input chan string, logr *zap.Logger, interval time.Duration) {
	// get a reference to the scheduler instance
	scheduler := scheduler.NewRoundRobin()

	// healthMap keeps track of the last time a health update was received for each Docker daemon
	healthMap := make(map[string]time.Time)

	// ticker is used to periodically check for stale entries in the healthMap
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	logr.Info("worker started", zap.Duration("interval", interval))

	for {
		select {
		case <-ctx.Done():
			return
		case dockerd := <-input:
			// update the healthMap with the current time for the received Docker daemon
			healthMap[dockerd] = time.Now()
			scheduler.Append(dockerd)

			logr.Debug("dockerd append", zap.String("dockerd_id", dockerd))
		case <-ticker.C:
			timeSnapshot := time.Now()

			// loop through the healthMap and remove any entries that haven't been updated within the interval
			for dockerd, lastUpdated := range healthMap {
				if timeSnapshot.Sub(lastUpdated) > interval {
					logr.Warn("removing stale Docker daemon from health map", zap.String("dockerd", dockerd))

					delete(healthMap, dockerd)
					scheduler.Drop(dockerd)
				}
			}
		}
	}
}

// WorkerRemoveFinishedSessions checks for finished sessions and marks them as deleted to avoid duplicate ZMQ cleanups.
// Note: This worker must see a finished session three times before it is marked as deleted, to ensure that the ZMQ server
// has processed the session status update and performed any necessary cleanups.
func WorkerRemoveFinishedSessions(ctx context.Context, logr *zap.Logger, interval time.Duration) {
	// get a reference to the session store instance
	ss := sessions.NewSessionStore(storage.NewGoCache())

	// finishedSessions keeps track of the number of times a finished session has been seen
	finishedSessions := make(map[string]int)

	// ticker is used to periodically check for finished sessions
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	logr.Info("worker started", zap.Duration("interval", interval))

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			records, err := ss.ListSessions()
			if err != nil {
				logr.Error("failed to list sessions", zap.Error(err))
				continue
			}

			// loop through the sessions in the session store and mark any that are finished as deleted
			for _, session := range records {
				if session.DeletedAt != nil {
					continue
				}

				if session.Status == enums.SessionStatusFinished || session.Status == enums.SessionStatusFailed {
					finishedSessions[session.Id]++

					if finishedSessions[session.Id] >= 3 {
						tmp := time.Now()
						session.DeletedAt = &tmp

						logr.Debug("marking session as deleted", zap.String("session_id", session.Id))

						// update the session
						if err := ss.SaveSession(session); err != nil {
							logr.Error("failed to update session", zap.Error(err))
						}
					}
				}
			}
		}
	}
}

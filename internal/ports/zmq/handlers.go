package zmq

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/amirhnajafiz/bedrock-api/pkg/enums"
	"github.com/amirhnajafiz/bedrock-api/pkg/models"

	"github.com/zeromq/goczmq"
	"go.uber.org/zap"
)

// socket receiver reads input messages from router and sends them over handler channel.
func (z ZMQServer) socketReceiver(ctx context.Context, router *goczmq.Sock, channel chan [][]byte) error {
	// set receive timeout to 2 seconds to allow graceful shutdown
	router.SetRcvtimeo(2000)

	for {
		// check the context before receiving
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// receive message from router
		request, err := router.RecvMessage()
		if err != nil {
			if !errors.Is(err, goczmq.ErrTimeout) && !errors.Is(err, goczmq.ErrRecvFrame) {
				z.Logr.Warn("failed to receive message", zap.Error(err))
			}

			continue
		}

		z.Logr.Debug("received message", zap.String("mac_address", string(request[0])))

		// publish over input channel
		channel <- request
	}
}

// socket sender reads input from handler channel and sends them to router.
func (z ZMQServer) socketSender(ctx context.Context, router *goczmq.Sock, channel chan [][]byte) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case event := <-channel:
			if err := router.SendMessage(event); err != nil {
				z.Logr.Warn("failed to send message", zap.Error(err))
				return fmt.Errorf("sender router failed: %v", err)
			}

			z.Logr.Debug("sent message", zap.String("mac_address", string(event[0])))
		}

	}
}

// socket handler is the main loop of ZMQ server.
func (z ZMQServer) socketHandler(ctx context.Context, in chan [][]byte, out chan [][]byte) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case event := <-in:
			// socket handler needs to have both MAC and event frame
			if len(event) == 2 {
				mac := event[0]
				response := z.processPacketData(event[1])

				out <- [][]byte{mac, response}
			} else {
				out <- event
			}
		}
	}
}

// process packet data is the main handler of API's ZMQ server.
func (z ZMQServer) processPacketData(raw []byte) []byte {
	// parse events into packets
	pkt, err := models.PacketFromBytes(raw)
	if err != nil {
		z.Logr.Warn("failed to parse event", zap.Error(err))
		return raw
	}

	// reply empty packets
	if pkt.IsEmpty() {
		return pkt.ToBytes()
	}

	// create a response packet
	responsePkt := models.NewPacket()
	responsePkt.WithSender("api")

	// check sender header and registration status, if invalid, reply with empty packet
	dockerd := ""
	if val, ok := pkt.Headers["sender"]; !ok {
		z.Logr.Warn("sender header is missing")
		return responsePkt.ToBytes()
	} else {
		dockerd = val
	}

	// update health status of the sender daemon
	z.DockerDHealthChannel <- dockerd
	z.Logr.Debug("new message from daemon", zap.String("dockerd", dockerd))

	// read events from packet events and update session status in KV storage accordingly
	for _, event := range pkt.Events {
		// get the session id from header
		sid := event.GetSessionId()

		// retrieve the session record from KV storage using session id
		// if session record is not found or dockerd id does not match, skip the event
		record, err := z.sessionStore.GetSessionById(sid)
		if err != nil || record.DockerDId != dockerd {
			z.Logr.Warn(
				"failed to get session",
				zap.Error(err),
				zap.String("session id", sid),
				zap.String("dockerd id", dockerd),
			)
			continue
		}

		// update session status based on event type
		var newSessionStatus enums.SessionStatus
		switch event.GetEventType() {
		case enums.EventTypeSessionRunning:
			newSessionStatus = enums.SessionStatusRunning
		case enums.EventTypeSessionEnd:
			newSessionStatus = enums.SessionStatusFinished
		case enums.EventTypeSessionFailed:
			newSessionStatus = enums.SessionStatusFailed
		default:
			continue
		}

		// use state machine for safe state transition
		record.Status = z.stateMachine.Transition(record.Status, newSessionStatus)

		// update the session in KV storage
		if err := z.sessionStore.SaveSession(record); err != nil {
			z.Logr.Warn(
				"failed to update session",
				zap.Error(err),
				zap.String("session id", record.Id),
				zap.String("dockerd id", dockerd),
			)
			continue
		}
	}

	// retrieve all sessions of the sender daemon from KV storage
	sessions, err := z.sessionStore.ListSessionsByDockerDId(dockerd)
	if err != nil {
		z.Logr.Warn("failed to list sessions", zap.Error(err))
		return responsePkt.ToBytes()
	}

	z.Logr.Debug("processing sessions", zap.String("dockerd_id", dockerd), zap.Int("sessions", len(sessions)))

	// only include running, stopped, or finished sessions
	events := make([]models.Event, 0)
	for _, session := range sessions {
		switch session.Status {
		case enums.SessionStatusPending:
			bytes, err := json.Marshal(session.Spec)
			if err != nil {
				z.Logr.Warn("failed to parse spec", zap.String("session_id", session.Id), zap.Error(err))
				continue
			}

			events = append(
				events,
				models.NewEvent().
					WithSessionId(session.Id).
					WithEventType(enums.EventTypeSessionStart).
					WithPayload(bytes),
			)
		case enums.SessionStatusStopped:
			events = append(
				events,
				models.NewEvent().
					WithSessionId(session.Id).
					WithEventType(enums.EventTypeSessionStopped).
					WithPayload(nil),
			)
		case enums.SessionStatusFailed:
		case enums.SessionStatusFinished:
			events = append(
				events,
				models.NewEvent().
					WithSessionId(session.Id).
					WithEventType(enums.EventTypeSessionCleanup).
					WithPayload(nil),
			)
		}
	}

	// send the response packet back to the sender
	return responsePkt.WithEvents(events...).ToBytes()
}

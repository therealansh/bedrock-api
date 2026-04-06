package zmq

import (
	"context"
	"fmt"

	"github.com/amirhnajafiz/bedrock-api/pkg/models"

	"github.com/zeromq/goczmq"
	"go.uber.org/zap"
)

// socket receiver reads input messages from router and sends them over handler channel.
func (z ZMQServer) socketReceiver(ctx context.Context, router *goczmq.Sock, channel chan [][]byte) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		request, err := router.RecvMessage()
		if err != nil {
			z.Logr.Warn("failed to received message", zap.Error(err))
			continue
		}

		channel <- request
	}
}

// socket sender reads input from handler channel and sends them to router.
func (z ZMQServer) socketSender(ctx context.Context, router *goczmq.Sock, channel chan [][]byte) error {
	for event := range channel {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if err := router.SendMessage(event); err != nil {
			z.Logr.Warn("failed to send message", zap.Error(err))
			return fmt.Errorf("sender router failed: %v", err)
		}
	}

	return nil
}

// socket handler is the main loop of ZMQ server.
func (z ZMQServer) socketHandler(ctx context.Context, in chan [][]byte, out chan [][]byte) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case event := <-in:
			out <- z.processEvent(event)
		}
	}
}

// process event is the main logic of ZMQ server. It processes incoming events.
func (z ZMQServer) processEvent(event [][]byte) [][]byte {
	// parse events into packets
	pkt, err := models.PacketFromBytes(event[1])
	if err != nil {
		z.Logr.Warn("failed to parse event", zap.Error(err))
		return event
	}

	// reply empty packets
	if pkt.IsEmpty() {
		return [][]byte{event[0], pkt.ToBytes()}
	}

	// create a response packet
	responsePkt := models.NewPacket()
	responsePkt.WithSender("api")

	// check sender header and registration status, if invalid, reply with empty packet
	dockerd := ""
	if val, ok := pkt.Headers["sender"]; !ok {
		z.Logr.Warn("sender header is missing")
		return [][]byte{event[0], responsePkt.ToBytes()}
	} else {
		dockerd = val
	}

	// update health status of the sender daemon
	z.DockerDHealthChannel <- dockerd

	// read sessions from packet and update KV storage
	for _, session := range pkt.Sessions {
		record, err := z.sessionStore.GetSession(session.Id, dockerd)
		if err != nil {
			z.Logr.Warn(
				"failed to get session",
				zap.Error(err),
				zap.String("session id", session.Id),
				zap.String("dockerd id", dockerd),
			)
			continue
		}

		// transition session status using state machine
		record.Status = z.stateMachine.Transition(record.Status, session.Status)

		// update the session in KV storage
		if err := z.sessionStore.SaveSession(record); err != nil {
			z.Logr.Warn(
				"failed to update session",
				zap.Error(err),
				zap.String("session id", session.Id),
				zap.String("dockerd id", dockerd),
			)
			continue
		}
	}

	// respond with dockerd sessions
	sessions, err := z.sessionStore.ListSessionsByDockerDId(dockerd)
	if err != nil {
		z.Logr.Warn("failed to list sessions", zap.Error(err))

		return [][]byte{event[0], responsePkt.ToBytes()}
	}

	// process the sessions and add them to the response packet
	for _, session := range sessions {
		responsePkt.Sessions = append(responsePkt.Sessions, *session)
	}

	// send the response packet back to the sender
	return [][]byte{event[0], responsePkt.ToBytes()}
}

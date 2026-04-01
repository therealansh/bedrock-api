package zmq

import (
	"github.com/amirhnajafiz/bedrock-api/pkg/models"

	"github.com/zeromq/goczmq"
	"go.uber.org/zap"
)

// socket receiver reads input messages from router and sends them over handler channel.
func (z ZMQServer) socketReceiver(router *goczmq.Sock, channel chan [][]byte) {
	for {
		request, err := router.RecvMessage()
		if err != nil {
			z.Logr.Warn("failed to received message", zap.Error(err))
			continue
		}

		channel <- request
	}
}

// socket sender reads input from handler channel and sends them to router.
func (z ZMQServer) socketSender(router *goczmq.Sock, channel chan [][]byte) {
	for event := range channel {
		if err := router.SendMessage(event); err != nil {
			z.Logr.Warn("failed to send message", zap.Error(err))
			continue
		}
	}
}

// socket handler is the main loop of ZMQ server.
func (z ZMQServer) socketHandler(in chan [][]byte, out chan [][]byte) {
	for event := range in {
		// parse events into packets
		pkt, err := models.PacketFromBytes(event[1])
		if err != nil {
			z.Logr.Warn("failed to parse event", zap.Error(err))
			continue
		}

		// reply empty packets
		if pkt.IsEmpty() {
			out <- [][]byte{event[0], pkt.ToBytes()}
			continue
		}

		// create a response packet
		responsePkt := models.NewPacket()
		responsePkt.WithSender("api")

		// check daemon registration
		if val, ok := pkt.Headers["register_daemon"]; ok {
			z.Scheduler.Append(val)
			z.Logr.Info("new daemon registered", zap.String("name", val))

			out <- [][]byte{event[0], responsePkt.ToBytes()}
			continue
		}

		// check sender header and registration status, if invalid, reply with empty packet
		dockerd := ""
		if val, ok := pkt.Headers["sender"]; !ok {
			z.Logr.Warn("sender header is missing")

			out <- [][]byte{event[0], responsePkt.ToBytes()}
			continue
		} else if !z.Scheduler.Exists(val) {
			z.Logr.Warn("sender is not a registered daemon", zap.String("name", val))

			out <- [][]byte{event[0], responsePkt.ToBytes()}
			continue
		} else {
			dockerd = val
		}

		// read sessions from packet and update KV storage
		for _, session := range pkt.Sessions {
			record, err := z.SessionStore.GetSession(session.Id, dockerd)
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
			record.Status = z.sm.Transition(record.Status, session.Status)

			// update the session in KV storage
			if err := z.SessionStore.SaveSession(record.Id, dockerd, record); err != nil {
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
		sessions, err := z.SessionStore.ListSessionsByDockerDId(dockerd)
		if err != nil {
			z.Logr.Warn("failed to list sessions", zap.Error(err))

			out <- [][]byte{event[0], responsePkt.ToBytes()}
			continue
		}

		// process the sessions and add them to the response packet
		for _, session := range sessions {
			responsePkt.Sessions = append(responsePkt.Sessions, *session)
		}

		// send the response packet back to the sender
		out <- [][]byte{event[0], responsePkt.ToBytes()}
	}
}

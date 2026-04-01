package zmq

import (
	"fmt"

	"github.com/amirhnajafiz/bedrock-api/internal/components/sessions"
	statemachine "github.com/amirhnajafiz/bedrock-api/internal/components/state_machine"
	"github.com/amirhnajafiz/bedrock-api/internal/scheduler"

	"github.com/zeromq/goczmq"
	"go.uber.org/zap"
)

// ZMQServer represents the ZeroMQ server that handles incoming messages from clients, interacts with the session store and scheduler,
// and sends responses back to clients.
type ZMQServer struct {
	// public shared modules
	Logr         *zap.Logger
	Scheduler    scheduler.Scheduler
	SessionStore sessions.SessionStore

	// private modules
	address string
	sm      *statemachine.StateMachine
}

// Build initializes the ZMQServer with the specified address and returns the server instance.
func (z ZMQServer) Build(address string) *ZMQServer {
	z.address = address
	z.sm = statemachine.NewStateMachine()

	return &z
}

func (z ZMQServer) Serve() error {
	// create a router socket and bind it to the specified host and port
	router, err := goczmq.NewRouter(z.address)
	if err != nil {
		return fmt.Errorf("failed to start zmq server: %v", err)
	}
	defer router.Destroy()

	z.Logr.Info("server started", zap.String("address", z.address))

	// start the socket receiver, handler, and sender goroutines
	in := make(chan [][]byte)
	out := make(chan [][]byte)

	go z.socketReceiver(router, in)
	go z.socketSender(router, out)

	// main loop to handle incoming messages and send responses
	z.socketHandler(in, out)

	return nil
}

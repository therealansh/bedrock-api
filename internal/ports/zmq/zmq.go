package zmq

import (
	"fmt"

	"github.com/zeromq/goczmq"
	"go.uber.org/zap"
)

type ZMQServer struct {
	Address string

	Logr *zap.Logger
}

func (z ZMQServer) Serve() error {
	// create a router socket and bind it to the specified host and port
	router, err := goczmq.NewRouter(z.Address)
	if err != nil {
		return fmt.Errorf("failed to start zmq server: %v", err)
	}
	defer router.Destroy()

	z.Logr.Info("server started", zap.String("address", z.Address))

	// start the socket receiver, handler, and sender goroutines
	in := make(chan [][]byte)
	out := make(chan [][]byte)

	go z.socketReceiver(router, in)
	go z.socketSender(router, out)

	// main loop to handle incoming messages and send responses
	z.socketHandler(in, out)

	return nil
}

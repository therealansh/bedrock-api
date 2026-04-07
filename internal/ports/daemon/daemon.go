package daemon

import (
	"context"
	"time"

	"github.com/amirhnajafiz/bedrock-api/internal/components/containers"
	zmqclient "github.com/amirhnajafiz/bedrock-api/internal/components/zmq_client"
	"github.com/amirhnajafiz/bedrock-api/pkg/models"

	"go.uber.org/zap"
)

// Daemon represents the main loop of the Docker Daemon that manages system sessions.
type Daemon struct {
	// public shared modules
	ContainerManager containers.ContainerManager
	Logr             *zap.Logger
	PullInterval     time.Duration

	// private modules
	name        string
	datadir     string
	tracerImage string
	zclient     *zmqclient.ZMQClient
}

// Build initializes the daemon and returns it.
func (d Daemon) Build(name, datadir, tracerImage, apiAddress string) *Daemon {
	d.name = name
	d.datadir = datadir
	d.tracerImage = tracerImage
	d.zclient = zmqclient.NewZMQClient(apiAddress)

	return &d
}

// Serve starts the daemon and polls for sessions from API.
func (d Daemon) Serve(ctx context.Context) error {
	for {
		// check if the context is done before each iteration
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// interval between each API call
		time.Sleep(d.PullInterval)

		// prepare the packet with the current system status
		packet, err := d.preparePullRequest()
		if err != nil {
			d.Logr.Warn("failed to prepare pull request", zap.Error(err))
			continue
		}

		// send the packet to ZMQ server
		resp, err := d.zclient.SendWithTimeout(packet.ToBytes(), int(d.PullInterval.Seconds()))
		if err != nil {
			d.Logr.Warn("failed to call API", zap.Error(err))
			continue
		}

		// get the response from ZMQ server
		respPacket, err := models.PacketFromBytes(resp)
		if err != nil {
			d.Logr.Warn("failed to parse packet", zap.Error(err))
			continue
		}

		// sync the local container state with the API state
		ers := d.syncWithAPI(respPacket.Sessions)
		for _, er := range ers {
			d.Logr.Warn("failed to sync with API", zap.Error(er))
		}
	}
}

# Bedrock API

Bedrock API is an HTTP server service that enables interaction with **Bedrock** tools via HTTP requests. This service is responsible for tracing management and log collection.

![Abstract Flow](.github/images/abstract_flow_diagram.svg)

## Components

* API: The core coordinator of the system. It's the only component that can access key-value storage. All internal components must interact with API via sockets.
* Docker Daemon: The component that interacts with host's docker daemon to manage system's containers. Responsible for managing target and tracer containers. It get's session data from API using a socket.
* File Manager Daemon: POSIX type file manager, that interacts with API using a socket.
* Key Value Storage: A KV storage for API to keep track of the user sessions.

## Models

* Session
  * UUID
  * Request
    * Docker Image
    * Command
    * Timeout
  * Status
    * Pending | Running | Terminating | Stopped | Finished | Failed
    * Uptime
    * Trace bytes

## API Endpoints

* [POST] /api/new
  * Accept a request from user, set the status (uid, pending, uptime), store it in KV storage.
  * The docker daemon gets pending requests and starts the container (the bedrock tracer container first and the target container).
  * The file manager daemon creates the output directory to store the tracing logs and metadata.
    * data/container-name/...
  * The docker daemon updates the status of a request.
  * Once a container is stopped, finished or failed, the docker daemon must do the cleanup process.
* [POST] /api/stop
  * If a request is not finished or failed, the user can stop it.
  * The docker daemon must terminate all related containers.
* [GET] /api
  * The default route of API must return a list of requests with it's current status.
* [GET] /api/id
  * Upon calling the default route with a request UID, we must serve the output files of tracing from the file manager daemon.

## Requirements

* Docker
* libzmq3-dev
* libczmq-dev
* libsodium-dev
* pkg-config

## Related Projects

* [Bedrock Tracer](https://github.com/amirhnajafiz/bedrock-tracer)

## Libraries

* [Echo](https://github.com/labstack/echo)
* [Docker Go SDK](https://github.com/docker/go-sdk)
* [ZMQ Go](https://github.com/go-zeromq/zmq4)
* [Memory Cache](https://github.com/eko/gocache)

package models

import "time"

// Session represents the client's requests.
// Id: unique string that is the session key
// DockerD: id of the DockerD instance managing this session
// Status: session status
// CreatedAt: session creation time
// Spec: session details
type Session struct {
	Id        string    `json:"id"`
	DockerDId string    `json:"dockerd_id"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	Spec      Spec      `json:"spec"`
}

// Spec field of the session holds the user request.
type Spec struct {
	Image   string `json:"image"`
	Command string `json:"command"`
	TTL     int    `json:"ttl"`
}

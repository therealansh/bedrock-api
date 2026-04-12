package http

import "github.com/amirhnajafiz/bedrock-api/pkg/models"

// ResponseSession represents the session data that will be sent in the HTTP response.
type ResponseSession struct {
	Id        string  `json:"id"`
	Image     string  `json:"image"`
	Command   string  `json:"command"`
	TTL       string  `json:"ttl"`
	CreatedAt string  `json:"created_at"`
	DeletedAt *string `json:"deleted_at,omitempty"`
	Status    string  `json:"status"`
}

// ToResponseSession converts a Session model to a ResponseSession struct for HTTP responses.
func ToResponseSession(session *models.Session) *ResponseSession {
	return &ResponseSession{
		Id:        session.Id,
		Image:     session.Spec.Image,
		Command:   session.Spec.Command,
		TTL:       session.Spec.TTL.String(),
		CreatedAt: session.CreatedAt.String(),
		DeletedAt: func() *string {
			if session.DeletedAt != nil {
				deletedAtStr := session.DeletedAt.String()
				return &deletedAtStr
			}
			return nil
		}(),
		Status: session.Status.String(),
	}
}

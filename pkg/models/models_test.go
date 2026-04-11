package models_test

import (
	"testing"

	"github.com/amirhnajafiz/bedrock-api/pkg/enums"
	"github.com/amirhnajafiz/bedrock-api/pkg/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPacket tests the Packet struct and its methods.
func TestPacket(t *testing.T) {
	t.Run("Packet can be created and converted to bytes and back", func(t *testing.T) {
		original := models.NewPacket().
			WithSender("test-sender").
			WithEvents(
				models.NewEvent().WithEventType(enums.EventTypeSessionEnd).WithPayload([]byte("hello world")),
				models.NewEvent().WithEventType(enums.EventTypeSessionStart).WithPayload([]byte("some payload")),
			)

		bytes := original.ToBytes()

		converted, err := models.PacketFromBytes(bytes)
		require.NoError(t, err)

		assert.Equal(t, original.Headers, converted.Headers)
		assert.Equal(t, len(original.Events), len(converted.Events))

		for i, event := range original.Events {
			assert.Equal(t, event.Headers, converted.Events[i].Headers)
		}
	})

	t.Run("IsEmpty returns true for a packet with no headers", func(t *testing.T) {
		packet := models.NewPacket()
		assert.True(t, packet.IsEmpty())
	})

	t.Run("IsEmpty returns false for a packet with headers", func(t *testing.T) {
		packet := models.NewPacket().WithSender("test-sender")
		assert.False(t, packet.IsEmpty())
	})
}

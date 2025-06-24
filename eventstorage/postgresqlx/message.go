package postgresqlx

import (
	"database/sql"
	"time"

	"github.com/Mikhalevich/outbox"
)

// Message is a outbox_messages table representation.
type Message struct {
	ID           int64        `db:"id"`
	QueueURL     string       `db:"queue_url"`
	PayloadType  string       `db:"payload_type"`
	Payload      []byte       `db:"payload"`
	Dispatched   bool         `db:"dispatched"`
	CreatedAt    time.Time    `db:"created_at"`
	DispatchedAt sql.NullTime `db:"dispatched_at"`
}

// ConvertToMessage convert single outbox event to message.
func ConvertToMessage(e outbox.Event) Message {
	return Message{
		ID:          e.ID.Int64(),
		QueueURL:    e.URL,
		PayloadType: e.PayloadType,
		Payload:     e.Payload,
	}
}

// ConvertToEvents converts multiple messages to outbox events slice.
func ConvertToEvents(messages []Message) []outbox.Event {
	events := make([]outbox.Event, 0, len(messages))

	for _, msg := range messages {
		events = append(events, outbox.Event{
			ID:          outbox.EventID(msg.ID),
			URL:         msg.QueueURL,
			PayloadType: msg.PayloadType,
			Payload:     msg.Payload,
		})
	}

	return events
}

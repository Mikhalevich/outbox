package postgres

import (
	"database/sql"
	"time"

	"github.com/Mikhalevich/outbox"
)

type Message struct {
	ID           int64        `db:"id"`
	QueueURL     string       `db:"queue_url"`
	PayloadType  string       `db:"payload_type"`
	Payload      []byte       `db:"payload"`
	Dispatched   bool         `db:"dispatched"`
	CreatedAt    time.Time    `db:"created_at"`
	DispatchedAt sql.NullTime `db:"dispatched_at"`
}

func ConvertToMessage(e outbox.Event) Message {
	return Message{
		ID:          e.ID.Int64(),
		QueueURL:    e.URL,
		PayloadType: e.PayloadType,
		Payload:     e.Payload,
	}
}

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

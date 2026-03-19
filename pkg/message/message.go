package message

import (
	"encoding/json"
	"fmt"
	"time"
)

type Message struct {
	ID        string    `json:"id"`
	Room      string    `json:"room"`
	From      string    `json:"from"`
	Content   string    `json:"content"`
	ReplyTo   string    `json:"reply_to,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

func New(room, from, content string) Message {
	return Message{
		ID:        fmt.Sprintf("%d", time.Now().UnixNano()),
		Room:      room,
		From:      from,
		Content:   content,
		Timestamp: time.Now(),
	}
}

func (m Message) Encode() ([]byte, error) {
	return json.Marshal(m)
}

func Decode(data []byte) (Message, error) {
	var m Message
	err := json.Unmarshal(data, &m)
	return m, err
}

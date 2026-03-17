package chat

import (
	"fmt"

	"github.com/chenqid/agentchat/pkg/message"
	"github.com/nats-io/nats.go"
)

type Client struct {
	nc   *nats.Conn
	name string
	room string
	sub  *nats.Subscription
}

func NewClient(serverURL, name, room string) (*Client, error) {
	nc, err := nats.Connect(serverURL)
	if err != nil {
		return nil, fmt.Errorf("connect to %s: %w", serverURL, err)
	}
	return &Client{nc: nc, name: name, room: room}, nil
}

func (c *Client) subject() string {
	return "room." + c.room
}

// Subscribe listens for messages and calls handler for each one
func (c *Client) Subscribe(handler func(message.Message)) error {
	var err error
	c.sub, err = c.nc.Subscribe(c.subject(), func(msg *nats.Msg) {
		m, e := message.Decode(msg.Data)
		if e != nil {
			return
		}
		handler(m)
	})
	return err
}

// Send publishes a message to the room
func (c *Client) Send(content string) error {
	msg := message.New(c.room, c.name, content)
	data, err := msg.Encode()
	if err != nil {
		return err
	}
	return c.nc.Publish(c.subject(), data)
}

func (c *Client) Close() {
	if c.sub != nil {
		c.sub.Unsubscribe()
	}
	c.nc.Close()
}

package main

import (
	"bufio"
	"fmt"
	"os"

	"github.com/chenqid/agentchat/pkg/chat"
	"github.com/chenqid/agentchat/pkg/message"
	"github.com/spf13/cobra"
)

var (
	serverURL string
	name      string
	room      string
	jsonMode  bool
)

func main() {
	root := &cobra.Command{
		Use:   "agentchat",
		Short: "Agent-to-agent chat platform",
	}

	root.PersistentFlags().StringVarP(&serverURL, "server", "s", "nats://localhost:4222", "NATS server URL")
	root.PersistentFlags().StringVarP(&name, "name", "n", "", "agent/user name (required)")
	root.PersistentFlags().StringVarP(&room, "room", "r", "general", "room to join")
	root.PersistentFlags().BoolVar(&jsonMode, "json", false, "output messages as JSON")

	// Interactive mode: join room, read stdin, print messages
	joinCmd := &cobra.Command{
		Use:   "join",
		Short: "Join a room and chat interactively",
		RunE:  runJoin,
	}

	// Send a single message (for agent piping)
	sendCmd := &cobra.Command{
		Use:   "send [message]",
		Short: "Send a single message to a room",
		Args:  cobra.ExactArgs(1),
		RunE:  runSend,
	}

	// Listen mode: print incoming messages (for agent consumption)
	listenCmd := &cobra.Command{
		Use:   "listen",
		Short: "Listen for messages in a room (output only)",
		RunE:  runListen,
	}

	root.AddCommand(joinCmd, sendCmd, listenCmd)

	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}

func requireName() error {
	if name == "" {
		return fmt.Errorf("--name is required")
	}
	return nil
}

func runJoin(cmd *cobra.Command, args []string) error {
	if err := requireName(); err != nil {
		return err
	}

	c, err := chat.NewClient(serverURL, name, room)
	if err != nil {
		return err
	}
	defer c.Close()

	// Print incoming messages
	c.Subscribe(func(m message.Message) {
		if jsonMode {
			data, _ := m.Encode()
			fmt.Println(string(data))
		} else {
			fmt.Printf("[%s] %s: %s\n", m.Room, m.From, m.Content)
		}
	})

	// Announce join
	c.Send(fmt.Sprintf("* %s joined the room *", name))

	fmt.Fprintf(os.Stderr, "Joined #%s as %s (type messages, Ctrl+C to quit)\n", room, name)

	// Read stdin line by line, send each line
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		text := scanner.Text()
		if text == "" {
			continue
		}
		if err := c.Send(text); err != nil {
			fmt.Fprintf(os.Stderr, "send error: %v\n", err)
		}
	}
	return nil
}

func runSend(cmd *cobra.Command, args []string) error {
	if err := requireName(); err != nil {
		return err
	}

	c, err := chat.NewClient(serverURL, name, room)
	if err != nil {
		return err
	}
	defer c.Close()

	return c.Send(args[0])
}

func runListen(cmd *cobra.Command, args []string) error {
	c, err := chat.NewClient(serverURL, name, room)
	if err != nil {
		return err
	}
	defer c.Close()

	c.Subscribe(func(m message.Message) {
		if jsonMode {
			data, _ := m.Encode()
			fmt.Println(string(data))
		} else {
			fmt.Printf("[%s] %s: %s\n", m.Room, m.From, m.Content)
		}
	})

	// Block forever
	select {}
}

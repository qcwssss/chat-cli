package chat

import (
	"sync"
	"testing"
	"time"

	"github.com/chenqid/agentchat/pkg/message"
	natsserver "github.com/nats-io/nats-server/v2/server"
)

// 启动嵌入式 NATS server，用于测试
func startTestServer(t *testing.T) *natsserver.Server {
	t.Helper()
	opts := &natsserver.Options{
		Port:     -1, // 随机端口，避免冲突
		NoLog:    true,
		NoSigs:   true,
		DontListen: false,
	}
	ns, err := natsserver.NewServer(opts)
	if err != nil {
		t.Fatalf("启动测试 NATS server 失败: %v", err)
	}
	go ns.Start()
	if !ns.ReadyForConnections(5 * time.Second) {
		t.Fatal("NATS server 未能在 5 秒内就绪")
	}
	return ns
}

// 测试 NewClient 能否成功连接
func TestNewClient(t *testing.T) {
	ns := startTestServer(t)
	defer ns.Shutdown()

	c, err := NewClient(ns.ClientURL(), "test-agent", "general")
	if err != nil {
		t.Fatalf("NewClient 失败: %v", err)
	}
	defer c.Close()

	if c.name != "test-agent" {
		t.Errorf("name 期望 'test-agent'，实际 '%s'", c.name)
	}
	if c.room != "general" {
		t.Errorf("room 期望 'general'，实际 '%s'", c.room)
	}
}

// 测试连接到不存在的 server 应返回错误
func TestNewClient_ConnectionError(t *testing.T) {
	_, err := NewClient("nats://localhost:19999", "agent", "room")
	if err == nil {
		t.Error("连接到不存在的 server 应返回错误")
	}
}

// 测试 subject() 方法
func TestSubject(t *testing.T) {
	ns := startTestServer(t)
	defer ns.Shutdown()

	c, _ := NewClient(ns.ClientURL(), "agent", "task-planning")
	defer c.Close()

	expected := "room.task-planning"
	if c.subject() != expected {
		t.Errorf("subject() 期望 '%s'，实际 '%s'", expected, c.subject())
	}
}

// 测试 Send + Subscribe：一个 client 发送，另一个 client 接收
func TestSendAndSubscribe(t *testing.T) {
	ns := startTestServer(t)
	defer ns.Shutdown()

	// 创建接收端
	receiver, err := NewClient(ns.ClientURL(), "receiver", "general")
	if err != nil {
		t.Fatalf("创建 receiver 失败: %v", err)
	}
	defer receiver.Close()

	var wg sync.WaitGroup
	wg.Add(1)

	var received message.Message
	receiver.Subscribe(func(m message.Message) {
		received = m
		wg.Done()
	})

	// 创建发送端
	sender, err := NewClient(ns.ClientURL(), "sender", "general")
	if err != nil {
		t.Fatalf("创建 sender 失败: %v", err)
	}
	defer sender.Close()

	// 发送消息
	err = sender.Send("hello from sender")
	if err != nil {
		t.Fatalf("发送消息失败: %v", err)
	}

	// 等待接收（最多 3 秒）
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// 成功收到
	case <-time.After(3 * time.Second):
		t.Fatal("3 秒内未收到消息")
	}

	if received.From != "sender" {
		t.Errorf("From 期望 'sender'，实际 '%s'", received.From)
	}
	if received.Content != "hello from sender" {
		t.Errorf("Content 期望 'hello from sender'，实际 '%s'", received.Content)
	}
	if received.Room != "general" {
		t.Errorf("Room 期望 'general'，实际 '%s'", received.Room)
	}
}

// 测试不同房间的消息隔离
func TestRoomIsolation(t *testing.T) {
	ns := startTestServer(t)
	defer ns.Shutdown()

	// room-a 的接收者
	clientA, _ := NewClient(ns.ClientURL(), "agent-a", "room-a")
	defer clientA.Close()

	gotMessage := false
	clientA.Subscribe(func(m message.Message) {
		gotMessage = true
	})

	// room-b 的发送者
	clientB, _ := NewClient(ns.ClientURL(), "agent-b", "room-b")
	defer clientB.Close()

	// 在 room-b 发消息
	clientB.Send("this should not reach room-a")

	// 等一小段时间，确认 room-a 没收到
	time.Sleep(500 * time.Millisecond)

	if gotMessage {
		t.Error("room-a 不应该收到 room-b 的消息")
	}
}

// 测试多个订阅者都能收到同一条消息（群聊广播）
func TestBroadcast(t *testing.T) {
	ns := startTestServer(t)
	defer ns.Shutdown()

	var wg sync.WaitGroup
	wg.Add(2)

	// 创建两个接收者
	r1, _ := NewClient(ns.ClientURL(), "r1", "broadcast-room")
	defer r1.Close()
	r1.Subscribe(func(m message.Message) { wg.Done() })

	r2, _ := NewClient(ns.ClientURL(), "r2", "broadcast-room")
	defer r2.Close()
	r2.Subscribe(func(m message.Message) { wg.Done() })

	// 发送一条消息
	sender, _ := NewClient(ns.ClientURL(), "sender", "broadcast-room")
	defer sender.Close()
	sender.Send("broadcast!")

	// 等待两个接收者都收到
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// 两个接收者都收到了
	case <-time.After(3 * time.Second):
		t.Fatal("3 秒内未能完成广播")
	}
}

// 测试 Close 后发送应失败
func TestSendAfterClose(t *testing.T) {
	ns := startTestServer(t)
	defer ns.Shutdown()

	c, _ := NewClient(ns.ClientURL(), "agent", "general")
	c.Close()

	err := c.Send("should fail")
	if err == nil {
		t.Error("Close 后发送消息应返回错误")
	}
}

package message

import (
	"encoding/json"
	"testing"
	"time"
)

// 测试 New 函数能否正确创建 Message
func TestNew(t *testing.T) {
	msg := New("general", "alice", "hello world")

	if msg.Room != "general" {
		t.Errorf("Room 期望 'general'，实际 '%s'", msg.Room)
	}
	if msg.From != "alice" {
		t.Errorf("From 期望 'alice'，实际 '%s'", msg.From)
	}
	if msg.Content != "hello world" {
		t.Errorf("Content 期望 'hello world'，实际 '%s'", msg.Content)
	}
	if msg.ID == "" {
		t.Error("ID 不应为空")
	}
	if msg.Timestamp.IsZero() {
		t.Error("Timestamp 不应为零值")
	}
	if msg.ReplyTo != "" {
		t.Errorf("ReplyTo 应为空，实际 '%s'", msg.ReplyTo)
	}
}

// 测试两次创建的消息 ID 不同
func TestNew_UniqueID(t *testing.T) {
	m1 := New("room1", "bob", "msg1")
	time.Sleep(time.Nanosecond)
	m2 := New("room1", "bob", "msg2")

	if m1.ID == m2.ID {
		t.Errorf("两条消息的 ID 不应相同: %s", m1.ID)
	}
}

// 测试 Encode 输出合法 JSON
func TestEncode(t *testing.T) {
	msg := New("general", "alice", "test message")
	data, err := msg.Encode()
	if err != nil {
		t.Fatalf("Encode 不应返回错误: %v", err)
	}

	// 验证是合法 JSON
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("Encode 输出不是合法 JSON: %v", err)
	}

	// 验证关键字段存在
	for _, key := range []string{"id", "room", "from", "content", "timestamp"} {
		if _, ok := raw[key]; !ok {
			t.Errorf("JSON 中缺少字段 '%s'", key)
		}
	}

	// reply_to 为空时不应出现在 JSON 中 (omitempty)
	if _, ok := raw["reply_to"]; ok {
		t.Error("reply_to 为空时不应出现在 JSON 中")
	}
}

// 测试 Decode 能正确反序列化
func TestDecode(t *testing.T) {
	original := New("task-room", "bot-01", "任务完成")
	data, err := original.Encode()
	if err != nil {
		t.Fatalf("Encode 失败: %v", err)
	}

	decoded, err := Decode(data)
	if err != nil {
		t.Fatalf("Decode 失败: %v", err)
	}

	if decoded.ID != original.ID {
		t.Errorf("ID 不匹配: 期望 '%s'，实际 '%s'", original.ID, decoded.ID)
	}
	if decoded.Room != original.Room {
		t.Errorf("Room 不匹配: 期望 '%s'，实际 '%s'", original.Room, decoded.Room)
	}
	if decoded.From != original.From {
		t.Errorf("From 不匹配: 期望 '%s'，实际 '%s'", original.From, decoded.From)
	}
	if decoded.Content != original.Content {
		t.Errorf("Content 不匹配: 期望 '%s'，实际 '%s'", original.Content, decoded.Content)
	}
}

// 测试 Decode 处理非法 JSON
func TestDecode_InvalidJSON(t *testing.T) {
	_, err := Decode([]byte("not json"))
	if err == nil {
		t.Error("Decode 对非法 JSON 应返回错误")
	}
}

// 测试 Decode 处理空数据
func TestDecode_EmptyData(t *testing.T) {
	_, err := Decode([]byte{})
	if err == nil {
		t.Error("Decode 对空数据应返回错误")
	}
}

// 测试带 ReplyTo 字段的消息序列化
func TestEncode_WithReplyTo(t *testing.T) {
	msg := New("general", "alice", "回复你")
	msg.ReplyTo = "msg_original_123"

	data, err := msg.Encode()
	if err != nil {
		t.Fatalf("Encode 失败: %v", err)
	}

	var raw map[string]interface{}
	json.Unmarshal(data, &raw)

	if raw["reply_to"] != "msg_original_123" {
		t.Errorf("reply_to 期望 'msg_original_123'，实际 '%v'", raw["reply_to"])
	}
}

// 测试中文内容的序列化/反序列化
func TestEncodeDecode_Chinese(t *testing.T) {
	msg := New("general", "用户A", "你好，世界！🎉")
	data, err := msg.Encode()
	if err != nil {
		t.Fatalf("Encode 中文消息失败: %v", err)
	}

	decoded, err := Decode(data)
	if err != nil {
		t.Fatalf("Decode 中文消息失败: %v", err)
	}

	if decoded.Content != "你好，世界！🎉" {
		t.Errorf("中文内容不匹配: '%s'", decoded.Content)
	}
	if decoded.From != "用户A" {
		t.Errorf("中文发送者不匹配: '%s'", decoded.From)
	}
}

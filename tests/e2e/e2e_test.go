package e2e

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	natsserver "github.com/nats-io/nats-server/v2/server"
)

// 编译后的二进制路径，在 TestMain 中设置
var binaryPath string

// 测试用的 NATS server URL
var natsURL string

// TestMain 负责：编译二进制 → 启动 NATS → 运行测试 → 清理
func TestMain(m *testing.M) {
	// 1. 编译 CLI 二进制到临时目录
	tmpDir, err := os.MkdirTemp("", "agentchat-e2e-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "创建临时目录失败: %v\n", err)
		os.Exit(1)
	}
	defer os.RemoveAll(tmpDir)

	binaryPath = filepath.Join(tmpDir, "agentchat")
	buildCmd := exec.Command("go", "build", "-o", binaryPath, "../../cmd/agentchat")
	buildCmd.Stdout = os.Stdout
	buildCmd.Stderr = os.Stderr
	if err := buildCmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "编译失败: %v\n", err)
		os.Exit(1)
	}

	// 2. 启动嵌入式 NATS server
	ns, err := natsserver.NewServer(&natsserver.Options{
		Port:   -1,
		NoLog:  true,
		NoSigs: true,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "启动 NATS 失败: %v\n", err)
		os.Exit(1)
	}
	go ns.Start()
	if !ns.ReadyForConnections(5 * time.Second) {
		fmt.Fprintf(os.Stderr, "NATS 未就绪\n")
		os.Exit(1)
	}
	natsURL = ns.ClientURL()
	defer ns.Shutdown()

	// 3. 运行测试
	os.Exit(m.Run())
}

// ==================== 辅助函数 ====================

// 启动 CLI 进程，返回 cmd 对象和 stdout reader
func startProcess(args ...string) (*exec.Cmd, io.ReadCloser, error) {
	cmd := exec.Command(binaryPath, args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, fmt.Errorf("获取 stdout pipe 失败: %w", err)
	}
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return nil, nil, fmt.Errorf("启动进程失败: %w", err)
	}
	return cmd, stdout, nil
}

// 启动带 stdin 的交互进程
func startInteractiveProcess(args ...string) (*exec.Cmd, io.WriteCloser, io.ReadCloser, error) {
	cmd := exec.Command(binaryPath, args...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("获取 stdin pipe 失败: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("获取 stdout pipe 失败: %w", err)
	}
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return nil, nil, nil, fmt.Errorf("启动进程失败: %w", err)
	}
	return cmd, stdin, stdout, nil
}

// 从 reader 中读取一行，带超时
func readLineWithTimeout(r io.Reader, timeout time.Duration) (string, error) {
	ch := make(chan string, 1)
	errCh := make(chan error, 1)

	scanner := bufio.NewScanner(r)
	go func() {
		if scanner.Scan() {
			ch <- scanner.Text()
		} else {
			errCh <- fmt.Errorf("读取结束: %v", scanner.Err())
		}
	}()

	select {
	case line := <-ch:
		return line, nil
	case err := <-errCh:
		return "", err
	case <-time.After(timeout):
		return "", fmt.Errorf("读取超时（%v）", timeout)
	}
}

// ==================== E2E 测试用例 ====================

// 测试 send → listen 完整消息传递流程
func TestE2E_SendAndListen(t *testing.T) {
	room := "e2e-send-listen"

	// 启动 listen 进程
	listenCmd, stdout, err := startProcess(
		"listen",
		"--server", natsURL,
		"--name", "listener",
		"--room", room,
		"--json",
	)
	if err != nil {
		t.Fatalf("启动 listen 进程失败: %v", err)
	}
	defer func() {
		listenCmd.Process.Kill()
		listenCmd.Wait()
	}()

	// 等 listen 就绪
	time.Sleep(300 * time.Millisecond)

	// 用 send 发一条消息
	sendCmd := exec.Command(binaryPath,
		"send", "hello-e2e",
		"--server", natsURL,
		"--name", "sender",
		"--room", room,
	)
	sendCmd.Stderr = os.Stderr
	if err := sendCmd.Run(); err != nil {
		t.Fatalf("send 命令失败: %v", err)
	}

	// 读取 listen 的输出
	line, err := readLineWithTimeout(stdout, 5*time.Second)
	if err != nil {
		t.Fatalf("读取 listen 输出失败: %v", err)
	}

	// 验证是合法 JSON 并包含正确内容
	var msg map[string]interface{}
	if err := json.Unmarshal([]byte(line), &msg); err != nil {
		t.Fatalf("listen 输出不是合法 JSON: %s", line)
	}
	if msg["content"] != "hello-e2e" {
		t.Errorf("content 期望 'hello-e2e'，实际 '%v'", msg["content"])
	}
	if msg["from"] != "sender" {
		t.Errorf("from 期望 'sender'，实际 '%v'", msg["from"])
	}
	if msg["room"] != room {
		t.Errorf("room 期望 '%s'，实际 '%v'", room, msg["room"])
	}
}

// 测试 join 交互模式：通过 stdin 发消息，另一端 listen 收到
func TestE2E_JoinInteractive(t *testing.T) {
	room := "e2e-join"

	// 启动 listen 进程
	listenCmd, stdout, err := startProcess(
		"listen",
		"--server", natsURL,
		"--name", "listener",
		"--room", room,
		"--json",
	)
	if err != nil {
		t.Fatalf("启动 listen 失败: %v", err)
	}
	defer func() {
		listenCmd.Process.Kill()
		listenCmd.Wait()
	}()

	time.Sleep(300 * time.Millisecond)

	// 启动 join 交互进程
	joinCmd, stdin, _, err := startInteractiveProcess(
		"join",
		"--server", natsURL,
		"--name", "joiner",
		"--room", room,
	)
	if err != nil {
		t.Fatalf("启动 join 失败: %v", err)
	}
	defer func() {
		joinCmd.Process.Kill()
		joinCmd.Wait()
	}()

	// 等 join 完成订阅和加入通知
	time.Sleep(500 * time.Millisecond)

	// 先读掉 join 通知消息 ("* joiner joined the room *")
	joinNotice, err := readLineWithTimeout(stdout, 3*time.Second)
	if err != nil {
		t.Fatalf("读取 join 通知失败: %v", err)
	}
	var noticeMsg map[string]interface{}
	json.Unmarshal([]byte(joinNotice), &noticeMsg)
	if content, ok := noticeMsg["content"].(string); ok {
		if !strings.Contains(content, "joined") {
			t.Logf("预期 join 通知，实际: %s", content)
		}
	}

	// 通过 stdin 发消息
	fmt.Fprintln(stdin, "interactive-hello")

	// 读取 listen 收到的消息
	line, err := readLineWithTimeout(stdout, 5*time.Second)
	if err != nil {
		t.Fatalf("读取交互消息失败: %v", err)
	}

	var msg map[string]interface{}
	json.Unmarshal([]byte(line), &msg)
	if msg["content"] != "interactive-hello" {
		t.Errorf("content 期望 'interactive-hello'，实际 '%v'", msg["content"])
	}
	if msg["from"] != "joiner" {
		t.Errorf("from 期望 'joiner'，实际 '%v'", msg["from"])
	}
}

// 测试不同房间的消息隔离
func TestE2E_RoomIsolation(t *testing.T) {
	// 在 room-alpha 启动 listen
	listenCmd, stdout, err := startProcess(
		"listen",
		"--server", natsURL,
		"--name", "alpha-listener",
		"--room", "room-alpha",
	)
	if err != nil {
		t.Fatalf("启动 listen 失败: %v", err)
	}
	defer func() {
		listenCmd.Process.Kill()
		listenCmd.Wait()
	}()

	time.Sleep(300 * time.Millisecond)

	// 在 room-beta 发消息
	sendCmd := exec.Command(binaryPath,
		"send", "wrong-room-msg",
		"--server", natsURL,
		"--name", "beta-sender",
		"--room", "room-beta",
	)
	if err := sendCmd.Run(); err != nil {
		t.Fatalf("send 命令失败: %v", err)
	}

	// room-alpha 的 listen 不应收到消息
	_, err = readLineWithTimeout(stdout, 1*time.Second)
	if err == nil {
		t.Error("room-alpha 不应收到 room-beta 的消息")
	}
	// 超时是预期行为，说明消息正确隔离
}

// 测试缺少 --name 参数时应报错退出
func TestE2E_MissingName(t *testing.T) {
	// join 需要 --name
	cmd := exec.Command(binaryPath,
		"join",
		"--server", natsURL,
		"--room", "some-room",
	)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err == nil {
		t.Error("缺少 --name 时 join 应返回非零退出码")
	}
	if !strings.Contains(stderr.String(), "--name") {
		t.Errorf("错误信息应提示 --name，实际: %s", stderr.String())
	}
}

// 测试 --json 模式输出合法 JSON
func TestE2E_JsonOutput(t *testing.T) {
	room := "e2e-json"

	// 启动 listen --json
	listenCmd, stdout, err := startProcess(
		"listen",
		"--server", natsURL,
		"--name", "json-listener",
		"--room", room,
		"--json",
	)
	if err != nil {
		t.Fatalf("启动 listen 失败: %v", err)
	}
	defer func() {
		listenCmd.Process.Kill()
		listenCmd.Wait()
	}()

	time.Sleep(300 * time.Millisecond)

	// 发送带特殊字符的消息
	sendCmd := exec.Command(binaryPath,
		"send", `{"nested":"json"} & <html> 你好`,
		"--server", natsURL,
		"--name", "special-sender",
		"--room", room,
	)
	if err := sendCmd.Run(); err != nil {
		t.Fatalf("send 失败: %v", err)
	}

	line, err := readLineWithTimeout(stdout, 5*time.Second)
	if err != nil {
		t.Fatalf("读取输出失败: %v", err)
	}

	// 验证整行是合法 JSON
	var msg map[string]interface{}
	if err := json.Unmarshal([]byte(line), &msg); err != nil {
		t.Fatalf("输出不是合法 JSON: %s", line)
	}

	// 验证所有必要字段存在
	for _, field := range []string{"id", "room", "from", "content", "timestamp"} {
		if _, ok := msg[field]; !ok {
			t.Errorf("JSON 缺少字段 '%s'", field)
		}
	}

	// 内容应完整保留特殊字符
	expected := `{"nested":"json"} & <html> 你好`
	if msg["content"] != expected {
		t.Errorf("content 不匹配:\n  期望: %s\n  实际: %v", expected, msg["content"])
	}
}

package chatter_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

const serverAddr = "127.0.0.1:48090"

// TestWebSocketRealtimePush 验证实时推送链路：
// WS 连接订阅记录频道 → HTTP 新增评论 → 客户端收到 timeline.new 推送。
// 需先运行服务（go run ./cmd/server）；服务未运行时跳过。
func TestWebSocketRealtimePush(t *testing.T) {
	if resp, err := http.Get("http://" + serverAddr + "/health"); err != nil {
		t.Skipf("跳过：服务未运行 (%v)", err)
	} else {
		_ = resp.Body.Close()
	}

	conn, _, err := websocket.DefaultDialer.Dial("ws://"+serverAddr+"/infra/ws?token=devtoken", nil)
	if err != nil {
		t.Fatalf("WebSocket 连接失败: %v", err)
	}
	defer func() { _ = conn.Close() }()

	bizID := time.Now().Unix()

	// 订阅该业务记录的频道。
	if err := conn.WriteJSON(map[string]any{
		"type": "subscribe", "bizType": "ws_test", "bizId": bizID,
	}); err != nil {
		t.Fatalf("发送订阅失败: %v", err)
	}
	time.Sleep(400 * time.Millisecond) // 等待服务端处理订阅

	// 新增评论，触发时间线生成与推送。
	body, _ := json.Marshal(map[string]any{
		"bizType": "ws_test", "bizId": bizID, "content": "WebSocket 实时推送验证",
	})
	req, _ := http.NewRequest(http.MethodPost,
		"http://"+serverAddr+"/admin-api/chatter/comment", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer devtoken")
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("新增评论失败: %v", err)
	}
	_ = resp.Body.Close()

	// 在超时内等待 timeline.new 推送。
	_ = conn.SetReadDeadline(time.Now().Add(12 * time.Second))
	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			t.Fatalf("未在超时内收到推送: %v", err)
		}
		var msg struct {
			Type    string `json:"type"`
			BizType string `json:"bizType"`
		}
		if json.Unmarshal(data, &msg) != nil {
			continue
		}
		if msg.Type == "timeline.new" {
			if msg.BizType != "ws_test" {
				t.Fatalf("推送 bizType 不符: %s", msg.BizType)
			}
			fmt.Println("收到实时推送:", string(data))
			return // 成功
		}
	}
}

package websocket

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"

	"yudao-go/internal/framework/logger"
	"yudao-go/internal/framework/security"
	"yudao-go/internal/framework/web"
	"yudao-go/internal/pkg/errcode"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = 50 * time.Second // 必须小于 pongWait
	maxMessageSize = 4 * 1024
	sendBuffer     = 64
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	// TODO P4：生产环境按 yudao-ui 域名收紧来源校验。
	CheckOrigin: func(r *http.Request) bool { return true },
}

// Conn 表示一条 WebSocket 连接。
// 并发安全：send 通道不关闭，关闭信号经 done；close 由 closeOnce 保证幂等。
type Conn struct {
	userID    int64
	tenantID  int64
	ws        *websocket.Conn
	send      chan []byte
	done      chan struct{}
	channels  map[string]struct{} // 由 Hub.mu 保护
	closeOnce sync.Once
	owner     *Hub
}

// clientMessage 是客户端上行消息。
type clientMessage struct {
	Type    string `json:"type"` // subscribe | unsubscribe
	BizType string `json:"bizType"`
	BizID   int64  `json:"bizId"`
}

func newConn(hub *Hub, ws *websocket.Conn, userID, tenantID int64) *Conn {
	return &Conn{
		userID:   userID,
		tenantID: tenantID,
		ws:       ws,
		send:     make(chan []byte, sendBuffer),
		done:     make(chan struct{}),
		channels: make(map[string]struct{}),
		owner:    hub,
	}
}

// trySend 非阻塞推送。连接已关闭则丢弃；发送缓冲满（慢消费者）则关闭连接。
func (c *Conn) trySend(msg []byte) {
	select {
	case <-c.done:
		return
	default:
	}
	select {
	case c.send <- msg:
	case <-c.done:
	default:
		// 慢消费者：丢弃并关闭连接，避免拖垮 Hub。
		logger.L().Warn("websocket slow consumer, closing", "user_id", c.userID)
		c.close()
	}
}

// close 幂等关闭连接：发出 done 信号、摘除 Hub 注册、关闭底层连接。
func (c *Conn) close() {
	c.closeOnce.Do(func() {
		close(c.done)
		c.owner.unregister(c)
		_ = c.ws.Close()
	})
}

func (c *Conn) readPump() {
	defer c.close()
	c.ws.SetReadLimit(maxMessageSize)
	_ = c.ws.SetReadDeadline(time.Now().Add(pongWait))
	c.ws.SetPongHandler(func(string) error {
		return c.ws.SetReadDeadline(time.Now().Add(pongWait))
	})
	for {
		_, data, err := c.ws.ReadMessage()
		if err != nil {
			return
		}
		var msg clientMessage
		if json.Unmarshal(data, &msg) != nil {
			continue
		}
		switch msg.Type {
		case "subscribe":
			c.owner.subscribe(c, RecordChannel(c.tenantID, msg.BizType, msg.BizID))
		case "unsubscribe":
			c.owner.unsubscribe(c, RecordChannel(c.tenantID, msg.BizType, msg.BizID))
		}
	}
}

func (c *Conn) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.close()
	}()
	for {
		select {
		case msg := <-c.send:
			_ = c.ws.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.ws.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}
		case <-ticker.C:
			_ = c.ws.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.ws.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		case <-c.done:
			_ = c.ws.SetWriteDeadline(time.Now().Add(writeWait))
			_ = c.ws.WriteMessage(websocket.CloseMessage,
				websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
			return
		}
	}
}

// GinHandler 返回 WebSocket 接入处理器。须挂在带认证中间件的路由上。
func GinHandler(hub *Hub) gin.HandlerFunc {
	return func(c *gin.Context) {
		user := security.CurrentUser(c.Request.Context())
		if user == nil {
			web.Fail(c, errcode.Unauthorized)
			return
		}
		ws, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			return // Upgrade 失败时已写入响应
		}
		conn := newConn(hub, ws, user.ID, user.TenantID)
		hub.register(conn)
		go conn.writePump()
		conn.readPump() // 阻塞至连接关闭
	}
}

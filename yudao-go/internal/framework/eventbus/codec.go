package eventbus

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// Envelope 是领域事件的可序列化信封，用于发件箱存储与跨进程传输。
type Envelope struct {
	EventID       string          `json:"eventId"`
	Topic         string          `json:"topic"`
	AggregateType string          `json:"aggregateType"`
	AggregateID   int64           `json:"aggregateId"`
	OccurredAt    time.Time       `json:"occurredAt"`
	Payload       json.RawMessage `json:"payload"`
}

// Decoder 将事件载荷还原为具体领域事件。
type Decoder func(payload json.RawMessage) (DomainEvent, error)

// Codec 负责领域事件与 Envelope 的相互转换。并发安全：注册发生在启动期，编解码在运行期。
type Codec struct {
	mu       sync.RWMutex
	decoders map[string]Decoder
}

func NewCodec() *Codec { return &Codec{decoders: make(map[string]Decoder)} }

// Register 注册某主题的事件解码器。
func (c *Codec) Register(topic string, d Decoder) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.decoders[topic] = d
}

// Encode 将领域事件编码为 Envelope。
func (c *Codec) Encode(e DomainEvent) (Envelope, error) {
	payload, err := json.Marshal(e)
	if err != nil {
		return Envelope{}, fmt.Errorf("eventbus: encode %q: %w", e.Topic(), err)
	}
	return Envelope{
		EventID:       e.EventID(),
		Topic:         e.Topic(),
		AggregateType: e.AggregateType(),
		AggregateID:   e.AggregateID(),
		OccurredAt:    e.OccurredAt(),
		Payload:       payload,
	}, nil
}

// Decode 依据主题将 Envelope 载荷还原为领域事件。
func (c *Codec) Decode(env Envelope) (DomainEvent, error) {
	c.mu.RLock()
	d, ok := c.decoders[env.Topic]
	c.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("eventbus: no decoder registered for topic %q", env.Topic)
	}
	return d(env.Payload)
}

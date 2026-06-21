// Package idgen 提供 ID 生成能力。
package idgen

import (
	"sync"
	"time"

	"github.com/google/uuid"
)

// UUID 返回随机 UUID 字符串，用于 event_id 等幂等标识。
func UUID() string { return uuid.NewString() }

const (
	snowflakeEpoch    = int64(1700000000000) // 2023-11-14，自定义纪元
	snowflakeNodeBits = 10
	snowflakeSeqBits  = 12
	snowflakeMaxNode  = -1 ^ (-1 << snowflakeNodeBits)
	snowflakeMaxSeq   = -1 ^ (-1 << snowflakeSeqBits)
)

// Snowflake 是一个并发安全的 64 位分布式 ID 生成器。
type Snowflake struct {
	mu     sync.Mutex
	nodeID int64
	lastMs int64
	seq    int64
}

// NewSnowflake 创建生成器，nodeID 应在集群内唯一（0..1023）。
func NewSnowflake(nodeID int64) *Snowflake {
	return &Snowflake{nodeID: nodeID & snowflakeMaxNode}
}

// NextID 生成下一个 ID。并发安全：全程持锁，同毫秒序号溢出时自旋到下一毫秒。
func (s *Snowflake) NextID() int64 {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UnixMilli()
	// 时钟回拨保护：回拨时退化为沿用 lastMs，避免生成更小的 ID。
	if now < s.lastMs {
		now = s.lastMs
	}
	if now == s.lastMs {
		s.seq = (s.seq + 1) & snowflakeMaxSeq
		if s.seq == 0 { // 当前毫秒序号用尽，自旋等待下一毫秒
			for now <= s.lastMs {
				now = time.Now().UnixMilli()
			}
		}
	} else {
		s.seq = 0
	}
	s.lastMs = now
	return ((now - snowflakeEpoch) << (snowflakeNodeBits + snowflakeSeqBits)) |
		(s.nodeID << snowflakeSeqBits) | s.seq
}

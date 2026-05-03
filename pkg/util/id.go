// Package util 提供通用工具函数
package util

import (
	"crypto/rand"
	"fmt"
	"sync"
	"time"

	"github.com/oklog/ulid/v2"
)

// ulidMu 保护 ulid 熵池的并发安全
var ulidMu sync.Mutex

// newULIDEntropy 创建带时间戳的 ULID 熵池
func newULIDEntropy() ioReader {
	return &ulidLockedMonotonicReader{
		reader: rand.Reader,
	}
}

type ioReader interface {
	Read(p []byte) (n int, err error)
}

// ulidLockedMonotonicReader 线程安全的单调递增 ULID 读取器
type ulidLockedMonotonicReader struct {
	reader ioReader
	mtx    sync.Mutex
}

func (r *ulidLockedMonotonicReader) Read(p []byte) (int, error) {
	r.mtx.Lock()
	defer r.mtx.Unlock()
	return r.reader.Read(p)
}

// NewID 生成带业务前缀的 ULID
// 格式：{prefix}_{26字符ULID}
// 示例：NewID("u") → "u_01JQZ5ZJY8X7K2W6V9N3M4P5QR"
// 空前缀返回纯 ULID（26 字符）
func NewID(prefix string) string {
	entropy := ulid.Monotonic(rand.Reader, 0)
	id := ulid.MustNew(ulid.Timestamp(time.Now()), entropy)
	if prefix == "" {
		return id.String()
	}
	return fmt.Sprintf("%s_%s", prefix, id.String())
}

// MustID 类似 NewID，但如果前缀为空则 panic
func MustID(prefix string) string {
	if prefix == "" {
		panic("util.MustID: prefix 不能为空")
	}
	return NewID(prefix)
}

// IsValidID 验证 ID 是否符合 {prefix}_{ULID} 格式
func IsValidID(id, prefix string) bool {
	if len(id) != len(prefix)+1+ulid.EncodedSize {
		return false
	}
	if id[:len(prefix)] != prefix || id[len(prefix)] != '_' {
		return false
	}
	_, err := ulid.Parse(id[len(prefix)+1:])
	return err == nil
}

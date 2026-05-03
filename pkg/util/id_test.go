package util

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewID_ValidPrefix(t *testing.T) {
	id := NewID("s")
	assert.True(t, strings.HasPrefix(id, "s_"), "应以 s_ 开头")
	// ULID 长度为 26 字符，格式：s_ + 26 = 28 字符
	assert.Equal(t, 28, len(id), "ID 长度应为 28（前缀 2 + ULID 26）")
}

func TestNewID_EmptyPrefix(t *testing.T) {
	id := NewID("")
	assert.False(t, strings.HasPrefix(id, "_"), "空前缀不应加下划线")
	// 纯 ULID 长度为 26
	assert.Equal(t, 26, len(id), "空前缀应返回纯 ULID（26 字符）")
}

func TestNewID_Unique(t *testing.T) {
	ids := make(map[string]bool)
	for i := 0; i < 100; i++ {
		id := NewID("u")
		assert.False(t, ids[id], "ID 不应重复: %s", id)
		ids[id] = true
	}
}

func TestNewID_LongPrefix(t *testing.T) {
	id := NewID("session")
	assert.True(t, strings.HasPrefix(id, "session_"))
	assert.Equal(t, 34, len(id), "长前缀 ID 长度应为 len('session') + 1 + 26")
}

func TestMustID_PanicsOnEmpty(t *testing.T) {
	assert.Panics(t, func() {
		MustID("")
	}, "空前缀应 panic")
}

func TestMustID_Valid(t *testing.T) {
	id := MustID("t")
	assert.True(t, strings.HasPrefix(id, "t_"))
	assert.Equal(t, 28, len(id))
}

func TestIsValidID_Valid(t *testing.T) {
	id := NewID("s")
	assert.True(t, IsValidID(id, "s"))
}

func TestIsValidID_Invalid_Empty(t *testing.T) {
	assert.False(t, IsValidID("", "s"))
}

func TestIsValidID_Invalid_WrongPrefix(t *testing.T) {
	id := NewID("s")
	assert.False(t, IsValidID(id, "t"))
}

func TestIsValidID_Invalid_TooShort(t *testing.T) {
	assert.False(t, IsValidID("s_abc", "s"))
}

func TestIsValidID_Invalid_MissingUnderscore(t *testing.T) {
	id := NewID("s")
	// 去掉下划线
	noUnderscore := "s" + id[2:]
	assert.False(t, IsValidID(noUnderscore, "s"))
}

func TestIsValidID_MultiCharPrefix(t *testing.T) {
	id := NewID("task")
	assert.True(t, IsValidID(id, "task"))
	assert.False(t, IsValidID(id, "tas"))
	assert.False(t, IsValidID(id, "tasks"))
}

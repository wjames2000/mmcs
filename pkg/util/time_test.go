package util

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNow_ReturnsUTC(t *testing.T) {
	now := Now()
	assert.Equal(t, time.UTC, now.Location(), "Now 应返回 UTC 时间")
}

func TestNowUnix_Positive(t *testing.T) {
	ts := NowUnix()
	assert.True(t, ts > 0, "Unix 时间戳应大于 0")
}

func TestNowUnixMilli_Positive(t *testing.T) {
	ts := NowUnixMilli()
	assert.True(t, ts > 0, "毫秒时间戳应大于 0")
}

func TestSinceInMs_Positive(t *testing.T) {
	start := time.Now()
	elapsed := SinceInMs(start)
	assert.True(t, elapsed >= 0, "耗时应为非负数")
}

func TestSinceInMs_AfterSleep(t *testing.T) {
	start := time.Now()
	time.Sleep(5 * time.Millisecond)
	elapsed := SinceInMs(start)
	assert.True(t, elapsed >= 5, "sleep 5ms 后耗时应 >= 5ms")
}

func TestFormatTime_ISO8601(t *testing.T) {
	now := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	result := FormatTime(now, ISO8601)
	assert.Contains(t, result, "2024-01-15T10:30:00")
}

func TestFormatTime_DateOnly(t *testing.T) {
	now := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	result := FormatTime(now, DateOnly)
	assert.Equal(t, "2024-01-15", result)
}

func TestFormatTime_DateTimeCN(t *testing.T) {
	now := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	result := FormatTime(now, DateTimeCN)
	assert.Equal(t, "2024-01-15 10:30:00", result)
}

func TestParseTime_Valid(t *testing.T) {
	parsed, err := ParseTime("2024-01-15", DateOnly)
	assert.NoError(t, err)
	assert.Equal(t, 2024, parsed.Year())
	assert.Equal(t, time.January, parsed.Month())
	assert.Equal(t, 15, parsed.Day())
}

func TestParseTime_Invalid(t *testing.T) {
	_, err := ParseTime("not-a-date", DateOnly)
	assert.Error(t, err)
}

func TestParseTimeCN_Valid(t *testing.T) {
	parsed, err := ParseTimeCN("2024-01-15 10:30:00")
	assert.NoError(t, err)
	assert.Equal(t, 2024, parsed.Year())
	assert.Equal(t, 10, parsed.Hour())
}

func TestMaxTime_Later(t *testing.T) {
	a := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	b := time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC)
	assert.Equal(t, b, MaxTime(a, b))
	assert.Equal(t, b, MaxTime(b, a))
}

func TestMinTime_Earlier(t *testing.T) {
	a := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	b := time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC)
	assert.Equal(t, a, MinTime(a, b))
	assert.Equal(t, a, MinTime(b, a))
}

func TestTruncateDay(t *testing.T) {
	dt := time.Date(2024, 1, 15, 10, 30, 45, 123, time.UTC)
	truncated := TruncateDay(dt)
	assert.Equal(t, 2024, truncated.Year())
	assert.Equal(t, time.January, truncated.Month())
	assert.Equal(t, 15, truncated.Day())
	assert.Equal(t, 0, truncated.Hour())
	assert.Equal(t, 0, truncated.Minute())
	assert.Equal(t, 0, truncated.Second())
}

func TestStartOfDay(t *testing.T) {
	dt := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	sod := StartOfDay(dt)
	assert.Equal(t, 0, sod.Hour())
	assert.Equal(t, 0, sod.Minute())
	assert.Equal(t, 0, sod.Second())
	assert.Equal(t, 0, sod.Nanosecond())
}

func TestEndOfDay(t *testing.T) {
	dt := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	eod := EndOfDay(dt)
	assert.Equal(t, 23, eod.Hour())
	assert.Equal(t, 59, eod.Minute())
	assert.Equal(t, 59, eod.Second())
	assert.Equal(t, 999999999, eod.Nanosecond())
}

func TestSleepWithContext_Cancelled(t *testing.T) {
	done := make(chan struct{})
	close(done) // 已关闭的 channel 会立即返回 false

	result := SleepWithContext(done, 100*time.Millisecond)
	assert.False(t, result, "已取消时应返回 false")
}

func TestSleepWithContext_Completed(t *testing.T) {
	done := make(chan struct{})

	result := SleepWithContext(done, 1*time.Millisecond)
	assert.True(t, result, "正常等待完成应返回 true")
}

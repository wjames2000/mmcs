package util

import "time"

const (
	// 常用时间格式
	ISO8601      = "2006-01-02T15:04:05Z07:00"
	DateOnly     = "2006-01-02"
	DateTimeCN   = "2006-01-02 15:04:05"
	TimeOnly     = "15:04:05"
	TimeOnlyCN   = "15时04分05秒"
	RFC3339Milli = "2006-01-02T15:04:05.000Z07:00"
)

// Now 返回当前 UTC 时间
func Now() time.Time {
	return time.Now().UTC()
}

// NowUnix 返回当前 Unix 时间戳（秒）
func NowUnix() int64 {
	return time.Now().Unix()
}

// NowUnixMilli 返回当前 Unix 时间戳（毫秒）
func NowUnixMilli() int64 {
	return time.Now().UnixMilli()
}

// FormatTime 格式化时间为指定布局
func FormatTime(t time.Time, layout string) string {
	return t.Format(layout)
}

// ParseTime 解析字符串为时间
func ParseTime(s, layout string) (time.Time, error) {
	return time.Parse(layout, s)
}

// ParseTimeCN 解析中文日期时间格式
func ParseTimeCN(s string) (time.Time, error) {
	return time.Parse(DateTimeCN, s)
}

// SinceInMs 返回从开始到现在经过的毫秒数
func SinceInMs(start time.Time) int64 {
	return time.Since(start).Milliseconds()
}

// MaxTime 返回两个时间中较晚的一个
func MaxTime(a, b time.Time) time.Time {
	if a.After(b) {
		return a
	}
	return b
}

// MinTime 返回两个时间中较早的一个
func MinTime(a, b time.Time) time.Time {
	if a.Before(b) {
		return a
	}
	return b
}

// TruncateDay 将时间截断到天（年月日）
func TruncateDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

// StartOfDay 返回当天 00:00:00
func StartOfDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

// EndOfDay 返回当天 23:59:59.999999999
func EndOfDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 23, 59, 59, 999999999, t.Location())
}

// SleepWithContext 可被上下文取消的休眠
func SleepWithContext(done <-chan struct{}, d time.Duration) bool {
	select {
	case <-time.After(d):
		return true
	case <-done:
		return false
	}
}

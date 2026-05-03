package stream

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewStreamReader(t *testing.T) {
	ch := make(chan *Message)
	sr := NewStreamReader("test", ch)
	assert.Equal(t, "test", sr.ID)
	assert.NotNil(t, sr.Ch)
}

func TestStreamReaderClose(t *testing.T) {
	ch := make(chan *Message)
	sr := NewStreamReader("test", ch)
	sr.Close()

	// 多次关闭不应 panic
	sr.Close()
}

func TestNewMergedStream(t *testing.T) {
	ch1 := make(chan *Message)
	ch2 := make(chan *Message)

	sr1 := NewStreamReader("stream1", ch1)
	sr2 := NewStreamReader("stream2", ch2)

	ms := NewMergedStream(sr1, sr2)
	assert.NotNil(t, ms)
	assert.NotNil(t, ms.Output())
}

func TestMergeStream_Basic(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ch1 := make(chan *Message, 10)
	ch2 := make(chan *Message, 10)

	sr1 := NewStreamReader("stream1", ch1)
	sr2 := NewStreamReader("stream2", ch2)

	ms := NewMergedStream(sr1, sr2)
	ms.Start(ctx)

	// 发送消息
	now := time.Now()
	ch1 <- &Message{Data: "msg1", CreatedAt: now, StreamID: "stream1", Seq: 1}
	ch2 <- &Message{Data: "msg2", CreatedAt: now.Add(time.Millisecond), StreamID: "stream2", Seq: 1}
	ch1 <- &Message{Data: "msg3", CreatedAt: now.Add(2 * time.Millisecond), StreamID: "stream1", Seq: 2}

	time.Sleep(100 * time.Millisecond)

	// 关闭所有输入 channel
	close(ch1)
	close(ch2)

	// 收集输出
	var received []*Message
	for msg := range ms.Output() {
		received = append(received, msg)
	}

	// 应该收到 3 条消息
	assert.Equal(t, 3, len(received), "应该收到 3 条合并消息")
}

func TestMergeStream_NoStreams(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ms := NewMergedStream()
	ms.Start(ctx)

	// 没有输入流，output 应该立即关闭
	time.Sleep(50 * time.Millisecond)
	_, ok := <-ms.Output()
	assert.False(t, ok, "没有输入流时 output 应该关闭")
}

func TestMergeStream_ConcurrentWrites(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ch1 := make(chan *Message, 100)
	ch2 := make(chan *Message, 100)

	sr1 := NewStreamReader("stream1", ch1)
	sr2 := NewStreamReader("stream2", ch2)

	ms := NewMergedStream(sr1, sr2)
	ms.Start(ctx)

	// 并发写入
	var wg sync.WaitGroup
	now := time.Now()

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(seq int) {
			defer wg.Done()
			ch1 <- &Message{
				Data:      "msg",
				CreatedAt: now.Add(time.Duration(seq) * time.Millisecond),
				StreamID:  "stream1",
				Seq:       int64(seq),
			}
		}(i)

		wg.Add(1)
		go func(seq int) {
			defer wg.Done()
			ch2 <- &Message{
				Data:      "msg",
				CreatedAt: now.Add(time.Duration(seq) * time.Millisecond),
				StreamID:  "stream2",
				Seq:       int64(seq),
			}
		}(i)
	}

	wg.Wait()
	close(ch1)
	close(ch2)

	// 收集输出
	var received []*Message
	for msg := range ms.Output() {
		received = append(received, msg)
	}

	// 应该收到 100 条消息（50+50）
	assert.Equal(t, 100, len(received), "应该收到 100 条合并消息")
}

func TestMessageBuffer(t *testing.T) {
	buf := newMessageBuffer()
	assert.Equal(t, 0, buf.Len())

	now := time.Now()
	buf.add(&Message{Data: "later", CreatedAt: now.Add(time.Second)})
	buf.add(&Message{Data: "earlier", CreatedAt: now})

	// 按时间戳顺序弹出
	first := buf.pop()
	assert.Equal(t, "earlier", first.Data)

	second := buf.pop()
	assert.Equal(t, "later", second.Data)

	// 空 buffer 弹出应返回 nil
	empty := buf.pop()
	assert.Nil(t, empty)
}

func TestMergeStream_Close(t *testing.T) {
	ch1 := make(chan *Message)
	sr1 := NewStreamReader("stream1", ch1)

	ms := NewMergedStream(sr1)
	ms.Close()

	// 多次关闭不应 panic
	ms.Close()
}

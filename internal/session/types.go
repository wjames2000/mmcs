package session

// InterruptSignal 中断信号
type InterruptSignal struct {
	NodeName string `json:"node_name"`
	Message  string `json:"message"`
	UserID   string `json:"user_id"`
}

// ResumeSignal 恢复信号
type ResumeSignal struct {
	Message string `json:"message"`
}

// SessionChannels 会话运行时 channel 集合
// 用于 Pause/Resume 与运行中的编排器通信
type SessionChannels struct {
	InterruptCh chan *InterruptSignal
	ResumeCh    chan *ResumeSignal
}

// NewSessionChannels 创建会话运行时 channel
func NewSessionChannels() *SessionChannels {
	return &SessionChannels{
		InterruptCh: make(chan *InterruptSignal, 1),
		ResumeCh:    make(chan *ResumeSignal, 1),
	}
}

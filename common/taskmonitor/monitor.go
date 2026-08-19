package taskmonitor

import (
	"time"

	"github.com/sagernet/sing-box/log"
	F "github.com/sagernet/sing/common/format"
)

type Monitor struct {
	logger  log.StructuredLogger
	timeout time.Duration
	timer   *time.Timer
}

func New(logger log.StructuredLogger, timeout time.Duration) *Monitor {
	return &Monitor{
		logger:  logger,
		timeout: timeout,
	}
}

func (m *Monitor) Start(taskName ...any) {
	m.timer = time.AfterFunc(m.timeout, func() {
		m.logger.WarnEvent("lifecycle.slow", "still running", log.String("name", F.ToString(taskName...)))
	})
}

func (m *Monitor) Finish() {
	m.timer.Stop()
}

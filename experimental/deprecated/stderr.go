package deprecated

import (
	"os"
	"strconv"
	"sync"

	"github.com/sagernet/sing-box/log"
)

type stderrManager struct {
	access   sync.Mutex
	logger   log.StructuredLogger
	reported map[string]bool
}

func NewStderrManager(logger log.StructuredLogger) Manager {
	return &stderrManager{
		logger:   logger,
		reported: make(map[string]bool),
	}
}

func (f *stderrManager) ReportDeprecated(feature Note) {
	f.access.Lock()
	defer f.access.Unlock()
	if f.reported[feature.Name] {
		return
	}
	f.reported[feature.Name] = true
	if !feature.Impending() {
		f.logger.WarnEvent("deprecated.feature", feature.MessageWithLink())
		return
	}
	if feature.EnvName != "" {
		enable, enableErr := strconv.ParseBool(os.Getenv("ENABLE_DEPRECATED_" + feature.EnvName))
		if enableErr == nil && enable {
			f.logger.WarnEvent("deprecated.feature", feature.MessageWithLink())
			return
		}
		f.logger.ErrorEvent("deprecated.feature", feature.MessageWithLink())
		f.logger.FatalEvent("deprecated.blocked", "to continuing using this feature, set environment variable ENABLE_DEPRECATED_"+feature.EnvName+"=true", log.String("env_var", "ENABLE_DEPRECATED_"+feature.EnvName))
	} else {
		f.logger.ErrorEvent("deprecated.feature", feature.MessageWithLink())
	}
}

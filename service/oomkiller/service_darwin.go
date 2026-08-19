//go:build darwin && cgo

package oomkiller

/*
#include <dispatch/dispatch.h>

static dispatch_source_t memoryPressureSource;

extern void goMemoryPressureCallback(unsigned long status);

static void startMemoryPressureMonitor() {
	memoryPressureSource = dispatch_source_create(
		DISPATCH_SOURCE_TYPE_MEMORYPRESSURE,
		0,
		DISPATCH_MEMORYPRESSURE_CRITICAL,
		dispatch_get_global_queue(QOS_CLASS_DEFAULT, 0)
	);
	dispatch_source_set_event_handler(memoryPressureSource, ^{
		unsigned long status = dispatch_source_get_data(memoryPressureSource);
		goMemoryPressureCallback(status);
	});
	dispatch_activate(memoryPressureSource);
}

static void stopMemoryPressureMonitor() {
	if (memoryPressureSource) {
		dispatch_source_cancel(memoryPressureSource);
		memoryPressureSource = NULL;
	}
}
*/
import "C"

import (
	"sync"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing/common/byteformats"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/service"
)

const oomDraftMinInterval = time.Hour

var (
	globalAccess   sync.Mutex
	globalServices []*Service
)

func (s *Service) Start(stage adapter.StartStage) error {
	if stage != adapter.StartStateStart {
		return nil
	}
	if s.timerConfig.policyMode == policyModeNetworkExtension {
		s.adaptiveTimer = newAdaptiveTimer(s.logger, s.network, s.timerConfig, nil)
		globalAccess.Lock()
		isFirst := len(globalServices) == 0
		globalServices = append(globalServices, s)
		globalAccess.Unlock()
		if isFirst {
			C.startMemoryPressureMonitor()
		}
		return nil
	}
	if !s.timerConfig.policyMode.hasTimerMode() {
		return E.New("memory pressure monitoring is not available on this platform without memory_limit")
	}
	s.adaptiveTimer = newAdaptiveTimer(s.logger, s.network, s.timerConfig, s.writeOOMReport)
	s.adaptiveTimer.start()
	return nil
}

func (s *Service) Close() error {
	if s.adaptiveTimer != nil {
		s.adaptiveTimer.stop()
	}
	if s.timerConfig.policyMode == policyModeNetworkExtension {
		globalAccess.Lock()
		for i, svc := range globalServices {
			if svc == s {
				globalServices = append(globalServices[:i], globalServices[i+1:]...)
				break
			}
		}
		isLast := len(globalServices) == 0
		globalAccess.Unlock()
		if isLast {
			C.stopMemoryPressureMonitor()
		}
		s.discardOOMDraft()
	}
	return nil
}

//export goMemoryPressureCallback
func goMemoryPressureCallback(status C.ulong) {
	globalAccess.Lock()
	services := make([]*Service, len(globalServices))
	copy(services, globalServices)
	globalAccess.Unlock()
	if len(services) == 0 {
		return
	}
	sample := readMemorySample(policyModeNetworkExtension)
	for _, s := range services {
		s.logger.WarnEvent("oom.pressure.critical", "memory pressure critical", log.String("usage", byteformats.FormatMemoryBytes(sample.usage)))
		s.writeOOMDraft(sample.usage)
		s.adaptiveTimer.notifyPressure()
	}
}

func (s *Service) writeOOMDraft(memoryUsage uint64) {
	if s.draftCancelled.Load() {
		return
	}
	now := time.Now().UnixNano()
	lastDraft := s.lastDraftTime.Load()
	if time.Duration(now-lastDraft) < oomDraftMinInterval {
		return
	}
	s.lastDraftTime.Store(now)
	reporter := service.FromContext[OOMReporter](s.ctx)
	if reporter == nil {
		return
	}
	err := reporter.WriteDraft(memoryUsage)
	if s.draftCancelled.Load() {
		reporter.DiscardDraft()
		return
	}
	if err != nil {
		s.logger.ErrorEvent("oom.draft.error", "write OOM draft", log.Err(err))
	} else {
		s.logger.WarnEvent("oom.draft.saved", "OOM draft saved")
	}
}

func (s *Service) discardOOMDraft() {
	s.draftCancelled.Store(true)
	reporter := service.FromContext[OOMReporter](s.ctx)
	if reporter == nil {
		return
	}
	err := reporter.DiscardDraft()
	if err != nil {
		s.logger.ErrorEvent("oom.draft.error", "discard OOM draft", log.Err(err))
	}
}

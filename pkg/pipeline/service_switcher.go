// Package pipeline provides ServiceSwitcher and LLMSwitcher for runtime service switching.
package pipeline

import (
	"context"
	"fmt"
	"sync"

	"voila-go/pkg/frames"
	"voila-go/pkg/logger"
	"voila-go/pkg/processors"
)

// ServiceSwitcherStrategy decides which service is active and handles switch frames.
type ServiceSwitcherStrategy interface {
	ActiveServiceName() string
	// HandleFrame handles a service switcher frame (e.g. ManuallySwitchServiceFrame).
	// Returns true if the active service was changed.
	HandleFrame(ctx context.Context, f frames.Frame, dir processors.Direction) (switched bool)
}

// ServiceSwitcherStrategyManual keeps the first service active until a ManuallySwitchServiceFrame requests a switch.
type ServiceSwitcherStrategyManual struct {
	mu            sync.Mutex
	serviceNames  []string
	activeIndex   int
}

// NewServiceSwitcherStrategyManual returns a strategy that starts with the first service and switches on ManuallySwitchServiceFrame.
func NewServiceSwitcherStrategyManual(serviceNames []string) *ServiceSwitcherStrategyManual {
	if len(serviceNames) == 0 {
		panic("ServiceSwitcherStrategyManual needs at least one service name")
	}
	return &ServiceSwitcherStrategyManual{serviceNames: serviceNames, activeIndex: 0}
}

// ActiveServiceName returns the currently active service name.
func (s *ServiceSwitcherStrategyManual) ActiveServiceName() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.activeIndex < 0 || s.activeIndex >= len(s.serviceNames) {
		return ""
	}
	return s.serviceNames[s.activeIndex]
}

// HandleFrame implements ServiceSwitcherStrategy. On ManuallySwitchServiceFrame, switches if the name is in the list.
func (s *ServiceSwitcherStrategyManual) HandleFrame(ctx context.Context, f frames.Frame, dir processors.Direction) (switched bool) {
	m, ok := f.(*frames.ManuallySwitchServiceFrame)
	if !ok {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, name := range s.serviceNames {
		if name == m.ServiceName {
			s.activeIndex = i
			logger.Info("service switcher: switched to %q", m.ServiceName)
			return true
		}
	}
	return false
}

func isServiceSwitcherFrame(f frames.Frame) bool {
	if f == nil {
		return false
	}
	_, ok := f.(*frames.ManuallySwitchServiceFrame)
	return ok
}

// ServiceSwitcher is a ParallelPipeline that routes frames to one active service at a time.
// Each branch is [Filter(downstream), service, Filter(upstream)]; only the active branch passes frames.
type ServiceSwitcher struct {
	*ParallelPipeline
	strategy     ServiceSwitcherStrategy
	serviceNames []string
}

// NewServiceSwitcher builds a service switcher from a list of named processors and a strategy.
func NewServiceSwitcher(services []struct {
	Name      string
	Processor processors.Processor
}, strategy ServiceSwitcherStrategy) (*ServiceSwitcher, error) {
	if len(services) == 0 {
		return nil, fmt.Errorf("ServiceSwitcher needs at least one service")
	}
	if strategy == nil {
		return nil, fmt.Errorf("ServiceSwitcher needs a strategy")
	}
	names := make([]string, len(services))
	branches := make([][]processors.Processor, len(services))
	for i, svc := range services {
		names[i] = svc.Name
		activeName := names[i]
		filterDown := processors.NewFilterProcessor(
			fmt.Sprintf("ServiceSwitcher::FilterDown::%s", activeName),
			func(ctx context.Context, f frames.Frame, dir processors.Direction) bool {
				return dir == processors.Downstream && strategy.ActiveServiceName() == activeName
			},
		)
		filterUp := processors.NewFilterProcessor(
			fmt.Sprintf("ServiceSwitcher::FilterUp::%s", activeName),
			func(ctx context.Context, f frames.Frame, dir processors.Direction) bool {
				return dir == processors.Upstream && strategy.ActiveServiceName() == activeName
			},
		)
		branches[i] = []processors.Processor{filterDown, svc.Processor, filterUp}
	}
	pp, err := NewParallelPipeline(branches)
	if err != nil {
		return nil, err
	}
	ss := &ServiceSwitcher{ParallelPipeline: pp, strategy: strategy, serviceNames: names}
	pp.SetOutputFilter(ss.outputFilter)
	return ss, nil
}

func (ss *ServiceSwitcher) outputFilter(f frames.Frame) bool {
	active := ss.strategy.ActiveServiceName()
	if req, ok := f.(*frames.ServiceSwitcherRequestMetadataFrame); ok && req.ServiceName == active {
		return false
	}
	if meta, ok := f.(*frames.ServiceMetadataFrame); ok && meta.ServiceName != active {
		return false
	}
	return true
}

// ProcessFrame implements Processor. Intercepts ManuallySwitchServiceFrame and requests metadata on switch.
func (ss *ServiceSwitcher) ProcessFrame(ctx context.Context, f frames.Frame, dir processors.Direction) error {
	if dir == processors.Upstream {
		if ss.Prev() != nil {
			return ss.Prev().ProcessFrame(ctx, f, dir)
		}
		return nil
	}
	if isServiceSwitcherFrame(f) {
		if ss.strategy.HandleFrame(ctx, f, dir) {
			active := ss.strategy.ActiveServiceName()
			_ = ss.ParallelPipeline.Push(ctx, frames.NewServiceSwitcherRequestMetadataFrame(active))
		}
		return nil
	}
	return ss.ParallelPipeline.ProcessFrame(ctx, f, dir)
}

// LLMSwitcher is a ServiceSwitcher for LLM services. It uses the same mechanics; use NewServiceSwitcher
// with LLM processor instances and a strategy. This type exists for API parity with the Python pipecat module.
type LLMSwitcher struct {
	*ServiceSwitcher
}

// NewLLMSwitcher builds an LLMSwitcher from named LLM processors and a strategy.
func NewLLMSwitcher(services []struct {
	Name      string
	Processor processors.Processor
}, strategy ServiceSwitcherStrategy) (*LLMSwitcher, error) {
	ss, err := NewServiceSwitcher(services, strategy)
	if err != nil {
		return nil, err
	}
	return &LLMSwitcher{ServiceSwitcher: ss}, nil
}

var _ processors.Processor = (*ServiceSwitcher)(nil)

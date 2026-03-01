// Package pipeline provides a processor registry for dynamic loading by name from config.
package pipeline

import (
	"encoding/json"
	"fmt"
	"sync"

	"voila-go/pkg/config"
	"voila-go/pkg/processors"
)

// ProcessorConstructor builds a processor from a name and optional JSON options (nil = use defaults).
type ProcessorConstructor func(name string, opts json.RawMessage) processors.Processor

var (
	registry   = make(map[string]ProcessorConstructor)
	registryMu sync.RWMutex
)

// RegisterProcessor registers a processor constructor by name. Used for dynamic loading from config.
// The constructor receives opts from cfg.PluginOptions[name]; opts may be nil.
func RegisterProcessor(name string, ctor ProcessorConstructor) {
	registryMu.Lock()
	defer registryMu.Unlock()
	registry[name] = ctor
}

// ProcessorsFromConfig returns a slice of processors for the plugin names in cfg.Plugins.
// Unknown names return an error. Each constructor receives opts from cfg.PluginOptions[name] (may be nil).
func ProcessorsFromConfig(cfg *config.Config) ([]processors.Processor, error) {
	if cfg == nil {
		return nil, nil
	}
	registryMu.RLock()
	defer registryMu.RUnlock()
	out := make([]processors.Processor, 0, len(cfg.Plugins))
	for _, name := range cfg.Plugins {
		ctor, ok := registry[name]
		if !ok {
			return nil, fmt.Errorf("unknown processor/plugin: %q", name)
		}
		var opts json.RawMessage
		if cfg.PluginOptions != nil {
			opts = cfg.PluginOptions[name]
		}
		out = append(out, ctor(name, opts))
	}
	return out, nil
}

// AddFromConfig appends processors to the pipeline from cfg.Plugins (by name). Processors must be registered first.
func (p *Pipeline) AddFromConfig(cfg *config.Config) error {
	procs, err := ProcessorsFromConfig(cfg)
	if err != nil {
		return err
	}
	for _, proc := range procs {
		p.Add(proc)
	}
	return nil
}

package plugin

import (
	"fmt"
)

// Plugin defines an interface for any custom processing module.
type Plugin interface {
	Name() string
	Process(input string) (string, error)
}

// Registry keeps track of available plugins.
var Registry = make(map[string]Plugin)

// RegisterPlugin adds a plugin to the registry.
func RegisterPlugin(p Plugin) {
	name := p.Name()
	Registry[name] = p
	fmt.Printf("Registered plugin: %s\n", name)
}

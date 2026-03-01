package plugin_test

import (
	"testing"

	"voila-go/pkg/plugin"
)

func TestBuild(t *testing.T) {
	if plugin.Registry == nil {
		t.Fatal("Registry should be non-nil")
	}
}

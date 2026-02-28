package plugin

import "testing"

func TestBuild(t *testing.T) {
	if Registry == nil {
		t.Fatal("Registry should be non-nil")
	}
}

package sarvam

import (
	"fmt"
	"runtime"
)

const (
	// DefaultBaseURL is the default Sarvam AI API base URL.
	DefaultBaseURL = "https://api.sarvam.ai"
)

// sdkHeaders returns identification headers similar to the Python SDK's sdk_headers().
func sdkHeaders() map[string]string {
	return map[string]string{
		"User-Agent": fmt.Sprintf("Voxray-Go/dev Go/%s", runtime.Version()),
	}
}


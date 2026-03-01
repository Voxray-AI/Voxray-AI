package filters

import "voila-go/pkg/frames"

// isLifecycleFrame returns true for Start, End, Cancel, Stop (and Error).
// These frames are always passed through by frame_filter, null_filter, function_filter.
func isLifecycleFrame(f frames.Frame) bool {
	switch f.(type) {
	case *frames.StartFrame, *frames.EndFrame, *frames.CancelFrame, *frames.StopFrame, *frames.ErrorFrame:
		return true
	}
	return false
}

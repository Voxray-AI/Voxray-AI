package utils_test

import (
	"testing"
	"time"

	"voxray-go/pkg/utils"
)

func TestExponentialBackoff(t *testing.T) {
	// attempt 0 or negative treated as 1 -> 2^1 = 2s
	if d := utils.ExponentialBackoff(0); d != 2*time.Second {
		t.Errorf("ExponentialBackoff(0) = %v, want 2s", d)
	}
	if d := utils.ExponentialBackoff(-1); d != 2*time.Second {
		t.Errorf("ExponentialBackoff(-1) = %v, want 2s", d)
	}
	// 1 -> 2s, 2 -> 4s, etc.
	if d := utils.ExponentialBackoff(1); d != 2*time.Second {
		t.Errorf("ExponentialBackoff(1) = %v, want 2s", d)
	}
	if d := utils.ExponentialBackoff(2); d != 4*time.Second {
		t.Errorf("ExponentialBackoff(2) = %v, want 4s", d)
	}
	// cap at 60s
	if d := utils.ExponentialBackoff(10); d != utils.BackoffCap {
		t.Errorf("ExponentialBackoff(10) = %v, want BackoffCap %v", d, utils.BackoffCap)
	}
}

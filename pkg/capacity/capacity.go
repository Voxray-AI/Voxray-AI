// Package capacity provides memory-based capacity checks for session admission (e.g. system and process memory).
package capacity

import (
	"bufio"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	// DefaultCacheTTL is the TTL for cached memory reads to avoid thundering herd near the limit.
	DefaultCacheTTL = 3 * time.Second
)

var (
	// System memory cache (Linux /proc/meminfo).
	sysMemCache struct {
		usedPercent float64
		err        error
		at         time.Time
		mu         sync.Mutex
	}
	// Process memory cache (runtime.ReadMemStats).
	processMemCache struct {
		heapSysMB uint64
		at        time.Time
		mu        sync.Mutex
	}
)

// SystemMemoryUsedPercent returns system memory used as a percentage (0-100).
// On Linux uses MemAvailable from /proc/meminfo: used = (MemTotal - MemAvailable) / MemTotal * 100.
// MemAvailable is used (not MemFree) because it includes reclaimable page cache.
// Result is cached for DefaultCacheTTL. On error (e.g. non-Linux or read failure) returns 0, err.
func SystemMemoryUsedPercent(cacheTTL time.Duration) (float64, error) {
	sysMemCache.mu.Lock()
	defer sysMemCache.mu.Unlock()
	if cacheTTL > 0 && time.Since(sysMemCache.at) < cacheTTL && (sysMemCache.err != nil || sysMemCache.usedPercent >= 0) {
		return sysMemCache.usedPercent, sysMemCache.err
	}
	used, err := systemMemoryUsedPercentLinux()
	sysMemCache.usedPercent = used
	sysMemCache.err = err
	sysMemCache.at = time.Now()
	return used, err
}

// systemMemoryUsedPercentLinux reads /proc/meminfo and returns (MemTotal - MemAvailable) / MemTotal * 100.
func systemMemoryUsedPercentLinux() (float64, error) {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return 0, err
	}
	defer f.Close()
	var total, available uint64
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(line, "MemTotal:") {
			total = parseMemInfoKB(line)
		} else if strings.HasPrefix(line, "MemAvailable:") {
			available = parseMemInfoKB(line)
			break
		}
	}
	if err := sc.Err(); err != nil {
		return 0, err
	}
	if total == 0 {
		return 0, nil
	}
	used := total - available
	if used > total {
		used = total
	}
	return 100.0 * float64(used) / float64(total), nil
}

func parseMemInfoKB(line string) uint64 {
	fields := strings.Fields(line)
	if len(fields) < 2 {
		return 0
	}
	v, _ := strconv.ParseUint(fields[1], 10, 64)
	return v
}

// ProcessHeapSysMB returns the process heap/system memory in MB (from runtime.ReadMemStats).
// Cached for DefaultCacheTTL.
func ProcessHeapSysMB(cacheTTL time.Duration) uint64 {
	processMemCache.mu.Lock()
	defer processMemCache.mu.Unlock()
	if cacheTTL > 0 && time.Since(processMemCache.at) < cacheTTL {
		return processMemCache.heapSysMB
	}
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	processMemCache.heapSysMB = m.Sys / (1024 * 1024)
	processMemCache.at = time.Now()
	return processMemCache.heapSysMB
}

// Hysteresis holds state for memory hysteresis: reject when >= high, resume when < low (high - hysteresis).
type Hysteresis struct {
	mu   sync.Mutex
	over bool // true once we've exceeded threshold; cleared when we drop below threshold - hysteresis
}

// Allow returns true if a new session should be allowed based on current usedPercent.
// threshold is the reject-above value (e.g. 80); hysteresisPercent is the band (e.g. 5 → resume at 75).
// Reject when usedPercent >= threshold; once rejecting, allow again only when usedPercent < threshold - hysteresisPercent.
func (h *Hysteresis) Allow(usedPercent, threshold, hysteresisPercent float64) bool {
	if threshold <= 0 {
		return true
	}
	low := threshold - hysteresisPercent
	if low < 0 {
		low = 0
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if usedPercent >= threshold {
		h.over = true
		return false
	}
	if usedPercent < low {
		h.over = false
	}
	return !h.over
}

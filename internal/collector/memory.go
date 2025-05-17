package collector

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// MemInfo represents data parsed from /proc/meminfo
type MemInfo struct {
	MemTotal     uint64 // kB
	MemFree      uint64 // kB
	MemAvailable uint64 // kB (More useful than MemFree)
	Buffers      uint64 // kB
	Cached       uint64 // kB
	SwapTotal    uint64 // kB
	SwapFree     uint64 // kB
}

func parseMemInfo() (*MemInfo, error) {
	file, err := os.Open("/proc/meminfo")
	if err != nil {
		return nil, fmt.Errorf("failed to open /proc/meminfo: %w", err)
	}
	defer file.Close()

	info := &MemInfo{}
	scanner := bufio.NewScanner(file)
	requiredFields := map[string]*uint64{
		"MemTotal:":     &info.MemTotal,
		"MemFree:":      &info.MemFree,
		"MemAvailable:": &info.MemAvailable,
		"Buffers:":      &info.Buffers,
		"Cached:":       &info.Cached,
		"SwapTotal:":    &info.SwapTotal,
		"SwapFree:":     &info.SwapFree,
	}
	foundCount := 0

	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}

		fieldName := parts[0]
		if ptr, ok := requiredFields[fieldName]; ok {
			val, err := strconv.ParseUint(parts[1], 10, 64)
			if err == nil {
				*ptr = val
				foundCount++
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error scanning /proc/meminfo: %w", err)
	}
	if foundCount < len(requiredFields) {
		// If MemAvailable is missing (older kernels), we might try to calculate it
		if info.MemAvailable == 0 && info.MemTotal > 0 {
			// This is a rough estimate, modern kernels provide MemAvailable directly
			// For simplicity, we'll rely on MemAvailable being present.
			// If not, mem_percent_used calculation would be less accurate.
		}
		// Log a warning or error if not all fields are found, but proceed if essential ones are.
	}


	return info, nil
}

// CollectMemoryStats gathers memory and swap usage statistics.
func CollectMemoryStats() (CollectedMetrics, error) {
	memInfo, err := parseMemInfo()
	if err != nil {
		return nil, err
	}

	metrics := make(CollectedMetrics)

	// Memory
	if memInfo.MemTotal > 0 {
		var usedMemPercentage float64
		if memInfo.MemAvailable > 0 { // Prefer MemAvailable for 'used' calculation
			usedMemPercentage = (1.0 - float64(memInfo.MemAvailable)/float64(memInfo.MemTotal)) * 100.0
		} else { // Fallback if MemAvailable is not present (older kernels)
			// Used = Total - Free - Buffers - Cached (This is a common interpretation)
			// However, Buffers and Cached are reclaimable. Using (Total - Free) is too simplistic.
			// (Total - Free - (Buffers + Cached)) is one way, but MemAvailable is better.
			// For simplicity, if MemAvailable is 0, we use Total - Free.
			usedMemPercentage = (1.0 - float64(memInfo.MemFree)/float64(memInfo.MemTotal)) * 100.0
		}
		metrics["mem_percent_used"] = usedMemPercentage
		metrics["mem_percent_free"] = (float64(memInfo.MemAvailable)/float64(memInfo.MemTotal)) * 100.0 // Based on MemAvailable
	} else {
		metrics["mem_percent_used"] = 0
		metrics["mem_percent_free"] = 0
	}

	// Swap
	if memInfo.SwapTotal > 0 {
		swapUsed := memInfo.SwapTotal - memInfo.SwapFree
		metrics["swap_percent_used"] = (float64(swapUsed) / float64(memInfo.SwapTotal)) * 100.0
		metrics["swap_percent_free"] = (float64(memInfo.SwapFree) / float64(memInfo.SwapTotal)) * 100.0
	} else {
		metrics["swap_percent_used"] = 0
		metrics["swap_percent_free"] = 0
	}

	return metrics, nil
}


func NewMemoryCollector() MetricCollector {
	return &memoryCollectorAdaptor{}
}
type memoryCollectorAdaptor struct{}
func (mca *memoryCollectorAdaptor) Collect() (CollectedMetrics, error) { return CollectMemoryStats() }
func (mca *memoryCollectorAdaptor) Name() string                       { return "memory" }

package collector

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
)

// Store previous CPU times to calculate usage delta.
var (
	prevCPUTotal uint64
	prevCPUIdle  uint64
	cpuOnce      sync.Once
	cpuMu        sync.Mutex
)

// CPUStats stores values from /proc/stat for the 'cpu' line.
type CPUStatLine struct {
	User      uint64
	Nice      uint64
	System    uint64
	Idle      uint64
	IOWait    uint64
	IRQ       uint64
	SoftIRQ   uint64
	Steal     uint64
	Guest     uint64
	GuestNice uint64
}

func parseCPUStatLine(line string) (*CPUStatLine, error) {
	fields := strings.Fields(line)
	if len(fields) < 9 || fields[0] != "cpu" { // Need at least user, nice, system, idle, iowait, irq, softirq, steal
		return nil, fmt.Errorf("invalid cpu stat line format")
	}

	var s CPUStatLine
	var err error

	s.User, err = strconv.ParseUint(fields[1], 10, 64)
	if err != nil { return nil, err }
	s.Nice, err = strconv.ParseUint(fields[2], 10, 64)
	if err != nil { return nil, err }
	s.System, err = strconv.ParseUint(fields[3], 10, 64)
	if err != nil { return nil, err }
	s.Idle, err = strconv.ParseUint(fields[4], 10, 64)
	if err != nil { return nil, err }
	if len(fields) > 5 { s.IOWait, _ = strconv.ParseUint(fields[5], 10, 64) }
	if len(fields) > 6 { s.IRQ, _ = strconv.ParseUint(fields[6], 10, 64) }
	if len(fields) > 7 { s.SoftIRQ, _ = strconv.ParseUint(fields[7], 10, 64) }
	if len(fields) > 8 { s.Steal, _ = strconv.ParseUint(fields[8], 10, 64) }
	if len(fields) > 9 { s.Guest, _ = strconv.ParseUint(fields[9], 10, 64) }
	if len(fields) > 10 { s.GuestNice, _ = strconv.ParseUint(fields[10], 10, 64) }

	return &s, nil
}

func getCPUTimes() (totalTime, idleTime uint64, err error) {
	file, err := os.Open("/proc/stat")
	if err != nil {
		return 0, 0, fmt.Errorf("failed to open /proc/stat: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	if scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "cpu ") {
			stats, err := parseCPUStatLine(line)
			if err != nil {
				return 0, 0, fmt.Errorf("failed to parse cpu line from /proc/stat: %w", err)
			}

			// Total time is sum of all times except Guest and GuestNice if they are already included in User and Nice
			// More accurately, total = user + nice + system + idle + iowait + irq + softirq + steal
			total := stats.User + stats.Nice + stats.System + stats.Idle + stats.IOWait + stats.IRQ + stats.SoftIRQ + stats.Steal
			// Some consider IOWait as idle, others as busy. Common to include in idle for overall usage.
			// idle := stats.Idle + stats.IOWait
			// For strict CPU busy, idle is just stats.Idle. Let's use simple idle.
			idle := stats.Idle
			return total, idle, nil
		}
	}
	if err := scanner.Err(); err != nil {
		return 0, 0, fmt.Errorf("error scanning /proc/stat: %w", err)
	}
	return 0, 0, fmt.Errorf("cpu line not found in /proc/stat")
}


// CollectCPUStats returns total CPU usage percentage.
// This function is stateful and needs to be called sequentially.
func CollectCPUStats(elapsedHint float64) (CollectedMetrics, error) {
	cpuMu.Lock()
	defer cpuMu.Unlock()

	metrics := make(CollectedMetrics)

	currentTotal, currentIdle, err := getCPUTimes()
	if err != nil {
		return nil, err
	}

	// On the first run, we can't calculate a percentage, so store and return 0 or error.
	// For simplicity, we'll allow it to report 0 on the first valid run if prev values are 0.
	// The caller (GlobalCollector) manages the elapsed time, so it won't call with elapsedHint=0 after the first time.

	if prevCPUTotal == 0 && prevCPUIdle == 0 && elapsedHint <= 0 { // Very first call
		prevCPUTotal = currentTotal
		prevCPUIdle = currentIdle
		metrics["cpu_percent_total"] = 0.0 // Cannot calculate on first sample
		return metrics, nil
	}


	deltaTotal := currentTotal - prevCPUTotal
	deltaIdle := currentIdle - prevCPUIdle

	prevCPUTotal = currentTotal
	prevCPUIdle = currentIdle

	if deltaTotal == 0 { // No change in ticks, or time warped backwards.
		metrics["cpu_percent_total"] = 0.0
	} else {
		cpuUsage := (1.0 - float64(deltaIdle)/float64(deltaTotal)) * 100.0
		if cpuUsage < 0 { cpuUsage = 0.0 } // Cap at 0 if deltaIdle > deltaTotal (e.g. time skew)
		if cpuUsage > 100 { cpuUsage = 100.0 } // Cap at 100
		metrics["cpu_percent_total"] = cpuUsage
	}

	return metrics, nil
}

// For unit testing or direct use if GlobalCollector doesn't handle initialization
func NewCPUCollector() MetricCollector {
	return &cpuCollectorAdaptor{}
}

type cpuCollectorAdaptor struct{}

func (cca *cpuCollectorAdaptor) Collect() (CollectedMetrics, error) {
	// This simplified adapter implies CollectCPUStats is called by GlobalCollector which manages elapsed time
	// For standalone, it would need its own prev time tracking.
	// We rely on GlobalCollector's elapsedSeconds calculation for now.
	// A truly independent CPUCollector would need its own lastCollectTime.
	// For the given design, GlobalCollector is managing state for rates, which is fine.
	return CollectCPUStats(1) // Dummy elapsed, actual elapsed is handled by GlobalCollector
}

func (cca *cpuCollectorAdaptor) Name() string {
	return "cpu"
}

package collector

import (
	"bufio"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
)

// NetworkStats holds aggregated network I/O counters from /proc/net/dev.
type NetworkStats struct {
	TotalRecvBytes uint64
	TotalSentBytes uint64
}

// NetworkInterfaceFilter holds the configuration for filtering network interfaces.
type NetworkInterfaceFilter struct {
	ExcludeInterfaces []string
	ExcludePrefixes   []string
}

// DefaultNetworkInterfaceFilter returns the default filter that excludes
// loopback and Docker-related interfaces to avoid double-counting.
func DefaultNetworkInterfaceFilter() NetworkInterfaceFilter {
	return NetworkInterfaceFilter{
		ExcludeInterfaces: []string{"lo", "docker0"},
		ExcludePrefixes:   []string{"veth", "br-", "docker"},
	}
}

// isRelevantInterface checks if the interface should be monitored based on the filter.
func isRelevantInterface(ifaceName string, filter NetworkInterfaceFilter) bool {
	// Check exact match exclusions
	for _, excluded := range filter.ExcludeInterfaces {
		if ifaceName == excluded {
			return false
		}
	}
	// Check prefix exclusions
	for _, prefix := range filter.ExcludePrefixes {
		if strings.HasPrefix(ifaceName, prefix) {
			return false
		}
	}
	return true
}

// GetNetworkStats reads /proc/net/dev and aggregates received/transmitted bytes.
// It uses the provided filter to exclude certain interfaces.
func GetNetworkStats(filter NetworkInterfaceFilter) (*NetworkStats, error) {
	file, err := os.Open("/proc/net/dev")
	if err != nil {
		return nil, fmt.Errorf("failed to open /proc/net/dev: %w", err)
	}
	defer file.Close()

	stats := &NetworkStats{}
	scanner := bufio.NewScanner(file)

	// Skip header lines
	for i := 0; i < 2; i++ {
		if !scanner.Scan() {
			if err := scanner.Err(); err != nil {
				return nil, fmt.Errorf("error reading header from /proc/net/dev: %w", err)
			}
			return nil, fmt.Errorf("unexpected EOF reading /proc/net/dev header")
		}
	}

	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(strings.ReplaceAll(line, ":", " ")) // Replace colon for easier field split
		if len(fields) < 10 { // Interface name, RecvBytes, RecvPackets, ..., SentBytes, SentPackets, ...
			continue
		}

		ifaceName := fields[0]
		if !isRelevantInterface(ifaceName, filter) {
			continue
		}

		// Received bytes is the 1st field after name (index 1 if name is 0)
		recvBytes, err := strconv.ParseUint(fields[1], 10, 64)
		if err != nil {
			// log.Printf("Warning: could not parse recv_bytes for %s: %v", ifaceName, err)
			continue
		}
		// Transmitted bytes is the 8th field after name (index 9 if name is 0, but after split it's index 8 after name)
		// fields: face |bytes    packets errs drop fifo frame compressed multicast|bytes    packets errs drop fifo colls carrier compressed
		//         0     1        2       3    4    5    6     7          8         9       10
		// After splitting by space and ':', fields are:
		// <iface_name> <recv_bytes> <recv_packets> ... <sent_bytes> ...
		// So, if fields[0] is iface_name, fields[1] is recv_bytes, fields[9] is sent_bytes
		sentBytes, err := strconv.ParseUint(fields[9], 10, 64) // Index 9 after splitting with multiple spaces
		if err != nil {
			// log.Printf("Warning: could not parse sent_bytes for %s: %v", ifaceName, err)
			continue
		}

		stats.TotalRecvBytes += recvBytes
		stats.TotalSentBytes += sentBytes
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error scanning /proc/net/dev: %w", err)
	}
	return stats, nil
}

// CalculateNetworkIORates computes received/sent bytes per second.
func CalculateNetworkIORates(prev, curr NetworkStats, elapsedSeconds float64) (recvBytesPs, sentBytesPs float64) {
	if elapsedSeconds <= 0 {
		return 0, 0
	}

	var deltaRecvBytes, deltaSentBytes uint64

	// Handle counter wrap-around (unsigned 64-bit integers)
	if curr.TotalRecvBytes >= prev.TotalRecvBytes {
		deltaRecvBytes = curr.TotalRecvBytes - prev.TotalRecvBytes
	} else {
		// Counter wrapped around: delta = (MaxUint64 - prev) + curr + 1
		deltaRecvBytes = (math.MaxUint64 - prev.TotalRecvBytes) + curr.TotalRecvBytes + 1
	}

	if curr.TotalSentBytes >= prev.TotalSentBytes {
		deltaSentBytes = curr.TotalSentBytes - prev.TotalSentBytes
	} else {
		// Counter wrapped around: delta = (MaxUint64 - prev) + curr + 1
		deltaSentBytes = (math.MaxUint64 - prev.TotalSentBytes) + curr.TotalSentBytes + 1
	}


	recvBps := float64(deltaRecvBytes) / elapsedSeconds
	sentBps := float64(deltaSentBytes) / elapsedSeconds

	return recvBps, sentBps
}

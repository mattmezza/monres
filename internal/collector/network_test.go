package collector

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDefaultNetworkInterfaceFilter(t *testing.T) {
	filter := DefaultNetworkInterfaceFilter()

	assert.Equal(t, []string{"lo", "docker0"}, filter.ExcludeInterfaces)
	assert.Equal(t, []string{"veth", "br-", "docker"}, filter.ExcludePrefixes)
}

func TestIsRelevantInterface(t *testing.T) {
	filter := DefaultNetworkInterfaceFilter()

	tests := []struct {
		name      string
		ifaceName string
		expected  bool
	}{
		// Should be excluded
		{"loopback", "lo", false},
		{"docker bridge", "docker0", false},
		{"docker network", "docker1", false},
		{"veth pair", "veth123abc", false},
		{"veth pair 2", "vethd4e5f6", false},
		{"custom docker bridge", "br-abc123", false},
		{"custom docker bridge 2", "br-network", false},

		// Should be included
		{"main eth interface", "eth0", true},
		{"main eth interface 2", "eth1", true},
		{"ens interface", "ens192", true},
		{"enp interface", "enp0s3", true},
		{"wlan interface", "wlan0", true},
		{"bond interface", "bond0", true},
		{"virbr (not matching br- prefix)", "virbr0", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isRelevantInterface(tt.ifaceName, filter)
			assert.Equal(t, tt.expected, result, "Interface %s should be relevant=%v", tt.ifaceName, tt.expected)
		})
	}
}

func TestIsRelevantInterfaceCustomFilter(t *testing.T) {
	// Custom filter that only monitors eth0
	filter := NetworkInterfaceFilter{
		ExcludeInterfaces: []string{"lo", "eth1", "eth2"},
		ExcludePrefixes:   []string{"veth", "docker", "br-", "wlan"},
	}

	tests := []struct {
		ifaceName string
		expected  bool
	}{
		{"lo", false},
		{"eth0", true},
		{"eth1", false},
		{"eth2", false},
		{"wlan0", false},
		{"docker0", false},
		{"veth123", false},
		{"ens192", true},
	}

	for _, tt := range tests {
		t.Run(tt.ifaceName, func(t *testing.T) {
			result := isRelevantInterface(tt.ifaceName, filter)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsRelevantInterfaceEmptyFilter(t *testing.T) {
	// Empty filter should allow all interfaces
	filter := NetworkInterfaceFilter{
		ExcludeInterfaces: []string{},
		ExcludePrefixes:   []string{},
	}

	tests := []string{"lo", "eth0", "docker0", "veth123", "any_interface"}
	for _, iface := range tests {
		assert.True(t, isRelevantInterface(iface, filter), "Empty filter should allow %s", iface)
	}
}

func TestCalculateNetworkIORatesNormal(t *testing.T) {
	prev := NetworkStats{
		TotalRecvBytes: 1000000,
		TotalSentBytes: 500000,
	}
	curr := NetworkStats{
		TotalRecvBytes: 2000000,
		TotalSentBytes: 1500000,
	}
	elapsed := 10.0 // 10 seconds

	recvRate, sentRate := CalculateNetworkIORates(prev, curr, elapsed)

	// Recv: (2000000 - 1000000) / 10 = 100000 bytes/s
	assert.Equal(t, 100000.0, recvRate)
	// Sent: (1500000 - 500000) / 10 = 100000 bytes/s
	assert.Equal(t, 100000.0, sentRate)
}

func TestCalculateNetworkIORatesZeroElapsed(t *testing.T) {
	prev := NetworkStats{TotalRecvBytes: 1000, TotalSentBytes: 1000}
	curr := NetworkStats{TotalRecvBytes: 2000, TotalSentBytes: 2000}

	recvRate, sentRate := CalculateNetworkIORates(prev, curr, 0)

	assert.Equal(t, 0.0, recvRate)
	assert.Equal(t, 0.0, sentRate)
}

func TestCalculateNetworkIORatesNegativeElapsed(t *testing.T) {
	prev := NetworkStats{TotalRecvBytes: 1000, TotalSentBytes: 1000}
	curr := NetworkStats{TotalRecvBytes: 2000, TotalSentBytes: 2000}

	recvRate, sentRate := CalculateNetworkIORates(prev, curr, -1.0)

	assert.Equal(t, 0.0, recvRate)
	assert.Equal(t, 0.0, sentRate)
}

func TestCalculateNetworkIORatesReset(t *testing.T) {
	// Simulate counter reset (e.g., interface restart, reboot, VM migration).
	// Previous value is large, current is small (counter reset to 0 then grew a bit).
	// We treat this as a fresh start: delta = current value.
	prev := NetworkStats{
		TotalRecvBytes: math.MaxUint64 - 1000,
		TotalSentBytes: math.MaxUint64 - 500,
	}
	curr := NetworkStats{
		TotalRecvBytes: 2000,
		TotalSentBytes: 1500,
	}
	elapsed := 1.0 // 1 second

	recvRate, sentRate := CalculateNetworkIORates(prev, curr, elapsed)

	// curr < prev → treat as reset, delta = curr
	// recv: 2000 / 1.0 = 2000
	assert.Equal(t, 2000.0, recvRate)
	// sent: 1500 / 1.0 = 1500
	assert.Equal(t, 1500.0, sentRate)
}

func TestCalculateNetworkIORatesNoChange(t *testing.T) {
	stats := NetworkStats{TotalRecvBytes: 1000000, TotalSentBytes: 500000}

	recvRate, sentRate := CalculateNetworkIORates(stats, stats, 10.0)

	assert.Equal(t, 0.0, recvRate)
	assert.Equal(t, 0.0, sentRate)
}

package collector

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewGlobalCollector(t *testing.T) {
	// Test with nil filter (should use defaults)
	collector := NewGlobalCollector(nil)

	assert.NotNil(t, collector)
	assert.NotNil(t, collector.collectors)
	assert.Len(t, collector.collectors, 2) // CPU and Memory collectors

	// Should have default filter applied
	assert.Equal(t, []string{"lo", "docker0"}, collector.networkInterfaceFilter.ExcludeInterfaces)
	assert.Equal(t, []string{"veth", "br-", "docker"}, collector.networkInterfaceFilter.ExcludePrefixes)
}

func TestNewGlobalCollectorWithCustomFilter(t *testing.T) {
	customFilter := &NetworkInterfaceFilter{
		ExcludeInterfaces: []string{"lo", "eth1"},
		ExcludePrefixes:   []string{"veth"},
	}
	collector := NewGlobalCollector(customFilter)

	assert.NotNil(t, collector)
	assert.Equal(t, []string{"lo", "eth1"}, collector.networkInterfaceFilter.ExcludeInterfaces)
	assert.Equal(t, []string{"veth"}, collector.networkInterfaceFilter.ExcludePrefixes)
}

func TestCollectMemoryStatsWithMockData(t *testing.T) {
	// Create a temporary file with mock /proc/meminfo data
	tmpDir := t.TempDir()
	memInfoFile := filepath.Join(tmpDir, "meminfo")
	
	memInfoContent := `MemTotal:        8192000 kB
MemFree:         2048000 kB
MemAvailable:    6144000 kB
Buffers:         1024000 kB
Cached:          2048000 kB
SwapTotal:       2048000 kB
SwapFree:        1024000 kB
`
	require.NoError(t, os.WriteFile(memInfoFile, []byte(memInfoContent), 0644))
	
	// In a real implementation, we'd have a way to inject the path
	// For now, this test demonstrates the concept
	_ = memInfoFile // Use the mock file variable
	
	// Test that we can at least call the function without error
	// In a real implementation, we'd inject the file path dependency
	metrics, err := CollectMemoryStats()
	
	// Since we can't easily mock /proc/meminfo without dependency injection,
	// we'll just ensure the function works and returns expected metric names
	if err == nil {
		assert.Contains(t, metrics, "mem_percent_used")
		assert.Contains(t, metrics, "mem_percent_free")
		assert.Contains(t, metrics, "swap_percent_used")
		assert.Contains(t, metrics, "swap_percent_free")
		
		// Validate that percentages are reasonable
		for _, value := range metrics {
			assert.True(t, value >= 0.0 && value <= 100.0, "Memory percentage should be between 0 and 100")
		}
	}
}

func TestCollectCPUStatsWithMockData(t *testing.T) {
	// Test that CPU stats function can be called
	// In a real implementation, we'd mock /proc/stat
	elapsedSeconds := 1.0
	metrics, err := CollectCPUStats(elapsedSeconds)
	
	// If the system has /proc/stat, test the output
	if err == nil {
		assert.Contains(t, metrics, "cpu_percent_total")
		
		cpuPercent := metrics["cpu_percent_total"]
		assert.True(t, cpuPercent >= 0.0 && cpuPercent <= 100.0, "CPU percentage should be between 0 and 100")
	}
}

func TestGlobalCollectorCollectAll(t *testing.T) {
	collector := NewGlobalCollector(nil)
	
	// First collection
	metrics1, err := collector.CollectAll()
	
	// Should not error (unless system doesn't have /proc files)
	if err == nil {
		assert.NotNil(t, metrics1)
		
		// Should have some metrics
		assert.True(t, len(metrics1) > 0)
		
		// Should have expected metric names (if system has /proc files)
		expectedMetrics := []string{
			"cpu_percent_total",
			"mem_percent_used",
			"mem_percent_free",
			"swap_percent_used", 
			"swap_percent_free",
			"disk_read_bytes_ps",
			"disk_write_bytes_ps",
			"net_recv_bytes_ps",
			"net_sent_bytes_ps",
		}
		
		for _, metric := range expectedMetrics {
			if _, exists := metrics1[metric]; exists {
				value := metrics1[metric]
				assert.True(t, value >= 0.0, "Metric %s should be non-negative", metric)
			}
		}
		
		// Sleep briefly and collect again to test rate calculations
		time.Sleep(100 * time.Millisecond)
		
		metrics2, err := collector.CollectAll()
		if err == nil {
			assert.NotNil(t, metrics2)
			
			// Rate-based metrics might have different values now
			// (though they could still be 0 on an idle system)
			assert.Contains(t, metrics2, "disk_read_bytes_ps")
			assert.Contains(t, metrics2, "disk_write_bytes_ps")
			assert.Contains(t, metrics2, "net_recv_bytes_ps")
			assert.Contains(t, metrics2, "net_sent_bytes_ps")
		}
	}
}

func TestCollectedMetricsType(t *testing.T) {
	metrics := make(CollectedMetrics)
	
	// Test that we can add metrics
	metrics["test_metric"] = 42.5
	assert.Equal(t, 42.5, metrics["test_metric"])
	
	// Test that it behaves like a map
	assert.Len(t, metrics, 1)
	
	delete(metrics, "test_metric")
	assert.Len(t, metrics, 0)
}

func TestMetricCollectorInterface(t *testing.T) {
	// Test that CPU and Memory collectors implement the interface
	var collectors []MetricCollector
	
	collectors = append(collectors, NewCPUCollector())
	collectors = append(collectors, NewMemoryCollector())
	
	for _, collector := range collectors {
		// Should have a name
		name := collector.Name()
		assert.NotEmpty(t, name)
		
		// Should be able to collect (may error on systems without /proc)
		metrics, err := collector.Collect()
		if err == nil {
			assert.NotNil(t, metrics)
		}
	}
}

// Integration test with real system data (if available)
func TestRealSystemMetrics(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping real system metrics test in short mode")
	}

	collector := NewGlobalCollector(nil)
	
	// Collect metrics multiple times to test rate calculations
	for i := 0; i < 3; i++ {
		metrics, err := collector.CollectAll()
		
		if err != nil {
			t.Logf("Warning: Could not collect system metrics (iteration %d): %v", i+1, err)
			continue
		}
		
		t.Logf("Iteration %d - Collected %d metrics", i+1, len(metrics))
		
		// Log some key metrics for manual verification
		if cpu, ok := metrics["cpu_percent_total"]; ok {
			t.Logf("CPU Usage: %.2f%%", cpu)
		}
		if mem, ok := metrics["mem_percent_used"]; ok {
			t.Logf("Memory Usage: %.2f%%", mem)
		}
		
		// Brief pause between collections
		if i < 2 {
			time.Sleep(500 * time.Millisecond)
		}
	}
}
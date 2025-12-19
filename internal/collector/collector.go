package collector

import (
	"log"
	"sync"
	"time"
)

// CollectedMetrics holds all metrics gathered in one collection cycle.
// Using a map allows flexibility for different metrics.
type CollectedMetrics map[string]float64

// MetricCollector defines the interface for specific collectors.
type MetricCollector interface {
	Collect() (CollectedMetrics, error)
	Name() string // e.g., "cpu", "memory"
}

// GlobalCollector orchestrates all individual metric collectors.
type GlobalCollector struct {
	collectors []MetricCollector
	// For rate-based metrics like disk/network IO
	lastDiskStats          *DiskStats             // Pointer to allow nil for first run
	lastNetworkStats       *NetworkStats          // Pointer to allow nil for first run
	lastCollectTime        time.Time
	networkInterfaceFilter NetworkInterfaceFilter // Filter for network interfaces
	mu                     sync.Mutex             // Protects last stats and time
}

// NewGlobalCollector creates a new GlobalCollector with the given network interface filter.
// If filter is nil or empty, it uses the default filter that excludes Docker interfaces.
func NewGlobalCollector(networkFilter *NetworkInterfaceFilter) *GlobalCollector {
	gc := &GlobalCollector{}
	// Initialize specific collectors
	gc.collectors = append(gc.collectors, NewCPUCollector())
	gc.collectors = append(gc.collectors, NewMemoryCollector())
	// Disk and Network collectors are special as they calculate rates.
	// They are implicitly handled by CollectAll method or integrated.

	// Set network interface filter (use default if not provided)
	if networkFilter != nil {
		gc.networkInterfaceFilter = *networkFilter
	} else {
		gc.networkInterfaceFilter = DefaultNetworkInterfaceFilter()
	}

	// For simplicity in this structure, we'll have explicit methods for disk/net
	// and store their previous states in GlobalCollector.
	return gc
}

// CollectAll gathers all metrics from all registered collectors.
func (gc *GlobalCollector) CollectAll() (CollectedMetrics, error) {
	gc.mu.Lock()
	defer gc.mu.Unlock()

	allMetrics := make(CollectedMetrics)
	now := time.Now()
	var elapsedSeconds float64
	if !gc.lastCollectTime.IsZero() {
		elapsedSeconds = now.Sub(gc.lastCollectTime).Seconds()
	}


	// CPU
	cpuMetrics, err := CollectCPUStats(elapsedSeconds) // Pass elapsed for rate based on previous total/idle
	if err != nil {
		log.Printf("Error collecting CPU metrics: %v", err)
	} else {
		for k, v := range cpuMetrics {
			allMetrics[k] = v
		}
	}

	// Memory
	memMetrics, err := CollectMemoryStats()
	if err != nil {
		log.Printf("Error collecting Memory metrics: %v", err)
	} else {
		for k, v := range memMetrics {
			allMetrics[k] = v
		}
	}

	// Disk I/O
	currentDiskStats, err := GetDiskStats()
	if err != nil {
		log.Printf("Error collecting Disk I/O stats: %v", err)
	} else {
		if gc.lastDiskStats != nil && elapsedSeconds > 0.1 { // Avoid division by zero or tiny intervals
			readBps, writeBps := CalculateDiskIORates(*gc.lastDiskStats, *currentDiskStats, elapsedSeconds)
			allMetrics["disk_read_bytes_ps"] = readBps
			allMetrics["disk_write_bytes_ps"] = writeBps
		} else {
			allMetrics["disk_read_bytes_ps"] = 0
			allMetrics["disk_write_bytes_ps"] = 0
		}
		gc.lastDiskStats = currentDiskStats
	}

	// Network I/O
	currentNetStats, err := GetNetworkStats(gc.networkInterfaceFilter)
	if err != nil {
		log.Printf("Error collecting Network I/O stats: %v", err)
	} else {
		if gc.lastNetworkStats != nil && elapsedSeconds > 0.1 {
			recvBps, sentBps := CalculateNetworkIORates(*gc.lastNetworkStats, *currentNetStats, elapsedSeconds)
			allMetrics["net_recv_bytes_ps"] = recvBps
			allMetrics["net_sent_bytes_ps"] = sentBps
		} else {
			allMetrics["net_recv_bytes_ps"] = 0
			allMetrics["net_sent_bytes_ps"] = 0
		}
		gc.lastNetworkStats = currentNetStats
	}


	gc.lastCollectTime = now
	return allMetrics, nil // Overall error can be nil if some collectors succeed
}

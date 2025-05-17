package collector

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// DiskStats holds aggregated disk I/O counters from /proc/diskstats.
// We are interested in fields 3 (reads completed) and 7 (sectors written).
// Field 3: reads completed successfully
// Field 4: reads merged
// Field 5: sectors read (1 sector = 512 bytes)
// Field 6: time spent reading (ms)
// Field 7: writes completed
// Field 8: writes merged
// Field 9: sectors written
// Field 10: time spent writing (ms)
type DiskStats struct {
	TotalSectorsRead    uint64
	TotalSectorsWritten uint64
}

const sectorSize = 512 // bytes

// isRelevantDevice checks if the device name from /proc/diskstats is a physical disk or partition we care about.
// This is a simple heuristic; a more robust solution might involve udev or lsblk.
// For v1, we'll monitor common patterns like sdX, hdX, vdX, nvmeXnY, xvdX and their partitions.
// We should exclude loop, ram, rom devices.
func isRelevantDevice(deviceName string) bool {
	// Exclude loop devices, ram disks, cd/dvd roms
	if strings.HasPrefix(deviceName, "loop") ||
		strings.HasPrefix(deviceName, "ram") ||
		strings.HasPrefix(deviceName, "sr") || // SCSI ROM
		strings.HasPrefix(deviceName, "fd") { // Floppy disk
		return false
	}
	// Include common disk types
	// sd[a-z], hd[a-z], vd[a-z], xvd[a-z], nvme[0-9]n[0-9]
	// and their partitions (e.g. sda1)
	// A simple check: if it doesn't start with the exclusion list and contains some typical disk letters.
	// This could be refined. For now, any device not explicitly excluded is considered.
	// For a VPS, we usually only have one or two main virtual disks (e.g., vda, sda).
	return true // A more sophisticated filter can be added if needed
}


// GetDiskStats reads /proc/diskstats and aggregates read/write bytes across relevant devices.
func GetDiskStats() (*DiskStats, error) {
	file, err := os.Open("/proc/diskstats")
	if err != nil {
		return nil, fmt.Errorf("failed to open /proc/diskstats: %w", err)
	}
	defer file.Close()

	stats := &DiskStats{}
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 10 { // Need at least up to sectors written
			continue
		}

		deviceName := fields[2]
		if !isRelevantDevice(deviceName) {
			continue
		}
		// Field 5: sectors read
		sectorsRead, err := strconv.ParseUint(fields[5], 10, 64)
		if err != nil {
			// log.Printf("Warning: could not parse sectors_read for %s: %v", deviceName, err)
			continue
		}
		// Field 9: sectors written
		sectorsWritten, err := strconv.ParseUint(fields[9], 10, 64)
		if err != nil {
			// log.Printf("Warning: could not parse sectors_written for %s: %v", deviceName, err)
			continue
		}

		stats.TotalSectorsRead += sectorsRead
		stats.TotalSectorsWritten += sectorsWritten
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error scanning /proc/diskstats: %w", err)
	}
	return stats, nil
}

// CalculateDiskIORates computes read/write bytes per second.
func CalculateDiskIORates(prev, curr DiskStats, elapsedSeconds float64) (readBytesPs, writeBytesPs float64) {
	if elapsedSeconds <= 0 {
		return 0, 0
	}

	deltaSectorsRead := curr.TotalSectorsRead - prev.TotalSectorsRead
	deltaSectorsWritten := curr.TotalSectorsWritten - prev.TotalSectorsWritten

    // Handle counter wrap-around (unsigned integers) - less likely for disk stats over short periods
    if curr.TotalSectorsRead < prev.TotalSectorsRead { // wrapped
        deltaSectorsRead = curr.TotalSectorsRead // treat as if started from 0
    }
    if curr.TotalSectorsWritten < prev.TotalSectorsWritten { // wrapped
        deltaSectorsWritten = curr.TotalSectorsWritten
    }


	readBps := float64(deltaSectorsRead*sectorSize) / elapsedSeconds
	writeBps := float64(deltaSectorsWritten*sectorSize) / elapsedSeconds

	return readBps, writeBps
}

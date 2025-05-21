package history

import (
	"sync"
	"time"

	"github.com/mattmezza/monres/internal/config"
)

type DataPoint struct {
	Timestamp time.Time
	Value     float64
}

type MetricHistoryBuffer struct {
	sync.RWMutex
	buffers       map[string][]DataPoint // metricName -> []DataPoint
	maxDataPoints int                    // Max data points to keep per metric
}

func NewMetricHistoryBuffer(maxAge time.Duration, collectionInterval time.Duration) *MetricHistoryBuffer {
	if maxAge <= 0 || collectionInterval <= 0 { // Should not happen with config validation
		maxDataPoints := 60 // Default to 60 points if params are weird.
		return &MetricHistoryBuffer{
			buffers:       make(map[string][]DataPoint),
			maxDataPoints: maxDataPoints,
		}
	}
	maxDataPoints := int(maxAge.Seconds()/collectionInterval.Seconds()) + 1 // +1 for safety
	if maxDataPoints < 2 { // Need at least 2 points for some calcs or reasonable history
		maxDataPoints = 2
	}

	return &MetricHistoryBuffer{
		buffers:       make(map[string][]DataPoint),
		maxDataPoints: maxDataPoints,
	}
}

// AddDataPoint adds a new data point for a metric.
// It evicts the oldest point if the buffer for that metric exceeds maxDataPoints.
func (hb *MetricHistoryBuffer) AddDataPoint(metricName string, value float64, timestamp time.Time) {
	hb.Lock()
	defer hb.Unlock()

	points, exists := hb.buffers[metricName]
	if !exists {
		points = make([]DataPoint, 0, hb.maxDataPoints)
	}

	points = append(points, DataPoint{Timestamp: timestamp, Value: value})

	if len(points) > hb.maxDataPoints {
		points = points[len(points)-hb.maxDataPoints:] // Keep the newest N points
	}
	hb.buffers[metricName] = points
}

// GetDataPointsForDuration retrieves data points for a specific metric within the given duration.
// It returns points whose Timestamp is within [now - duration, now].
func (hb *MetricHistoryBuffer) GetDataPointsForDuration(metricName string, duration time.Duration, now time.Time) []DataPoint {
	hb.RLock()
	defer hb.RUnlock()

	points, exists := hb.buffers[metricName]
	if !exists || len(points) == 0 {
		return nil
	}

	if duration == 0 { // If duration is 0, return only the latest point
		if len(points) > 0 {
			return []DataPoint{points[len(points)-1]}
		}
		return nil
}

	startTime := now.Add(-(duration + 1 * time.Second)) // -1s to ensure we get points within the duration
	var result []DataPoint
	for i := len(points) - 1; i >= 0; i-- { // Iterate backwards for efficiency
		dp := points[i]
		if dp.Timestamp.Before(startTime) {
			break // Older points are not needed
		}
		result = append(result, dp) // Will be in reverse chronological order
	}
    // Reverse result to be chronological
    for i, j := 0, len(result)-1; i < j; i, j = i+1, j-1 {
        result[i], result[j] = result[j], result[i]
    }
	return result
}

// GetLatestDataPoint returns the most recent data point for a metric, if any.
func (hb *MetricHistoryBuffer) GetLatestDataPoint(metricName string) (DataPoint, bool) {
	hb.RLock()
	defer hb.RUnlock()

	points, exists := hb.buffers[metricName]
	if !exists || len(points) == 0 {
		return DataPoint{}, false
	}
	return points[len(points)-1], true
}

// GetMaxConfiguredDuration determines the maximum duration from all alert rules
// This is used by the main app to initialize the history buffer appropriately.
func GetMaxConfiguredDuration(rules []config.AlertRuleConfig, collectionInterval time.Duration) time.Duration {
	var maxDuration time.Duration
	minRequiredDurationForBuffer := 2 * collectionInterval // Ensure buffer can hold at least 2 points

	for _, rule := range rules {
		if rule.Duration > maxDuration {
			maxDuration = rule.Duration
		}
	}
    if maxDuration < minRequiredDurationForBuffer {
        return minRequiredDurationForBuffer
    }
	return maxDuration
}

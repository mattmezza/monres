package history

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewMetricHistoryBuffer(t *testing.T) {
	testCases := []struct {
		name               string
		maxAge             time.Duration
		collectionInterval time.Duration
		expectedMaxPoints  int
	}{
		{
			name:               "normal_case",
			maxAge:             10 * time.Minute,
			collectionInterval: 30 * time.Second,
			expectedMaxPoints:  21, // (600/30) + 1 = 21
		},
		{
			name:               "small_buffer",
			maxAge:             1 * time.Minute,
			collectionInterval: 30 * time.Second,
			expectedMaxPoints:  3, // (60/30) + 1 = 3
		},
		{
			name:               "minimum_buffer_size",
			maxAge:             10 * time.Second,
			collectionInterval: 30 * time.Second,
			expectedMaxPoints:  2, // minimum is 2
		},
		{
			name:               "zero_duration_defaults",
			maxAge:             0,
			collectionInterval: 0,
			expectedMaxPoints:  60, // default
		},
		{
			name:               "negative_duration_defaults",
			maxAge:             -1 * time.Minute,
			collectionInterval: -10 * time.Second,
			expectedMaxPoints:  60, // default
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			buffer := NewMetricHistoryBuffer(tc.maxAge, tc.collectionInterval)
			
			assert.NotNil(t, buffer)
			assert.Equal(t, tc.expectedMaxPoints, buffer.maxDataPoints)
			assert.NotNil(t, buffer.buffers)
			assert.Equal(t, 0, len(buffer.buffers))
		})
	}
}

func TestAddDataPoint(t *testing.T) {
	buffer := NewMetricHistoryBuffer(5*time.Minute, 30*time.Second)
	now := time.Now()
	
	// Test adding first data point
	buffer.AddDataPoint("cpu_usage", 50.0, now)
	
	points, exists := buffer.buffers["cpu_usage"]
	require.True(t, exists)
	assert.Len(t, points, 1)
	assert.Equal(t, 50.0, points[0].Value)
	assert.Equal(t, now, points[0].Timestamp)
	
	// Test adding multiple data points
	for i := 1; i <= 5; i++ {
		buffer.AddDataPoint("cpu_usage", float64(50+i), now.Add(time.Duration(i)*time.Second))
	}
	
	points = buffer.buffers["cpu_usage"]
	assert.Len(t, points, 6) // 1 initial + 5 added
	assert.Equal(t, 55.0, points[5].Value) // last added value
}

func TestAddDataPointEviction(t *testing.T) {
	// Create buffer with small capacity
	buffer := NewMetricHistoryBuffer(1*time.Minute, 30*time.Second) // max 3 points
	now := time.Now()
	
	// Add more points than capacity
	for i := 0; i < 5; i++ {
		buffer.AddDataPoint("test_metric", float64(i), now.Add(time.Duration(i)*time.Second))
	}
	
	points := buffer.buffers["test_metric"]
	
	// Should have exactly maxDataPoints
	assert.Len(t, points, buffer.maxDataPoints)
	
	// Should contain the most recent points (oldest evicted)
	expectedStartValue := float64(5 - buffer.maxDataPoints) // 5-3=2
	assert.Equal(t, expectedStartValue, points[0].Value)
	assert.Equal(t, 4.0, points[len(points)-1].Value) // most recent
}

func TestGetLatestDataPoint(t *testing.T) {
	buffer := NewMetricHistoryBuffer(5*time.Minute, 30*time.Second)
	now := time.Now()
	
	// Test non-existent metric
	_, exists := buffer.GetLatestDataPoint("nonexistent")
	assert.False(t, exists)
	
	// Add some data points
	buffer.AddDataPoint("cpu_usage", 30.0, now.Add(-2*time.Second))
	buffer.AddDataPoint("cpu_usage", 40.0, now.Add(-1*time.Second))
	buffer.AddDataPoint("cpu_usage", 50.0, now)
	
	// Get latest point
	latest, exists := buffer.GetLatestDataPoint("cpu_usage")
	require.True(t, exists)
	assert.Equal(t, 50.0, latest.Value)
	assert.Equal(t, now, latest.Timestamp)
}

func TestGetDataPointsForDuration(t *testing.T) {
	buffer := NewMetricHistoryBuffer(10*time.Minute, 30*time.Second)
	now := time.Now()
	
	// Add test data points spanning 5 minutes
	testData := []struct {
		value     float64
		timestamp time.Time
	}{
		{10.0, now.Add(-5 * time.Minute)},
		{20.0, now.Add(-4 * time.Minute)},
		{30.0, now.Add(-3 * time.Minute)},
		{40.0, now.Add(-2 * time.Minute)},
		{50.0, now.Add(-1 * time.Minute)},
		{60.0, now},
	}
	
	for _, data := range testData {
		buffer.AddDataPoint("test_metric", data.value, data.timestamp)
	}
	
	testCases := []struct {
		name            string
		duration        time.Duration
		queryTime       time.Time
		expectedCount   int
		expectedValues  []float64
	}{
		{
			name:            "last_2_minutes",
			duration:        2 * time.Minute,
			queryTime:       now,
			expectedCount:   3, // points at -2min, -1min, 0min
			expectedValues:  []float64{40.0, 50.0, 60.0},
		},
		{
			name:            "last_1_minute",
			duration:        1 * time.Minute,
			queryTime:       now,
			expectedCount:   2, // points at -1min, 0min
			expectedValues:  []float64{50.0, 60.0},
		},
		{
			name:            "zero_duration_returns_latest",
			duration:        0,
			queryTime:       now,
			expectedCount:   1, // returns latest point for zero duration
			expectedValues:  []float64{60.0},
		},
		{
			name:            "future_query_time",
			duration:        1 * time.Minute,
			queryTime:       now.Add(1 * time.Minute),
			expectedCount:   1, // only the point at 'now' is within 1min of future time
			expectedValues:  []float64{60.0},
		},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			points := buffer.GetDataPointsForDuration("test_metric", tc.duration, tc.queryTime)
			
			assert.Len(t, points, tc.expectedCount)
			
			for i, expectedValue := range tc.expectedValues {
				if i < len(points) {
					assert.Equal(t, expectedValue, points[i].Value)
				}
			}
		})
	}
}

func TestGetDataPointsForDurationNonexistentMetric(t *testing.T) {
	buffer := NewMetricHistoryBuffer(5*time.Minute, 30*time.Second)
	now := time.Now()
	
	points := buffer.GetDataPointsForDuration("nonexistent", 1*time.Minute, now)
	assert.Len(t, points, 0)
}

func TestConcurrentAccess(t *testing.T) {
	buffer := NewMetricHistoryBuffer(5*time.Minute, 1*time.Second)
	now := time.Now()
	
	// Start goroutines that add data points concurrently
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(id int) {
			for j := 0; j < 100; j++ {
				buffer.AddDataPoint("concurrent_metric", float64(id*100+j), now.Add(time.Duration(j)*time.Millisecond))
			}
			done <- true
		}(i)
	}
	
	// Start goroutines that read data points concurrently
	for i := 0; i < 5; i++ {
		go func() {
			for j := 0; j < 50; j++ {
				buffer.GetLatestDataPoint("concurrent_metric")
				buffer.GetDataPointsForDuration("concurrent_metric", 1*time.Minute, now)
			}
			done <- true
		}()
	}
	
	// Wait for all goroutines to complete
	for i := 0; i < 15; i++ { // 10 writers + 5 readers
		<-done
	}
	
	// Verify data integrity
	points := buffer.GetDataPointsForDuration("concurrent_metric", 10*time.Minute, now.Add(1*time.Minute))
	assert.True(t, len(points) > 0)
	assert.True(t, len(points) <= buffer.maxDataPoints)
}

func TestMultipleMetrics(t *testing.T) {
	buffer := NewMetricHistoryBuffer(5*time.Minute, 30*time.Second)
	now := time.Now()
	
	// Add data for multiple metrics
	metrics := map[string][]float64{
		"cpu_usage":    {10, 20, 30},
		"memory_usage": {40, 50, 60},
		"disk_io":      {70, 80, 90},
	}
	
	for metric, values := range metrics {
		for i, value := range values {
			buffer.AddDataPoint(metric, value, now.Add(time.Duration(i)*time.Second))
		}
	}
	
	// Verify each metric has correct data
	for metric, expectedValues := range metrics {
		points := buffer.GetDataPointsForDuration(metric, 10*time.Minute, now.Add(10*time.Second))
		assert.Len(t, points, len(expectedValues))
		
		for i, point := range points {
			assert.Equal(t, expectedValues[i], point.Value)
		}
	}
	
	// Verify metrics are independent
	assert.Len(t, buffer.buffers, 3)
}
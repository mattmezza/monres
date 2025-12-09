package notifier

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattmezza/monres/internal/config"
)

func TestRenderTemplate(t *testing.T) {
	testData := NotificationData{
		AlertName:      "High CPU",
		MetricName:     "cpu_percent_total",
		MetricValue:    95.5,
		ThresholdValue: 90.0,
		Condition:      ">",
		State:          "FIRED",
		Hostname:       "test-server",
		Time:           time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC),
		DurationString: "5m",
		Aggregation:    "average",
	}

	testCases := []struct {
		name         string
		template     string
		expected     string
		expectError  bool
	}{
		{
			name:         "simple_template",
			template:     "Alert: {{ .AlertName }} on {{ .Hostname }}",
			expected:     "Alert: High CPU on test-server",
			expectError:  false,
		},
		{
			name:         "complex_template",
			template:     "{{ .State }}: {{ .MetricName }} = {{ printf \"%.1f\" .MetricValue }} {{ .Condition }} {{ .ThresholdValue }}",
			expected:     "FIRED: cpu_percent_total = 95.5 > 90",
			expectError:  false,
		},
		{
			name:         "time_formatting",
			template:     "Time: {{ .Time.Format \"2006-01-02 15:04:05\" }}",
			expected:     "Time: 2023-01-01 12:00:00",
			expectError:  false,
		},
		{
			name:         "invalid_template",
			template:     "{{ .NonExistentField }}",
			expected:     "",
			expectError:  true,
		},
		{
			name:         "syntax_error",
			template:     "{{ .AlertName",
			expected:     "",
			expectError:  true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := renderTemplate("test", tc.template, testData)

			if tc.expectError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.expected, result)
			}
		})
	}
}

func TestStdoutNotifier(t *testing.T) {
	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	notifier, err := NewStdoutNotifier("test-stdout")
	require.NoError(t, err)
	assert.Equal(t, "test-stdout", notifier.Name())

	testData := NotificationData{
		AlertName:      "Test Alert",
		MetricName:     "test_metric",
		MetricValue:    50.0,
		ThresholdValue: 40.0,
		Condition:      ">",
		State:          "FIRED",
		Hostname:       "test-host",
		Time:           time.Now(),
		DurationString: "1m",
		Aggregation:    "average",
	}

	templates := NotificationTemplates{
		FiredTemplate: "FIRED: {{ .AlertName }} on {{ .Hostname }}",
	}

	err = notifier.Send(testData, templates)
	require.NoError(t, err)

	// Close writer and read captured output
	w.Close()
	os.Stdout = oldStdout

	output, _ := io.ReadAll(r)
	assert.Contains(t, string(output), "FIRED: Test Alert on test-host")
}

func TestEmailNotifier(t *testing.T) {
	testCases := []struct {
		name        string
		config      config.EmailChannelConfig
		expectError bool
	}{
		{
			name: "valid_config",
			config: config.EmailChannelConfig{
				SMTPHost:     "smtp.example.com",
				SMTPPort:     587,
				SMTPUsername: "user@example.com",
				SMTPPassword: "password",
				SMTPFrom:     "Test <test@example.com>",
				SMTPTo:       []string{"admin@example.com"},
				SMTPUseTLS:   true,
			},
			expectError: false,
		},
		{
			name: "missing_host",
			config: config.EmailChannelConfig{
				SMTPPort: 587,
				SMTPFrom: "test@example.com",
				SMTPTo:   []string{"admin@example.com"},
			},
			expectError: true,
		},
		{
			name: "missing_port",
			config: config.EmailChannelConfig{
				SMTPHost: "smtp.example.com",
				SMTPFrom: "test@example.com",
				SMTPTo:   []string{"admin@example.com"},
			},
			expectError: true,
		},
		{
			name: "missing_from",
			config: config.EmailChannelConfig{
				SMTPHost: "smtp.example.com",
				SMTPPort: 587,
				SMTPTo:   []string{"admin@example.com"},
			},
			expectError: true,
		},
		{
			name: "missing_to",
			config: config.EmailChannelConfig{
				SMTPHost: "smtp.example.com",
				SMTPPort: 587,
				SMTPFrom: "test@example.com",
				SMTPTo:   []string{},
			},
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			notifier, err := NewEmailNotifier("test-email", tc.config)

			if tc.expectError {
				assert.Error(t, err)
				assert.Nil(t, notifier)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, notifier)
				assert.Equal(t, "test-email", notifier.Name())
			}
		})
	}
}

func TestTelegramNotifier(t *testing.T) {
	testCases := []struct {
		name        string
		config      config.TelegramChannelConfig
		expectError bool
	}{
		{
			name: "valid_config",
			config: config.TelegramChannelConfig{
				BotToken: "123456:ABC-DEF1234ghIkl-zyx57W2v1u123ew11",
				ChatID:   "-123456789",
			},
			expectError: false,
		},
		{
			name: "missing_token",
			config: config.TelegramChannelConfig{
				ChatID: "-123456789",
			},
			expectError: true,
		},
		{
			name: "missing_chat_id",
			config: config.TelegramChannelConfig{
				BotToken: "123456:ABC-DEF1234ghIkl-zyx57W2v1u123ew11",
			},
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			notifier, err := NewTelegramNotifier("test-telegram", tc.config)

			if tc.expectError {
				assert.Error(t, err)
				assert.Nil(t, notifier)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, notifier)
				assert.Equal(t, "test-telegram", notifier.Name())
			}
		})
	}
}

func TestTelegramNotifierSend(t *testing.T) {
	// Create a mock HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Contains(t, r.URL.Path, "/sendMessage")
		
		// Check request body (JSON format)
		body, _ := io.ReadAll(r.Body)
		bodyStr := string(body)
		assert.Contains(t, bodyStr, "\"-123456789\"")
		assert.Contains(t, bodyStr, "Test Alert")
		
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"ok": true, "result": {"message_id": 1}}`))
	}))
	defer server.Close()

	config := config.TelegramChannelConfig{
		BotToken: "123456:ABC-DEF1234ghIkl-zyx57W2v1u123ew11",
		ChatID:   "-123456789",
	}

	notifier, err := NewTelegramNotifier("test-telegram", config)
	require.NoError(t, err)

	// Replace the Telegram API URL with our test server
	// This is a bit hacky but works for testing
	originalClient := notifier.client
	notifier.client = &http.Client{
		Transport: &MockTransport{
			server: server,
		},
	}
	defer func() { notifier.client = originalClient }()

	testData := NotificationData{
		AlertName:      "Test Alert",
		MetricName:     "test_metric",
		MetricValue:    50.0,
		ThresholdValue: 40.0,
		Condition:      ">",
		State:          "FIRED",
		Hostname:       "test-host",
		Time:           time.Now(),
		DurationString: "1m",
		Aggregation:    "average",
	}

	templates := NotificationTemplates{
		FiredTemplate: "FIRED: {{ .AlertName }} on {{ .Hostname }}",
	}

	err = notifier.Send(testData, templates)
	require.NoError(t, err)
}

func TestTelegramNotifierSendError(t *testing.T) {
	// Create a mock HTTP server that returns an error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"ok": false, "error_code": 400, "description": "Bad Request"}`))
	}))
	defer server.Close()

	config := config.TelegramChannelConfig{
		BotToken: "123456:ABC-DEF1234ghIkl-zyx57W2v1u123ew11",
		ChatID:   "-123456789",
	}

	notifier, err := NewTelegramNotifier("test-telegram", config)
	require.NoError(t, err)

	notifier.client = &http.Client{
		Transport: &MockTransport{
			server: server,
		},
	}

	testData := NotificationData{
		AlertName: "Test Alert",
		State:     "FIRED",
		Hostname:  "test-host",
		Time:      time.Now(),
	}

	templates := NotificationTemplates{
		FiredTemplate: "FIRED: {{ .AlertName }}",
	}

	err = notifier.Send(testData, templates)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "telegram API request failed")
}

func TestInitializeNotifiers(t *testing.T) {
	channels := []config.NotificationChannelConfig{
		{
			Name: "email-test",
			Type: "email",
			Config: map[string]interface{}{
				"smtp_host": "smtp.example.com",
				"smtp_port": 587,
				"smtp_from": "test@example.com",
				"smtp_to":   []interface{}{"admin@example.com"},
			},
		},
		{
			Name: "telegram-test",
			Type: "telegram",
			Config: map[string]interface{}{
				"bot_token": "123456:ABC-DEF1234ghIkl-zyx57W2v1u123ew11",
				"chat_id":   "-123456789",
			},
		},
		{
			Name: "stdout-test",
			Type: "stdout",
		},
		{
			Name: "invalid-type",
			Type: "unsupported",
		},
	}

	notifiers, err := InitializeNotifiers(channels)
	require.NoError(t, err)

	// Should have 3 successful notifiers (email, telegram, stdout) and skip the invalid one
	assert.Len(t, notifiers, 3)
	assert.Contains(t, notifiers, "email-test")
	assert.Contains(t, notifiers, "telegram-test")
	assert.Contains(t, notifiers, "stdout-test")
	assert.NotContains(t, notifiers, "invalid-type")
}

func TestInitializeNotifiersDuplicateNames(t *testing.T) {
	channels := []config.NotificationChannelConfig{
		{
			Name: "duplicate",
			Type: "stdout",
		},
		{
			Name: "duplicate",
			Type: "stdout",
		},
	}

	notifiers, err := InitializeNotifiers(channels)
	assert.Error(t, err)
	assert.Nil(t, notifiers)
	assert.Contains(t, err.Error(), "duplicate notification channel name")
}

// MockTransport is a helper for mocking HTTP requests
type MockTransport struct {
	server *httptest.Server
}

func (t *MockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Replace the URL with our test server URL
	req.URL.Scheme = "http"
	req.URL.Host = strings.TrimPrefix(t.server.URL, "http://")

	return http.DefaultTransport.RoundTrip(req)
}

func TestFormatValue(t *testing.T) {
	testCases := []struct {
		name       string
		metricName string
		value      float64
		expected   string
	}{
		// Byte rate metrics
		{
			name:       "bytes_per_second_small",
			metricName: "disk_read_bytes_ps",
			value:      500,
			expected:   "500.0 B/s",
		},
		{
			name:       "bytes_per_second_kilobytes",
			metricName: "disk_write_bytes_ps",
			value:      1536,
			expected:   "1.5 KB/s",
		},
		{
			name:       "bytes_per_second_megabytes",
			metricName: "net_recv_bytes_ps",
			value:      5242880,
			expected:   "5.0 MB/s",
		},
		{
			name:       "bytes_per_second_large",
			metricName: "net_sent_bytes_ps",
			value:      551052824.82,
			expected:   "525.5 MB/s",
		},
		{
			name:       "bytes_per_second_gigabytes",
			metricName: "disk_read_bytes_ps",
			value:      1073741824,
			expected:   "1.0 GB/s",
		},
		// Percentage metrics
		{
			name:       "cpu_percent",
			metricName: "cpu_percent_total",
			value:      85.5,
			expected:   "85.5%",
		},
		{
			name:       "mem_percent_used",
			metricName: "mem_percent_used",
			value:      72.3,
			expected:   "72.3%",
		},
		{
			name:       "mem_percent_free",
			metricName: "mem_percent_free",
			value:      27.7,
			expected:   "27.7%",
		},
		{
			name:       "swap_percent",
			metricName: "swap_percent_used",
			value:      10.0,
			expected:   "10.0%",
		},
		// Unknown metrics (default format)
		{
			name:       "unknown_metric",
			metricName: "custom_metric",
			value:      123.456,
			expected:   "123.46",
		},
		{
			name:       "unknown_metric_integer",
			metricName: "some_count",
			value:      42.0,
			expected:   "42.00",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := FormatValue(tc.metricName, tc.value)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestFormatBytesPerSecond(t *testing.T) {
	testCases := []struct {
		name     string
		bytes    float64
		expected string
	}{
		{"zero", 0, "0.0 B/s"},
		{"bytes", 512, "512.0 B/s"},
		{"kilobytes_boundary", 1024, "1.0 KB/s"},
		{"kilobytes", 2048, "2.0 KB/s"},
		{"megabytes_boundary", 1048576, "1.0 MB/s"},
		{"megabytes", 10485760, "10.0 MB/s"},
		{"gigabytes_boundary", 1073741824, "1.0 GB/s"},
		{"gigabytes", 5368709120, "5.0 GB/s"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := formatBytesPerSecond(tc.bytes)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestFormatPercent(t *testing.T) {
	testCases := []struct {
		name     string
		value    float64
		expected string
	}{
		{"zero", 0, "0.0%"},
		{"decimal", 50.5, "50.5%"},
		{"whole", 100, "100.0%"},
		{"small", 0.1, "0.1%"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := formatPercent(tc.value)
			assert.Equal(t, tc.expected, result)
		})
	}
}
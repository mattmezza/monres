package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadConfig(t *testing.T) {
	testCases := []struct {
		name     string
		yaml     string
		expected *Config
		wantErr  bool
	}{
		{
			name: "valid_basic_config",
			yaml: `
interval_seconds: 10
hostname: "test-host"
alerts:
  - name: "CPU Alert"
    metric: "cpu_percent_total"
    condition: ">"
    threshold: 90
    duration: "5m"
    aggregation: "average"
    channels: ["email"]
notification_channels:
  - name: "email"
    type: "email"
    config:
      smtp_host: "smtp.example.com"
      smtp_port: 587
      smtp_from: "test@example.com"
      smtp_to: ["admin@example.com"]
templates:
  alert_fired: "Alert: {{ .AlertName }}"
  alert_resolved: "Resolved: {{ .AlertName }}"
`,
			expected: &Config{
				IntervalSeconds:    10,
				HostnameOverride:   "test-host",
				EffectiveHostname:  "test-host",
				CollectionInterval: 10 * time.Second,
				Alerts: []AlertRuleConfig{
					{
						Name:        "CPU Alert",
						Metric:      "cpu_percent_total",
						Condition:   ">",
						Threshold:   90,
						DurationStr: "5m",
						Duration:    5 * time.Minute,
						Aggregation: "average",
						Channels:    []string{"email"},
					},
				},
				NotificationChannels: []NotificationChannelConfig{
					{
						Name: "email",
						Type: "email",
						Config: map[string]interface{}{
							"smtp_host": "smtp.example.com",
							"smtp_port": 587,
							"smtp_from": "test@example.com",
							"smtp_to":   []interface{}{"admin@example.com"},
						},
					},
				},
				Templates: TemplateConfig{
					AlertFired:    "Alert: {{ .AlertName }}",
					AlertResolved: "Resolved: {{ .AlertName }}",
				},
			},
			wantErr: false,
		},
		{
			name: "minimal_config_with_defaults",
			yaml: `
alerts: []
notification_channels: []
`,
			expected: &Config{
				IntervalSeconds:      30, // default
				CollectionInterval:   30 * time.Second,
				Alerts:               []AlertRuleConfig{},
				NotificationChannels: []NotificationChannelConfig{},
				Templates: TemplateConfig{
					AlertFired:    `ALERT FIRED: {{.AlertName}} on {{.Hostname}}. Metric: {{.MetricName}} {{.Condition}} {{.ThresholdValue}} (Current: {{printf "%.2f" .MetricValue}}). Time: {{.Time.Format "2006-01-02 15:04:05"}}`,
					AlertResolved: `ALERT RESOLVED: {{.AlertName}} on {{.Hostname}}. Time: {{.Time.Format "2006-01-02 15:04:05"}}`,
				},
			},
			wantErr: false,
		},
		{
			name: "invalid_yaml",
			yaml: `
interval_seconds: 10
alerts:
  - name: "Test"
    invalid_field: [
`,
			wantErr: true,
		},
		{
			name: "missing_alert_name",
			yaml: `
alerts:
  - metric: "cpu_percent_total"
    condition: ">"
    threshold: 90
    channels: ["test"]
`,
			wantErr: true,
		},
		{
			name: "missing_alert_metric",
			yaml: `
alerts:
  - name: "Test Alert"
    condition: ">"
    threshold: 90
    channels: ["test"]
`,
			wantErr: true,
		},
		{
			name: "invalid_aggregation",
			yaml: `
alerts:
  - name: "Test Alert"
    metric: "cpu_percent_total"
    condition: ">"
    threshold: 90
    aggregation: "invalid"
    channels: ["test"]
`,
			wantErr: true,
		},
		{
			name: "invalid_duration",
			yaml: `
alerts:
  - name: "Test Alert"
    metric: "cpu_percent_total"
    condition: ">"
    threshold: 90
    duration: "invalid"
    channels: ["test"]
`,
			wantErr: true,
		},
		{
			name: "missing_channels",
			yaml: `
alerts:
  - name: "Test Alert"
    metric: "cpu_percent_total"
    condition: ">"
    threshold: 90
`,
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create temporary config file
			tmpDir := t.TempDir()
			configFile := filepath.Join(tmpDir, "config.yaml")
			require.NoError(t, os.WriteFile(configFile, []byte(tc.yaml), 0644))

			// Load config
			cfg, err := LoadConfig(configFile)

			if tc.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tc.expected.IntervalSeconds, cfg.IntervalSeconds)
			assert.Equal(t, tc.expected.CollectionInterval, cfg.CollectionInterval)
			
			if tc.expected.HostnameOverride != "" {
				assert.Equal(t, tc.expected.EffectiveHostname, cfg.EffectiveHostname)
			} else {
				// Should use system hostname
				assert.NotEmpty(t, cfg.EffectiveHostname)
			}

			assert.Equal(t, len(tc.expected.Alerts), len(cfg.Alerts))
			for i, expectedAlert := range tc.expected.Alerts {
				assert.Equal(t, expectedAlert.Name, cfg.Alerts[i].Name)
				assert.Equal(t, expectedAlert.Metric, cfg.Alerts[i].Metric)
				assert.Equal(t, expectedAlert.Condition, cfg.Alerts[i].Condition)
				assert.Equal(t, expectedAlert.Threshold, cfg.Alerts[i].Threshold)
				assert.Equal(t, expectedAlert.Duration, cfg.Alerts[i].Duration)
				assert.Equal(t, expectedAlert.Aggregation, cfg.Alerts[i].Aggregation)
				assert.Equal(t, expectedAlert.Channels, cfg.Alerts[i].Channels)
			}

			assert.Equal(t, len(tc.expected.NotificationChannels), len(cfg.NotificationChannels))
			assert.Equal(t, tc.expected.Templates, cfg.Templates)
		})
	}
}

func TestLoadConfigFileNotFound(t *testing.T) {
	_, err := LoadConfig("nonexistent.yaml")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read config file")
}

func TestGetEmailChannelConfig(t *testing.T) {
	testCases := []struct {
		name     string
		input    NotificationChannelConfig
		expected *EmailChannelConfig
		wantErr  bool
	}{
		{
			name: "valid_email_config",
			input: NotificationChannelConfig{
				Name: "test-email",
				Type: "email",
				Config: map[string]interface{}{
					"smtp_host":     "smtp.example.com",
					"smtp_port":     587,
					"smtp_username": "user@example.com",
					"smtp_from":     "Test <test@example.com>",
					"smtp_to":       []interface{}{"admin@example.com", "ops@example.com"},
					"smtp_use_tls":  true,
				},
			},
			expected: &EmailChannelConfig{
				SMTPHost:     "smtp.example.com",
				SMTPPort:     587,
				SMTPUsername: "user@example.com",
				SMTPFrom:     "Test <test@example.com>",
				SMTPTo:       []string{"admin@example.com", "ops@example.com"},
				SMTPUseTLS:   true,
			},
			wantErr: false,
		},
		{
			name: "missing_required_field",
			input: NotificationChannelConfig{
				Name: "test-email",
				Type: "email",
				Config: map[string]interface{}{
					"smtp_port": 587,
				},
			},
			wantErr: true,
		},
		{
			name: "invalid_port_type",
			input: NotificationChannelConfig{
				Name: "test-email",
				Type: "email",
				Config: map[string]interface{}{
					"smtp_host": "smtp.example.com",
					"smtp_port": "not-a-number",
					"smtp_from": "test@example.com",
					"smtp_to":   []interface{}{"admin@example.com"},
				},
			},
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := GetEmailChannelConfig(tc.input)

			if tc.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestGetTelegramChannelConfig(t *testing.T) {
	testCases := []struct {
		name     string
		input    NotificationChannelConfig
		expected *TelegramChannelConfig
		wantErr  bool
		envToken string
	}{
		{
			name: "valid_telegram_config_with_token",
			input: NotificationChannelConfig{
				Name: "test-telegram",
				Type: "telegram",
				Config: map[string]interface{}{
					"chat_id":   "-123456789",
					"bot_token": "test-token-123",
				},
			},
			expected: &TelegramChannelConfig{
				ChatID:   "-123456789",
				BotToken: "test-token-123",
			},
			wantErr: false,
		},
		{
			name: "missing_bot_token",
			input: NotificationChannelConfig{
				Name: "test-telegram",
				Type: "telegram",
				Config: map[string]interface{}{
					"chat_id": "-123456789",
				},
			},
			wantErr: true,
		},
		{
			name: "missing_chat_id",
			input: NotificationChannelConfig{
				Name:   "test-telegram",
				Type:   "telegram",
				Config: map[string]interface{}{},
			},
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := GetTelegramChannelConfig(tc.input)

			if tc.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tc.expected.ChatID, result.ChatID)
			assert.Equal(t, tc.expected.BotToken, result.BotToken)
		})
	}
}

func TestEnvironmentVariableInjection(t *testing.T) {
	// Set test environment variables
	os.Setenv("MONRES_SMTP_PASSWORD_TEST_EMAIL", "test-password")
	os.Setenv("MONRES_TELEGRAM_TOKEN_TEST_TELEGRAM", "test-token")
	defer func() {
		os.Unsetenv("MONRES_SMTP_PASSWORD_TEST_EMAIL")
		os.Unsetenv("MONRES_TELEGRAM_TOKEN_TEST_TELEGRAM")
	}()

	// Create a test config file with channels that will use environment variables
	yaml := `
interval_seconds: 5
alerts: []
notification_channels:
  - name: "test-email"
    type: "email"
    config:
      smtp_host: "smtp.example.com"
      smtp_port: 587
      smtp_from: "test@example.com"
      smtp_to: ["admin@example.com"]
  - name: "test-telegram"
    type: "telegram"
    config:
      chat_id: "-123456789"
`

	// Create temporary config file
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")
	require.NoError(t, os.WriteFile(configFile, []byte(yaml), 0644))

	// Load config - this should inject environment variables
	cfg, err := LoadConfig(configFile)
	require.NoError(t, err)

	// Test that environment variables were injected
	emailResult, err := GetEmailChannelConfig(cfg.NotificationChannels[0])
	require.NoError(t, err)
	assert.Equal(t, "test-password", emailResult.SMTPPassword)

	telegramResult, err := GetTelegramChannelConfig(cfg.NotificationChannels[1])
	require.NoError(t, err)
	assert.Equal(t, "test-token", telegramResult.BotToken)
}
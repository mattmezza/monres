package config

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/mattmezza/monres/internal/util" // Corrected import path
	"gopkg.in/yaml.v3"
)

type Config struct {
	IntervalSeconds    int                    `yaml:"interval_seconds"`
	HostnameOverride   string                 `yaml:"hostname"` // Field for Hostname
	Alerts             []AlertRuleConfig      `yaml:"alerts"`
	NotificationChannels []NotificationChannelConfig `yaml:"notification_channels"`
	Templates          TemplateConfig         `yaml:"templates"`
	CollectionInterval time.Duration          `yaml:"-"` // Derived
	EffectiveHostname  string                 `yaml:"-"` // Derived
}

type AlertRuleConfig struct {
	Name        string   `yaml:"name"`
	Metric      string   `yaml:"metric"`
	Condition   string   `yaml:"condition"`
	Threshold   float64  `yaml:"threshold"`
	DurationStr string   `yaml:"duration"` // e.g., "5m", "300s"
	Aggregation string   `yaml:"aggregation"` // "average", "max"
	Channels    []string `yaml:"channels"`
	Duration    time.Duration `yaml:"-"` // Parsed
}

type NotificationChannelConfig struct {
	Name   string                 `yaml:"name"`
	Type   string                 `yaml:"type"` // "email", "telegram"
	Config map[string]interface{} `yaml:"config"`
}

type EmailChannelConfig struct {
	SMTPHost     string   `yaml:"smtp_host"`
	SMTPPort     int      `yaml:"smtp_port"`
	SMTPUsername string   `yaml:"smtp_username"`
	SMTPPassword string   `yaml:"smtp_password"` // Will be populated from ENV
	SMTPFrom     string   `yaml:"smtp_from"`
	SMTPTo       []string `yaml:"smtp_to"`
	SMTPUseTLS   bool     `yaml:"smtp_use_tls"`
}

type TelegramChannelConfig struct {
	BotToken string `yaml:"bot_token"` // Will be populated from ENV
	ChatID   string `yaml:"chat_id"`
}

type TemplateConfig struct {
	AlertFired    string `yaml:"alert_fired"`
	AlertResolved string `yaml:"alert_resolved"`
}

func LoadConfig(filePath string) (*Config, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file %s: %w", filePath, err)
	}

	var cfg Config
	err = yaml.Unmarshal(data, &cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal config YAML from %s: %w", filePath, err)
	}

	// Validate and derive values
	if cfg.IntervalSeconds <= 0 {
		cfg.IntervalSeconds = 30 // Default
	}
	cfg.CollectionInterval = time.Duration(cfg.IntervalSeconds) * time.Second

	if strings.TrimSpace(cfg.HostnameOverride) != "" {
		cfg.EffectiveHostname = cfg.HostnameOverride
	} else {
		hostname, err := os.Hostname()
		if err != nil {
			return nil, fmt.Errorf("failed to get OS hostname: %w", err)
		}
		cfg.EffectiveHostname = hostname
	}


	for i := range cfg.Alerts {
		rule := &cfg.Alerts[i]
		if rule.Name == "" {
			return nil, fmt.Errorf("alert rule at index %d missing name", i)
		}
		if rule.Metric == "" {
			return nil, fmt.Errorf("alert rule '%s' missing metric", rule.Name)
		}
		// Validate condition, aggregation, etc.
		switch strings.ToLower(rule.Aggregation) {
		case "average", "max", "":
			// OK
		default:
			return nil, fmt.Errorf("alert rule '%s' has invalid aggregation '%s'", rule.Name, rule.Aggregation)
		}
		if rule.DurationStr != "" {
			rule.Duration, err = util.ParseDurationString(rule.DurationStr)
			if err != nil {
				return nil, fmt.Errorf("alert rule '%s' has invalid duration: %w", rule.Name, err)
			}
		}
		if len(rule.Channels) == 0 {
			return nil, fmt.Errorf("alert rule '%s' has no notification channels defined", rule.Name)
		}
	}

	for i := range cfg.NotificationChannels {
		nc := &cfg.NotificationChannels[i]
		if nc.Name == "" {
			return nil, fmt.Errorf("notification channel at index %d missing name", i)
		}
		// Load sensitive data from ENV vars
		// Naming convention: RESMON_<SENSITIVE_FIELD_NAME>_<CHANNEL_NAME_UPPERCASE>
		// e.g., RESMON_SMTP_PASSWORD_CRITICAL_EMAIL
		// e.g., RESMON_TELEGRAM_TOKEN_OPS_TELEGRAM
		envVarPrefix := "RESMON_"
		channelNameUpper := strings.ToUpper(strings.ReplaceAll(nc.Name, "-", "_"))

		switch nc.Type {
		case "email":
			passwordEnvKey := fmt.Sprintf("%sSMTP_PASSWORD_%s", envVarPrefix, channelNameUpper)
			if pass := os.Getenv(passwordEnvKey); pass != "" {
				if nc.Config == nil { nc.Config = make(map[string]interface{})}
				nc.Config["smtp_password"] = pass
			} else {
				// Check if password was in config (it shouldn't be)
				if _, ok := nc.Config["smtp_password"]; ok && nc.Config["smtp_password"] != "" {
					// Log warning, but it will be ignored in favor of ENV var (which is empty here)
					fmt.Printf("Warning: SMTP password for channel '%s' found in config file. It should be set via ENV var %s.\n", nc.Name, passwordEnvKey)
				}
				// If not in ENV and critical, could be an error or handled by notifier init
			}
		case "telegram":
			tokenEnvKey := fmt.Sprintf("%sTELEGRAM_TOKEN_%s", envVarPrefix, channelNameUpper)
			if token := os.Getenv(tokenEnvKey); token != "" {
				if nc.Config == nil { nc.Config = make(map[string]interface{})}
				nc.Config["bot_token"] = token
			} else {
				if _, ok := nc.Config["bot_token"]; ok && nc.Config["bot_token"] != "" {
					fmt.Printf("Warning: Telegram bot token for channel '%s' found in config file. It should be set via ENV var %s.\n", nc.Name, tokenEnvKey)
				}
			}
		case "stdout":
			// No sensitive data, just a simple channel
		default:
			return nil, fmt.Errorf("notification channel '%s' has unknown type '%s'", nc.Name, nc.Type)
		}
	}

	// Default templates
	if cfg.Templates.AlertFired == "" {
		cfg.Templates.AlertFired = `ALERT FIRED: {{.AlertName}} on {{.Hostname}}. Metric: {{.MetricName}} {{.Condition}} {{.ThresholdValue}} (Current: {{printf "%.2f" .MetricValue}}). Time: {{.Time.Format "2006-01-02 15:04:05"}}`
	}
	if cfg.Templates.AlertResolved == "" {
		cfg.Templates.AlertResolved = `ALERT RESOLVED: {{.AlertName}} on {{.Hostname}}. Time: {{.Time.Format "2006-01-02 15:04:05"}}`
	}

	return &cfg, nil
}

// Helper to get typed Email config
func GetEmailChannelConfig(nc NotificationChannelConfig) (*EmailChannelConfig, error) {
	if nc.Type != "email" {
		return nil, fmt.Errorf("not an email channel")
	}
	var emailCfg EmailChannelConfig
	// Simple conversion assuming map keys match struct fields (after lowercasing/snake_case)
	// A more robust way is to use a library like mapstructure if complex
	if host, ok := nc.Config["smtp_host"].(string); ok { emailCfg.SMTPHost = host } else { return nil, fmt.Errorf("channel '%s': smtp_host missing or not a string", nc.Name)}
	if port, ok := nc.Config["smtp_port"].(int); ok { emailCfg.SMTPPort = port } else { return nil, fmt.Errorf("channel '%s': smtp_port missing or not an int", nc.Name)}
	if user, ok := nc.Config["smtp_username"].(string); ok { emailCfg.SMTPUsername = user }
	if pass, ok := nc.Config["smtp_password"].(string); ok { emailCfg.SMTPPassword = pass } // Already from ENV
	if from, ok := nc.Config["smtp_from"].(string); ok { emailCfg.SMTPFrom = from } else { return nil, fmt.Errorf("channel '%s': smtp_from missing or not a string", nc.Name)}
	if toVal, ok := nc.Config["smtp_to"].([]interface{}); ok {
		for _, t := range toVal {
			if tStr, ok := t.(string); ok {
				emailCfg.SMTPTo = append(emailCfg.SMTPTo, tStr)
			}
		}
	} else { return nil, fmt.Errorf("channel '%s': smtp_to missing or not a list of strings", nc.Name)}
	if useTLS, ok := nc.Config["smtp_use_tls"].(bool); ok { emailCfg.SMTPUseTLS = useTLS}

	if emailCfg.SMTPHost == "" || emailCfg.SMTPPort == 0 || emailCfg.SMTPFrom == "" || len(emailCfg.SMTPTo) == 0 {
		return nil, fmt.Errorf("channel '%s': one or more required email config fields are missing (host, port, from, to)", nc.Name)
	}
	// Username/Password can be optional for some SMTP servers
	return &emailCfg, nil
}

// Helper to get typed Telegram config
func GetTelegramChannelConfig(nc NotificationChannelConfig) (*TelegramChannelConfig, error) {
	if nc.Type != "telegram" {
		return nil, fmt.Errorf("not a telegram channel")
	}
	var telegramCfg TelegramChannelConfig
	if token, ok := nc.Config["bot_token"].(string); ok { telegramCfg.BotToken = token } // Already from ENV
	if chatID, ok := nc.Config["chat_id"].(string); ok { telegramCfg.ChatID = chatID } else { return nil, fmt.Errorf("channel '%s': chat_id missing or not a string", nc.Name) }

	if telegramCfg.BotToken == "" || telegramCfg.ChatID == "" {
		 return nil, fmt.Errorf("channel '%s': bot_token (from ENV) or chat_id are missing", nc.Name)
	}
	return &telegramCfg, nil
}

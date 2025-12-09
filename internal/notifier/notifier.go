package notifier

import (
	"bytes"
	"fmt"
	"log"
	"strings"
	gotexttemplate "text/template"
	"time"

	"github.com/mattmezza/monres/internal/config"
)

// NotificationData is the data passed to templates.
type NotificationData struct {
	AlertName      string
	MetricName     string
	MetricValue    float64
	ThresholdValue float64
	Condition      string
	State          string // "FIRED" or "RESOLVED"
	Hostname       string
	Time           time.Time
	DurationString string // e.g. "5m"
	Aggregation    string // e.g. "average"

	// Pre-formatted fields for human-readable display
	FormattedMetricValue    string // e.g. "525.5 MB/s" or "85.5%"
	FormattedThresholdValue string // e.g. "500.0 MB/s" or "90.0%"
}

type NotificationTemplates struct {
	FiredTemplate    string
	ResolvedTemplate string
}

// Notifier is the interface for all notification channel types.
type Notifier interface {
	Send(data NotificationData, templates NotificationTemplates) error
	Name() string // Returns the configured channel name
}

func renderTemplate(templateName string, templateStr string, data NotificationData) (string, error) {
	// Using text/template as per requirements. If HTML emails were a primary concern, html/template would be safer.
	tmpl, err := gotexttemplate.New(templateName).Parse(templateStr)
	if err != nil {
		return "", fmt.Errorf("failed to parse notification template '%s': %w", templateName, err)
	}

	var buf bytes.Buffer
	err = tmpl.Execute(&buf, data)
	if err != nil {
		return "", fmt.Errorf("failed to execute notification template '%s': %w", templateName, err)
	}
	return buf.String(), nil
}


func InitializeNotifiers(cfgNotifChannels []config.NotificationChannelConfig) (map[string]Notifier, error) {
    notifiers := make(map[string]Notifier)
    for _, ncCfg := range cfgNotifChannels {
        var instance Notifier
        var err error
        switch ncCfg.Type {
        case "email":
            emailCfg, convErr := config.GetEmailChannelConfig(ncCfg)
            if convErr != nil {
                 log.Printf("Skipping email channel '%s' due to config error: %v", ncCfg.Name, convErr)
                 continue
            }
            instance, err = NewEmailNotifier(ncCfg.Name, *emailCfg)
        case "telegram":
            telegramCfg, convErr := config.GetTelegramChannelConfig(ncCfg)
             if convErr != nil {
                 log.Printf("Skipping telegram channel '%s' due to config error: %v", ncCfg.Name, convErr)
                 continue
            }
            instance, err = NewTelegramNotifier(ncCfg.Name, *telegramCfg)
		case "stdout":
			instance, err = NewStdoutNotifier(ncCfg.Name)
        default:
            log.Printf("Unsupported notification channel type '%s' for channel '%s'. Skipping.", ncCfg.Type, ncCfg.Name)
            continue
        }

        if err != nil {
            log.Printf("Failed to initialize notifier for channel '%s' (%s): %v. Skipping.", ncCfg.Name, ncCfg.Type, err)
            continue
        }
        if _, exists := notifiers[ncCfg.Name]; exists {
            return nil, fmt.Errorf("duplicate notification channel name defined: %s", ncCfg.Name)
        }
        notifiers[ncCfg.Name] = instance
        log.Printf("Successfully initialized notifier for channel: %s (type: %s)", ncCfg.Name, ncCfg.Type)
    }
    return notifiers, nil
}

// FormatValue formats a numeric value based on the metric name.
// Returns a human-readable string with appropriate units.
func FormatValue(metricName string, value float64) string {
	switch {
	case strings.HasSuffix(metricName, "_bytes_ps"):
		return formatBytesPerSecond(value)
	case strings.Contains(metricName, "_percent_"):
		return formatPercent(value)
	default:
		return fmt.Sprintf("%.2f", value)
	}
}

// formatBytesPerSecond converts bytes/s to human-readable format (B/s, KB/s, MB/s, GB/s)
func formatBytesPerSecond(bytes float64) string {
	const (
		KB = 1024.0
		MB = KB * 1024
		GB = MB * 1024
	)

	switch {
	case bytes >= GB:
		return fmt.Sprintf("%.1f GB/s", bytes/GB)
	case bytes >= MB:
		return fmt.Sprintf("%.1f MB/s", bytes/MB)
	case bytes >= KB:
		return fmt.Sprintf("%.1f KB/s", bytes/KB)
	default:
		return fmt.Sprintf("%.1f B/s", bytes)
	}
}

// formatPercent formats a percentage value with % suffix
func formatPercent(value float64) string {
	return fmt.Sprintf("%.1f%%", value)
}

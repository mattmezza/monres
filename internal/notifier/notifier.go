package notifier

import (
	"bytes"
	"fmt"
	gotexttemplate "text/template"
	"time"
	"log"

	"github.com/mattmezza/resmon/internal/config"
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

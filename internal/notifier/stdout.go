package notifier

import (
	"fmt"
)

type StdoutNotifier struct {
	name   string
}

func NewStdoutNotifier(name string) (*StdoutNotifier, error) {
	return &StdoutNotifier{
		name:   name,
	}, nil
}

func (sout *StdoutNotifier) Name() string {
	return sout.name
}

func (sout *StdoutNotifier) Send(data NotificationData, templates NotificationTemplates) error {
	var templateToUse string
	if data.State == "RESOLVED" {
		templateToUse = templates.ResolvedTemplate
	} else {
		templateToUse = templates.FiredTemplate
	}

	// Render the template (which is plain text)
	msg , err := renderTemplate("telegram_message", templateToUse, data)
	if err != nil {
		return fmt.Errorf("failed to render Telegram template for alert '%s': %w", data.AlertName, err)
	}

	// Print to Stdout
	fmt.Printf("%s\n", msg)

	return nil
}

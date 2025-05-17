package notifier

import (
	"crypto/tls"
	"fmt"
	"net/smtp"
	"strings"

	"github.com/mattmezza/resmon/internal/config"
)

type EmailNotifier struct {
	name   string
	config config.EmailChannelConfig
}

func NewEmailNotifier(name string, cfg config.EmailChannelConfig) (*EmailNotifier, error) {
	if cfg.SMTPHost == "" || cfg.SMTPPort == 0 || cfg.SMTPFrom == "" || len(cfg.SMTPTo) == 0 {
		return nil, fmt.Errorf("email notifier '%s' is missing required configuration (host, port, from, to)", name)
	}
	// Password check is tricky: it might be optional for some non-auth SMTP relays.
	// If username is present, password should ideally be present.
	if cfg.SMTPUsername != "" && cfg.SMTPPassword == "" {
		// This indicates RESMON_SMTP_PASSWORD_xyz was not set.
		// Depending on strictness, this could be an error. For now, allow it and let SMTP server reject.
		// log.Printf("Warning: Email notifier '%s' has a username but no password. SMTP auth might fail.", name)
	}

	return &EmailNotifier{name: name, config: cfg}, nil
}

func (en *EmailNotifier) Name() string {
	return en.name
}

func (en *EmailNotifier) Send(data NotificationData, templates NotificationTemplates) error {
	var subject, body string
	var err error

	templateToUse := templates.FiredTemplate
	subjectPrefix := "ALERT FIRED"
	if data.State == "RESOLVED" {
		templateToUse = templates.ResolvedTemplate
		subjectPrefix = "ALERT RESOLVED"
	}

	subject = fmt.Sprintf("%s: %s on %s", subjectPrefix, data.AlertName, data.Hostname)
	body, err = renderTemplate("email_body", templateToUse, data)
	if err != nil {
		return fmt.Errorf("failed to render email template for alert '%s': %w", data.AlertName, err)
	}

	// Construct message
	// MIME headers are important for many email clients
	toList := strings.Join(en.config.SMTPTo, ",")
	msg := []byte(fmt.Sprintf("To: %s\r\n"+
		"From: %s\r\n"+
		"Subject: %s\r\n"+
		"Content-Type: text/plain; charset=UTF-8\r\n"+
		"\r\n"+
		"%s\r\n", toList, en.config.SMTPFrom, subject, body))

	addr := fmt.Sprintf("%s:%d", en.config.SMTPHost, en.config.SMTPPort)
	var auth smtp.Auth
	if en.config.SMTPUsername != "" {
		auth = smtp.PlainAuth("", en.config.SMTPUsername, en.config.SMTPPassword, en.config.SMTPHost)
	}

	if en.config.SMTPUseTLS { // STARTTLS
		// Connect to the server, tell it we want to use TLS, and then switch to TLS.
		client, err := smtp.Dial(addr)
		if err != nil {
			return fmt.Errorf("failed to dial SMTP server (pre-TLS): %w", err)
		}
		defer client.Close()

		if ok, _ := client.Extension("STARTTLS"); ok {
			tlsConfig := &tls.Config{
				ServerName: en.config.SMTPHost,
				// InsecureSkipVerify: true, // Not recommended for production
			}
			if err = client.StartTLS(tlsConfig); err != nil {
				return fmt.Errorf("failed to start TLS with SMTP server: %w", err)
			}
		} else {
			// Server does not support STARTTLS, but config said to use it.
			// Or, if port is 465 (SMTPS), direct TLS connection is needed, not STARTTLS.
			// This simple client does not handle direct SMTPS on 465 well.
			// For port 465, a different approach is needed: tls.Dial then smtp.NewClient
			if en.config.SMTPPort == 465 { // SMTPS often on 465
                 return fmt.Errorf("STARTTLS configured, but port 465 suggests direct SSL/TLS. This client uses STARTTLS for smtp_use_tls=true. For port 465, explicit SSL/TLS connection is needed (not implemented in this basic SMTP sender).")
            }
			return fmt.Errorf("SMTP server does not support STARTTLS, but smtp_use_tls was true")
		}

		// Authenticate if credentials are provided
		if auth != nil {
			if err = client.Auth(auth); err != nil {
				return fmt.Errorf("SMTP authentication failed: %w", err)
			}
		}
		// Send email
		if err = client.Mail(extractEmail(en.config.SMTPFrom)); err != nil {
			return fmt.Errorf("SMTP MAIL FROM failed: %w", err)
		}
		for _, rcpt := range en.config.SMTPTo {
			if err = client.Rcpt(extractEmail(rcpt)); err != nil {
				return fmt.Errorf("SMTP RCPT TO failed for %s: %w", rcpt, err)
			}
		}
		w, err := client.Data()
		if err != nil {
			return fmt.Errorf("SMTP DATA command failed: %w", err)
		}
		_, err = w.Write(msg)
		if err != nil {
			return fmt.Errorf("failed to write email body: %w", err)
		}
		err = w.Close()
		if err != nil {
			return fmt.Errorf("failed to close email data writer: %w", err)
		}
		return client.Quit()

	} else { // Plain SMTP
		err = smtp.SendMail(addr, auth, en.config.SMTPFrom, en.config.SMTPTo, msg)
		if err != nil {
			return fmt.Errorf("failed to send email via plain SMTP: %w", err)
		}
	}

	return nil
}

// extractEmail parses "Display Name <email@example.com>" and returns "email@example.com"
func extractEmail(fullEmail string) string {
	if strings.Contains(fullEmail, "<") && strings.Contains(fullEmail, ">") {
		start := strings.LastIndex(fullEmail, "<")
		end := strings.LastIndex(fullEmail, ">")
		if start != -1 && end != -1 && end > start {
			return fullEmail[start+1 : end]
		}
	}
	return fullEmail // Assume it's already just the email address
}

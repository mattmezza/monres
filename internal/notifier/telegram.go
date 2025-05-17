package notifier

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/mattmezza/resmon/internal/config"
)

type TelegramNotifier struct {
	name   string
	config config.TelegramChannelConfig
	client *http.Client
}

func NewTelegramNotifier(name string, cfg config.TelegramChannelConfig) (*TelegramNotifier, error) {
	if cfg.BotToken == "" || cfg.ChatID == "" {
		return nil, fmt.Errorf("telegram notifier '%s' is missing bot_token (from ENV) or chat_id", name)
	}
	return &TelegramNotifier{
		name:   name,
		config: cfg,
		client: &http.Client{Timeout: 10 * time.Second},
	}, nil
}

func (tn *TelegramNotifier) Name() string {
	return tn.name
}

// Send sends a message to Telegram.
// Telegram API prefers MarkdownV2 or HTML for formatting. Let's use MarkdownV2.
// Note: text/template output needs to be escaped for MarkdownV2.
func (tn *TelegramNotifier) Send(data NotificationData, templates NotificationTemplates) error {
	var templateToUse string
	if data.State == "RESOLVED" {
		templateToUse = templates.ResolvedTemplate
	} else {
		templateToUse = templates.FiredTemplate
	}

	// Render the template (which is plain text)
	rawMessage, err := renderTemplate("telegram_message", templateToUse, data)
	if err != nil {
		return fmt.Errorf("failed to render Telegram template for alert '%s': %w", data.AlertName, err)
	}

	// Telegram API expects MarkdownV2 or HTML.
	// The default templates are simple text. For MarkdownV2, special chars need escaping.
	// For this version, we'll send as plain text (MarkdownV2 without special chars).
	// A more advanced version could allow Markdown in templates and then escape it here, or use HTML.
	// For now, we assume templates produce fairly plain text.
	// Telegram's parse_mode MarkdownV2 requires escaping characters like '.', '!', '-', '(', ')', etc.
	// For simplicity, let's use plain text and not set parse_mode or use "Markdown" which is more lenient but deprecated.
	// The example templates have '🔥' and '✅', which are fine.
	// Let's try with "MarkdownV2" and a simple escaper for critical characters.

	escapedMessage := escapeTextForMarkdownV2(rawMessage)

	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", tn.config.BotToken)

	payload := map[string]string{
		"chat_id":    tn.config.ChatID,
		"text":       escapedMessage,
		"parse_mode": "MarkdownV2", // Specify parse mode
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal Telegram payload: %w", err)
	}

	req, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return fmt.Errorf("failed to create Telegram request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := tn.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send message to Telegram API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var bodyBytes []byte
		bodyBytes, _ =ReadAll(resp.Body) // ioutil.ReadAll is deprecated
		return fmt.Errorf("telegram API request failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}

// escapeTextForMarkdownV2 escapes text for Telegram MarkdownV2.
// Telegram requires escaping: _ * [ ] ( ) ~ ` > # + - = | { } . !
func escapeTextForMarkdownV2(text string) string {
	escapeChars := []string{"_", "*", "[", "]", "(", ")", "~", "`", ">", "#", "+", "-", "=", "|", "{", "}", ".", "!"}
	var result strings.Builder
	for _, r := range text {
		char := string(r)
		shouldEscape := false
		for _, esc := range escapeChars {
			if char == esc {
				shouldEscape = true
				break
			}
		}
		if shouldEscape {
			result.WriteString("\\")
		}
		result.WriteString(char)
	}
	return result.String()
}

// Helper to read all from io.Reader (like ioutil.ReadAll)
func ReadAll(r io.Reader) ([]byte, error) {
    var b bytes.Buffer
    _, err := b.ReadFrom(r)
    return b.Bytes(), err
}

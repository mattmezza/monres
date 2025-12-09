package alerter

import (
	"log"
	"sync"
	"time"

	"github.com/mattmezza/monres/internal/collector"
	"github.com/mattmezza/monres/internal/config"
	"github.com/mattmezza/monres/internal/history"
	"github.com/mattmezza/monres/internal/notifier"
	"github.com/mattmezza/monres/internal/state"
)

type EventType string

const (
	EventTypeFired    EventType = "FIRED"
	EventTypeResolved EventType = "RESOLVED"
)

type AlertEvent struct {
	Rule          *AlertRule
	Type          EventType
	Hostname      string
	Timestamp     time.Time
	MetricValue   float64 // The value that caused the state change
	TriggeringPoints []history.DataPoint // Optional: points that led to this state
}

type Alerter struct {
	rules         []*AlertRule
	historyBuffer *history.MetricHistoryBuffer
	notifiers     map[string]notifier.Notifier // map channel name to notifier instance
	templates     notifier.NotificationTemplates
	hostname      string
	mu            sync.Mutex // Protects rules' states
}

func NewAlerter(cfg *config.Config, histBuffer *history.MetricHistoryBuffer, configuredNotifiers map[string]notifier.Notifier) (*Alerter, error) {
	a := &Alerter{
		historyBuffer: histBuffer,
		notifiers:     configuredNotifiers,
		hostname:      cfg.EffectiveHostname,
		templates: notifier.NotificationTemplates{
			FiredTemplate:    cfg.Templates.AlertFired,
			ResolvedTemplate: cfg.Templates.AlertResolved,
		},
	}

	for _, ruleCfg := range cfg.Alerts {
		rule := NewAlertRule(ruleCfg)
		a.rules = append(a.rules, rule)
	}

	return a, nil
}

// CheckAndNotify evaluates all rules and sends notifications if state changes.
func (a *Alerter) CheckAndNotify(now time.Time, currentMetrics collector.CollectedMetrics) {
	a.mu.Lock()
	defer a.mu.Unlock()

	var events []AlertEvent

	for _, rule := range a.rules {
		metricValuePoints := a.historyBuffer.GetDataPointsForDuration(rule.Metric, rule.Duration, now)

		// Rule evaluation can only happen if enough data exists for the duration window
		if rule.Duration > 0 {
			if len(metricValuePoints) == 0 {
				log.Printf("Alerter: Not enough data points yet for duration based rule '%s' (metric: %s, duration: %s). Skipping.", rule.Name, rule.Metric, rule.DurationStr)
				continue // Not enough data yet
			}
			// Check if the actual timespan of collected points covers the rule's duration
			// This is crucial for new services or after gaps in collection
			if len(metricValuePoints) > 0 {
				firstPointTime := metricValuePoints[0].Timestamp
				// Allow a small tolerance (e.g., 100ms) for time variations
				if now.Sub(firstPointTime) < rule.Duration - 100*time.Millisecond {
					log.Printf("Alerter: Data points for rule '%s' (metric: %s) span %s, which is less than required duration %s. Skipping.",
					rule.Name, rule.Metric, now.Sub(firstPointTime).String(), rule.Duration.String())
					continue // Not enough history accumulated yet
				}
			}
		} else { // Instantaneous alert
		    latestDP, exists := a.historyBuffer.GetLatestDataPoint(rule.Metric)
		    if !exists {
		        log.Printf("Alerter: No data point found for instantaneous rule '%s' (metric: %s). Skipping.", rule.Name, rule.Metric)
		        continue
		    }
		    metricValuePoints = []history.DataPoint{latestDP} // Evaluate on this single point
		}


		conditionMet, aggregatedValue, err := rule.Evaluate(metricValuePoints)
		if err != nil {
			log.Printf("Error evaluating rule '%s': %v", rule.Name, err)
			continue
		}

		if conditionMet && !rule.State.IsActive {
			// Alert FIRED
			rule.State.IsActive = true
			rule.State.LastActiveTime = now
			rule.State.LastValue = aggregatedValue
			events = append(events, AlertEvent{
				Rule:        rule,
				Type:        EventTypeFired,
				Hostname:    a.hostname,
				Timestamp:   now,
				MetricValue: aggregatedValue,
			})
			log.Printf("ALERT FIRED: %s (Metric: %s %s %.2f, Current: %.2f)", rule.Name, rule.Metric, rule.Condition, rule.Threshold, aggregatedValue)

		} else if !conditionMet && rule.State.IsActive {
			// Alert RESOLVED
			rule.State.IsActive = false
			rule.State.LastResolvedTime = now
			rule.State.LastValue = aggregatedValue // Value at time of resolution
			events = append(events, AlertEvent{
				Rule:        rule,
				Type:        EventTypeResolved,
				Hostname:    a.hostname,
				Timestamp:   now,
				MetricValue: aggregatedValue,  // Could be current value which is now "good"
			})
			log.Printf("ALERT RESOLVED: %s", rule.Name)
		}
	}

	// Send notifications outside the loop to avoid holding lock for too long if notifiers are slow
	// Unlock isn't needed here if defer is used, but good to keep in mind for complex locking
	// a.mu.Unlock()

	for _, event := range events {
		a.sendNotificationsForRule(event)
	}
    // a.mu.Lock() // Re-lock if needed for further state ops, covered by defer
}

func (a *Alerter) sendNotificationsForRule(event AlertEvent) {
	for _, channelName := range event.Rule.Channels {
		notifierInstance, ok := a.notifiers[channelName]
		if !ok {
			log.Printf("Warning: Notification channel '%s' for alert '%s' not found/configured.", channelName, event.Rule.Name)
			continue
		}

		// Prepare notification context
		data := notifier.NotificationData{
			AlertName:      event.Rule.Name,
			MetricName:     event.Rule.Metric,
			MetricValue:    event.MetricValue, // The value causing state change
			ThresholdValue: event.Rule.Threshold,
			Condition:      event.Rule.Condition,
			State:          string(event.Type),
			Hostname:       a.hostname,
			Time:           event.Timestamp,
			DurationString: event.Rule.DurationStr,
			Aggregation:    event.Rule.Aggregation,
			// Human-readable formatted values
			FormattedMetricValue:    notifier.FormatValue(event.Rule.Metric, event.MetricValue),
			FormattedThresholdValue: notifier.FormatValue(event.Rule.Metric, event.Rule.Threshold),
		}

		err := notifierInstance.Send(data, a.templates)
		if err != nil {
			log.Printf("Failed to send notification for alert '%s' via channel '%s': %v", event.Rule.Name, channelName, err)
		} else {
			log.Printf("Notification sent for alert '%s' via channel '%s' (State: %s)", event.Rule.Name, channelName, event.Type)
		}
	}
}

// GetCurrentActiveAlerts returns a map of active alert names for state saving.
func (a *Alerter) GetCurrentActiveAlerts() state.ActiveAlertsState {
	a.mu.Lock()
	defer a.mu.Unlock()

	activeStates := make(state.ActiveAlertsState)
	for _, rule := range a.rules {
		if rule.State.IsActive {
			activeStates[rule.Name] = true
		}
	}
	return activeStates
}

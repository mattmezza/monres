package alerter

import (
	"fmt"
	"strings"
	"time"

	"github.com/mattmezza/monres/internal/config" // Corrected import path
	"github.com/mattmezza/monres/internal/history"
)

// AlertState represents the current status of an alert.
type AlertState struct {
	IsActive         bool
	LastActiveTime   time.Time // When it last became active
	LastResolvedTime time.Time // When it last became resolved
	LastValue        float64   // The value that triggered/resolved the alert
}

// AlertRule is the runtime representation of an alert rule.
type AlertRule struct {
	config.AlertRuleConfig
	State AlertState
}

func NewAlertRule(cfg config.AlertRuleConfig) *AlertRule {
	return &AlertRule{
		AlertRuleConfig: cfg,
		State: AlertState{
			IsActive: false, // Initial state
		},
	}
}

// Evaluate processes a set of data points against the rule.
// Returns true if the alert condition is met, the aggregated value, and any error.
func (ar *AlertRule) Evaluate(points []history.DataPoint) (conditionMet bool, aggregatedValue float64, err error) {
	if len(points) == 0 && ar.Duration > 0 {
		return false, 0, fmt.Errorf("not enough data points for duration-based alert '%s'", ar.Name)
	}
    if len(points) == 0 && ar.Duration == 0 { // Instantaneous check but no data yet
        return false, 0, fmt.Errorf("no data point available for instantaneous alert '%s'", ar.Name)
    }


	var valueToCompare float64

	if ar.Duration == 0 { // Instantaneous: use the latest point
		if len(points) > 0 {
			valueToCompare = points[len(points)-1].Value
		} else {
			return false, 0, fmt.Errorf("no data points for instantaneous alert '%s'", ar.Name) // Should be caught earlier
		}
	} else { // Duration-based: aggregate
		// Ensure we have enough data for the duration
		if len(points) == 0 {
			return false, 0, fmt.Errorf("not enough data points (0) for duration '%s' for alert '%s'", ar.DurationStr, ar.Name)
		}
		// Check if the timespan of points covers the required duration.
		// The history buffer should ideally provide points *within* the duration window.
		// For simplicity, we assume `points` are correctly filtered by the history buffer.
		// If not, an additional check here:
		// firstPointTime := points[0].Timestamp
		// lastPointTime := points[len(points)-1].Timestamp
		// if lastPointTime.Sub(firstPointTime) < ar.Duration {
		//     return false, 0, fmt.Errorf("data points span %s, less than required %s for alert '%s'",
		//         lastPointTime.Sub(firstPointTime).String(), ar.DurationStr, ar.Name)
		// }


		switch strings.ToLower(ar.Aggregation) {
		case "average":
			sum := 0.0
			for _, dp := range points {
				sum += dp.Value
			}
			valueToCompare = sum / float64(len(points))
		case "max":
			if len(points) > 0 {
				valueToCompare = points[0].Value
				for _, dp := range points {
					if dp.Value > valueToCompare {
						valueToCompare = dp.Value
					}
				}
			} else {
                return false, 0, fmt.Errorf("no data points to calculate max for alert '%s'", ar.Name)
            }
		default: // Should be caught by config validation, but default to average or error.
			return false, 0, fmt.Errorf("unknown aggregation type '%s' for alert '%s'", ar.Aggregation, ar.Name)
		}
	}

	aggregatedValue = valueToCompare // This is the value to report

	switch ar.Condition {
	case ">":
		conditionMet = valueToCompare > ar.Threshold
	case "<":
		conditionMet = valueToCompare < ar.Threshold
	case "=":
		conditionMet = valueToCompare == ar.Threshold // Float equality can be tricky
	case "!=":
		conditionMet = valueToCompare != ar.Threshold
	case ">=":
		conditionMet = valueToCompare >= ar.Threshold
	case "<=":
		conditionMet = valueToCompare <= ar.Threshold
	default:
		return false, valueToCompare, fmt.Errorf("unknown condition '%s' for alert '%s'", ar.Condition, ar.Name)
	}

	return conditionMet, aggregatedValue, nil
}

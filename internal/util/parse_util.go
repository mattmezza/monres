package util

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var durationRegex = regexp.MustCompile(`^(\d+)([smh])$`)

// ParseDurationString converts strings like "5m", "300s", "1h" into time.Duration.
func ParseDurationString(durationStr string) (time.Duration, error) {
	if durationStr == "" || durationStr == "0" || durationStr == "0s" || durationStr == "0m" || durationStr == "0h" {
		return 0, nil
	}

	matches := durationRegex.FindStringSubmatch(strings.ToLower(durationStr))
	if len(matches) != 3 {
		return 0, fmt.Errorf("invalid duration string format: %s. Use '10s', '5m', '1h'", durationStr)
	}

	value, err := strconv.Atoi(matches[1])
	if err != nil {
		// Should not happen due to regex, but good practice
		return 0, fmt.Errorf("invalid duration numeric value: %s", matches[1])
	}

	unit := matches[2]
	var durationUnit time.Duration
	switch unit {
	case "s":
		durationUnit = time.Second
	case "m":
		durationUnit = time.Minute
	case "h":
		durationUnit = time.Hour
	default:
		// Should not happen due to regex
		return 0, fmt.Errorf("invalid duration unit: %s", unit)
	}

	return time.Duration(value) * durationUnit, nil
}

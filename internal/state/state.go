package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// ActiveAlertsState stores the names of alerts that are currently active.
// The value could be a struct with more info like activation time if needed later.
type ActiveAlertsState map[string]bool // alertName -> true if active

// LoadState loads the active alerts from the state file.
func LoadState(filePath string) (ActiveAlertsState, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return make(ActiveAlertsState), nil // No state file yet, return empty state
		}
		return nil, fmt.Errorf("failed to read state file %s: %w", filePath, err)
	}

	if len(data) == 0 { // Empty file
	    return make(ActiveAlertsState), nil
	}

	var state ActiveAlertsState
	err = json.Unmarshal(data, &state)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal state JSON from %s: %w", filePath, err)
	}
	if state == nil { // JSON was 'null' or empty array that unmarshalled to nil map
		return make(ActiveAlertsState), nil
	}
	return state, nil
}

// SaveState saves the current active alerts to the state file.
func SaveState(filePath string, activeAlerts ActiveAlertsState) error {
	data, err := json.MarshalIndent(activeAlerts, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal state to JSON: %w", err)
	}

	// Ensure directory exists
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0750); err != nil { // rwxr-x--- for dir
		return fmt.Errorf("failed to create state directory %s: %w", dir, err)
	}

	// Write with strict permissions (0600: rw------- for user only)
	err = os.WriteFile(filePath, data, 0600)
	if err != nil {
		return fmt.Errorf("failed to write state file %s: %w", filePath, err)
	}
	return nil
}

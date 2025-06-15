package state

// ActiveAlertsState stores the names of alerts that are currently active.
// The value could be a struct with more info like activation time if needed later.
type ActiveAlertsState map[string]bool // alertName -> true if active

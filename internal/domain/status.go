package domain

import (
	"fmt"
	"sort"
	"strings"
)

// Valid status values per entity type.
var validProjectStatuses = map[string]bool{"active": true, "paused": true, "archived": true}
var validBranchStatuses  = map[string]bool{"active": true, "merged": true, "abandoned": true}
var validThreadStatuses  = map[string]bool{"open": true, "decided": true, "abandoned": true}
var validTaskStatuses    = map[string]bool{"todo": true, "in-progress": true, "done": true, "cancelled": true}
var validTaskPriorities  = map[string]bool{"h": true, "m": true, "l": true}

// Valid task state transitions.
var validTaskTransitions = map[string]map[string]bool{
	"todo":        {"in-progress": true, "cancelled": true},
	"in-progress": {"done": true, "todo": true, "cancelled": true},
	"done":        {"todo": true},
	"cancelled":   {"todo": true},
}

func sortedKeys(m map[string]bool) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return strings.Join(keys, ", ")
}

// ValidateProjectStatus checks that the given status is valid for a project.
func ValidateProjectStatus(status string) error {
	if !validProjectStatuses[status] {
		return fmt.Errorf("invalid project status %q: must be one of %s", status, sortedKeys(validProjectStatuses))
	}
	return nil
}

// ValidateBranchStatus checks that the given status is valid for a branch.
func ValidateBranchStatus(status string) error {
	if !validBranchStatuses[status] {
		return fmt.Errorf("invalid branch status %q: must be one of %s", status, sortedKeys(validBranchStatuses))
	}
	return nil
}

// ValidateThreadStatus checks that the given status is valid for a thread.
func ValidateThreadStatus(status string) error {
	if !validThreadStatuses[status] {
		return fmt.Errorf("invalid thread status %q: must be one of %s", status, sortedKeys(validThreadStatuses))
	}
	return nil
}

// ValidateTaskStatus checks that the given status is valid for a task.
func ValidateTaskStatus(status string) error {
	if !validTaskStatuses[status] {
		return fmt.Errorf("invalid task status %q: must be one of %s", status, sortedKeys(validTaskStatuses))
	}
	return nil
}

// ValidateTaskPriority checks that the given priority is valid for a task.
func ValidateTaskPriority(priority string) error {
	if !validTaskPriorities[priority] {
		return fmt.Errorf("invalid task priority %q: must be one of %s", priority, sortedKeys(validTaskPriorities))
	}
	return nil
}

// ValidateTaskTransition checks that the state transition from -> to is allowed.
// If from is not a recognized status (e.g. legacy data), the transition is allowed
// to permit escaping from invalid states.
func ValidateTaskTransition(from, to string) error {
	// Allow transition from unrecognized statuses (escape from invalid state)
	allowed, knownFrom := validTaskTransitions[from]
	if !knownFrom {
		return nil
	}
	if !allowed[to] {
		return fmt.Errorf("invalid task transition: %s -> %s", from, to)
	}
	return nil
}

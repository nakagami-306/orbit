package domain

import "testing"

func TestValidateProjectStatus(t *testing.T) {
	valid := []string{"active", "paused", "archived"}
	for _, s := range valid {
		if err := ValidateProjectStatus(s); err != nil {
			t.Errorf("ValidateProjectStatus(%q) returned error: %v", s, err)
		}
	}

	invalid := []string{"wontfix", "invalid", "", "done", "open"}
	for _, s := range invalid {
		if err := ValidateProjectStatus(s); err == nil {
			t.Errorf("ValidateProjectStatus(%q) should have returned error", s)
		}
	}
}

func TestValidateBranchStatus(t *testing.T) {
	valid := []string{"active", "merged", "abandoned"}
	for _, s := range valid {
		if err := ValidateBranchStatus(s); err != nil {
			t.Errorf("ValidateBranchStatus(%q) returned error: %v", s, err)
		}
	}

	invalid := []string{"wontfix", "invalid", "", "done"}
	for _, s := range invalid {
		if err := ValidateBranchStatus(s); err == nil {
			t.Errorf("ValidateBranchStatus(%q) should have returned error", s)
		}
	}
}

func TestValidateThreadStatus(t *testing.T) {
	valid := []string{"open", "decided", "abandoned"}
	for _, s := range valid {
		if err := ValidateThreadStatus(s); err != nil {
			t.Errorf("ValidateThreadStatus(%q) returned error: %v", s, err)
		}
	}

	invalid := []string{"wontfix", "invalid", "", "closed", "done"}
	for _, s := range invalid {
		if err := ValidateThreadStatus(s); err == nil {
			t.Errorf("ValidateThreadStatus(%q) should have returned error", s)
		}
	}
}

func TestValidateTaskStatus(t *testing.T) {
	valid := []string{"todo", "in-progress", "done", "cancelled"}
	for _, s := range valid {
		if err := ValidateTaskStatus(s); err != nil {
			t.Errorf("ValidateTaskStatus(%q) returned error: %v", s, err)
		}
	}

	invalid := []string{"wontfix", "invalid", "", "open", "closed"}
	for _, s := range invalid {
		if err := ValidateTaskStatus(s); err == nil {
			t.Errorf("ValidateTaskStatus(%q) should have returned error", s)
		}
	}
}

func TestValidateTaskPriority(t *testing.T) {
	valid := []string{"h", "m", "l"}
	for _, p := range valid {
		if err := ValidateTaskPriority(p); err != nil {
			t.Errorf("ValidateTaskPriority(%q) returned error: %v", p, err)
		}
	}

	invalid := []string{"high", "medium", "low", "", "x", "H"}
	for _, p := range invalid {
		if err := ValidateTaskPriority(p); err == nil {
			t.Errorf("ValidateTaskPriority(%q) should have returned error", p)
		}
	}
}

func TestValidateTaskTransition_Valid(t *testing.T) {
	validTransitions := []struct{ from, to string }{
		{"todo", "in-progress"},
		{"todo", "cancelled"},
		{"in-progress", "done"},
		{"in-progress", "todo"},
		{"in-progress", "cancelled"},
		{"done", "todo"},
		{"cancelled", "todo"},
	}
	for _, tt := range validTransitions {
		if err := ValidateTaskTransition(tt.from, tt.to); err != nil {
			t.Errorf("ValidateTaskTransition(%q, %q) returned error: %v", tt.from, tt.to, err)
		}
	}
}

func TestValidateTaskTransition_Invalid(t *testing.T) {
	invalidTransitions := []struct{ from, to string }{
		{"done", "in-progress"},
		{"done", "cancelled"},
		{"cancelled", "in-progress"},
		{"cancelled", "done"},
		{"todo", "done"},
	}
	for _, tt := range invalidTransitions {
		if err := ValidateTaskTransition(tt.from, tt.to); err == nil {
			t.Errorf("ValidateTaskTransition(%q, %q) should have returned error", tt.from, tt.to)
		}
	}
}

func TestValidateTaskTransition_UnknownFrom(t *testing.T) {
	// Unknown from-status should allow any transition (escape from invalid state)
	unknownFromTransitions := []struct{ from, to string }{
		{"wontfix", "cancelled"},
		{"wontfix", "todo"},
		{"wontfix", "done"},
		{"unknown", "in-progress"},
		{"", "todo"},
	}
	for _, tt := range unknownFromTransitions {
		if err := ValidateTaskTransition(tt.from, tt.to); err != nil {
			t.Errorf("ValidateTaskTransition(%q, %q) should allow escape from unknown status, got error: %v", tt.from, tt.to, err)
		}
	}
}

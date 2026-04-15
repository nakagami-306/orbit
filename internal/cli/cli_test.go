package cli

import (
	"bytes"
	"encoding/json"
	"testing"
)

// TestRootCmdSilencesErrors verifies that SilenceUsage and SilenceErrors
// are set, so that Cobra does not print its own error/usage text.
func TestRootCmdSilencesErrors(t *testing.T) {
	root := NewRootCmd()

	if !root.SilenceUsage {
		t.Error("expected SilenceUsage to be true")
	}
	if !root.SilenceErrors {
		t.Error("expected SilenceErrors to be true")
	}
}

// TestRootCmdJSONError verifies that when --format json is set and the
// command fails, the error is returned (not printed by Cobra) so the
// caller (main.go) can output JSON to stderr.
func TestRootCmdJSONError(t *testing.T) {
	root := NewRootCmd()

	// Redirect stdout to capture any Cobra output
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)

	// Run a command that should fail (show requires a project context)
	root.SetArgs([]string{"--format", "json", "show"})
	err := root.Execute()

	if err == nil {
		t.Fatal("expected error from show command without project context")
	}

	// Verify that Cobra did NOT write its own error text to the buffer
	// (SilenceErrors should prevent this)
	output := buf.String()
	if len(output) > 0 {
		// Check it's not a plain text error (Cobra's default format)
		if !json.Valid([]byte(output)) {
			// Some output from Cobra is acceptable only if it's not an error message
			// With SilenceErrors=true and SilenceUsage=true, there should be no output
			t.Errorf("unexpected output from Cobra (should be silent): %q", output)
		}
	}
}

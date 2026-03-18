package output

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func TestFprintSuccess_NoColor(t *testing.T) {
	// Ensure color is disabled for predictable output
	ResetColorCache()
	t.Setenv("NO_COLOR", "1")
	defer ResetColorCache()

	var buf bytes.Buffer
	FprintSuccess(&buf, "Workflow %q created", "my-wf")

	got := buf.String()
	if !strings.Contains(got, "OK") {
		t.Errorf("expected 'OK' prefix, got: %s", got)
	}
	if !strings.Contains(got, `Workflow "my-wf" created`) {
		t.Errorf("expected formatted message, got: %s", got)
	}
	if !strings.HasSuffix(got, "\n") {
		t.Error("expected trailing newline")
	}
}

func TestFprintSuccess_WithColor(t *testing.T) {
	ResetColorCache()
	os.Unsetenv("NO_COLOR")
	t.Setenv("FORCE_COLOR", "1")
	defer ResetColorCache()

	var buf bytes.Buffer
	FprintSuccess(&buf, "created resource")

	got := buf.String()
	// Should contain ANSI green escape for "OK"
	if !strings.Contains(got, Green) {
		t.Errorf("expected green ANSI code in output, got: %s", got)
	}
	if !strings.Contains(got, "OK") {
		t.Errorf("expected 'OK' prefix, got: %s", got)
	}
	if !strings.Contains(got, "created resource") {
		t.Errorf("expected message text, got: %s", got)
	}
}

func TestFprintWarning_NoColor(t *testing.T) {
	ResetColorCache()
	t.Setenv("NO_COLOR", "1")
	defer ResetColorCache()

	var buf bytes.Buffer
	FprintWarning(&buf, "resource %s is deprecated", "old-res")

	got := buf.String()
	if !strings.Contains(got, "Warning:") {
		t.Errorf("expected 'Warning:' prefix, got: %s", got)
	}
	if !strings.Contains(got, `resource old-res is deprecated`) {
		t.Errorf("expected formatted message, got: %s", got)
	}
	if !strings.HasSuffix(got, "\n") {
		t.Error("expected trailing newline")
	}
}

func TestFprintWarning_WithColor(t *testing.T) {
	ResetColorCache()
	os.Unsetenv("NO_COLOR")
	t.Setenv("FORCE_COLOR", "1")
	defer ResetColorCache()

	var buf bytes.Buffer
	FprintWarning(&buf, "dry-run mode")

	got := buf.String()
	// Should contain ANSI yellow escape for "Warning:"
	if !strings.Contains(got, Yellow) {
		t.Errorf("expected yellow ANSI code in output, got: %s", got)
	}
	if !strings.Contains(got, "Warning:") {
		t.Errorf("expected 'Warning:' prefix, got: %s", got)
	}
	if !strings.Contains(got, "dry-run mode") {
		t.Errorf("expected message text, got: %s", got)
	}
}

func TestFprintSuccess_FormatArgs(t *testing.T) {
	ResetColorCache()
	t.Setenv("NO_COLOR", "1")
	defer ResetColorCache()

	var buf bytes.Buffer
	FprintSuccess(&buf, "%d resources created in %s", 5, "production")

	got := buf.String()
	if !strings.Contains(got, "5 resources created in production") {
		t.Errorf("expected formatted message with multiple args, got: %s", got)
	}
}

func TestFprintWarning_FormatArgs(t *testing.T) {
	ResetColorCache()
	t.Setenv("NO_COLOR", "1")
	defer ResetColorCache()

	var buf bytes.Buffer
	FprintWarning(&buf, "skipping %d of %d items", 3, 10)

	got := buf.String()
	if !strings.Contains(got, "skipping 3 of 10 items") {
		t.Errorf("expected formatted message with multiple args, got: %s", got)
	}
}

func TestFprintInfo(t *testing.T) {
	var buf bytes.Buffer
	FprintInfo(&buf, "  ID:   %s", "abc-123")

	got := buf.String()
	expected := "  ID:   abc-123\n"
	if got != expected {
		t.Errorf("FprintInfo output = %q, want %q", got, expected)
	}
}

func TestFprintInfo_NoFormatArgs(t *testing.T) {
	var buf bytes.Buffer
	FprintInfo(&buf, "Note: Bucket creation can take up to 1 minute")

	got := buf.String()
	expected := "Note: Bucket creation can take up to 1 minute\n"
	if got != expected {
		t.Errorf("FprintInfo output = %q, want %q", got, expected)
	}
}

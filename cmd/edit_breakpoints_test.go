package cmd

import (
	"testing"

	"github.com/spf13/cobra"
)

func TestEditBreakpointCommandRegistration(t *testing.T) {
	editCmd, _, err := rootCmd.Find([]string{"edit"})
	if err != nil {
		t.Fatalf("expected edit command to exist, got error: %v", err)
	}
	if editCmd == nil || editCmd.Name() != "edit" {
		t.Fatalf("expected edit command to exist")
	}

	breakpointCmd, _, err := rootCmd.Find([]string{"edit", "breakpoint"})
	if err != nil {
		t.Fatalf("expected edit breakpoint command to exist, got error: %v", err)
	}
	if breakpointCmd == nil || breakpointCmd.Name() != "breakpoint" {
		t.Fatalf("expected edit breakpoint command to exist")
	}
}

func TestBuildEditBreakpointSettings(t *testing.T) {
	rule := map[string]interface{}{
		"id": "dtctl-rule-1",
		"aug_json": map[string]interface{}{
			"action": map[string]interface{}{
				"operations": []interface{}{
					map[string]interface{}{
						"name": "set",
						"paths": map[string]interface{}{
							"store.rookout.frame":                    "frame.dump()",
							"store.rookout.traceback":                "stack.traceback()",
							"store.rookout.tracing":                  "trace.dump()",
							"store.rookout.processMonitoring":        "state.dump()",
							"store.rookout.variables.customerId":     "frame.customerId",
							"store.rookout.variables.MyClass.thread": "utils.class(\"com.example.MyClass\").thread",
						},
					},
				},
			},
			"conditional":            nil,
			"globalDisableAfterTime": "2026-03-17T08:25:11Z",
			"globalHitLimit":         float64(100),
			"location": map[string]interface{}{
				"filename": "OrderController.java",
				"lineno":   float64(306),
			},
			"rateLimit": "150/20000",
		},
		"processing": map[string]interface{}{
			"operations": []interface{}{
				map[string]interface{}{"name": "set", "paths": map[string]interface{}{"temp.message.rookout": "store.rookout"}},
				map[string]interface{}{"name": "format", "path": "temp.message.rookout.message", "format": "Hit on {store.rookout.frame.filename}:{store.rookout.frame.line}"},
				map[string]interface{}{"name": "send_rookout", "path": "temp.message"},
			},
		},
	}

	settings, err := buildEditBreakpointSettings(rule, "value>othervalue", true)
	if err != nil {
		t.Fatalf("buildEditBreakpointSettings returned error: %v", err)
	}

	if settings["mutableRuleId"] != "dtctl-rule-1" {
		t.Fatalf("unexpected mutableRuleId: %#v", settings["mutableRuleId"])
	}
	if settings["condition"] != "value>othervalue" {
		t.Fatalf("unexpected condition: %#v", settings["condition"])
	}
	if settings["outputMessage"] != breakpointDefaultOutputMessage {
		t.Fatalf("unexpected outputMessage: %#v", settings["outputMessage"])
	}
	if settings["collectLocalsMethod"] != "frame.dump()" {
		t.Fatalf("unexpected collectLocalsMethod: %#v", settings["collectLocalsMethod"])
	}
	if settings["stackTraceCollection"] != true {
		t.Fatalf("unexpected stackTraceCollection: %#v", settings["stackTraceCollection"])
	}
	if settings["tracingCollection"] != true {
		t.Fatalf("unexpected tracingCollection: %#v", settings["tracingCollection"])
	}
	if settings["processMonitoringCollection"] != true {
		t.Fatalf("unexpected processMonitoringCollection: %#v", settings["processMonitoringCollection"])
	}

	collectedVariables, ok := settings["collectedVariables"].([]string)
	if !ok {
		t.Fatalf("unexpected collectedVariables type: %#v", settings["collectedVariables"])
	}
	if len(collectedVariables) != 2 {
		t.Fatalf("unexpected collectedVariables length: %#v", collectedVariables)
	}
	if collectedVariables[0] != "com.example.MyClass.thread" || collectedVariables[1] != "customerId" {
		t.Fatalf("unexpected collectedVariables: %#v", collectedVariables)
	}

	targetConfiguration, ok := settings["targetConfiguration"].(map[string]interface{})
	if !ok {
		t.Fatalf("unexpected targetConfiguration: %#v", settings["targetConfiguration"])
	}
	if targetConfiguration["targetId"] != breakpointRookoutTargetID {
		t.Fatalf("unexpected targetId: %#v", targetConfiguration["targetId"])
	}
}

func TestResolveBreakpointRulesForEdit(t *testing.T) {
	rules := []map[string]interface{}{
		{
			"id": "bp-1",
			"aug_json": map[string]interface{}{
				"location": map[string]interface{}{"filename": "A.java", "lineno": float64(10)},
			},
		},
		{
			"id": "bp-2",
			"aug_json": map[string]interface{}{
				"location": map[string]interface{}{"filename": "A.java", "lineno": float64(10)},
			},
		},
	}

	matches, description, allowDirectID, err := resolveBreakpointRulesForEdit(rules, "A.java:10")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(matches) != 2 || description != "A.java:10" || allowDirectID {
		t.Fatalf("unexpected location resolution result: len=%d desc=%q direct=%t", len(matches), description, allowDirectID)
	}

	matches, description, allowDirectID, err = resolveBreakpointRulesForEdit(rules, "bp-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(matches) != 1 || description != "bp-1" || allowDirectID {
		t.Fatalf("unexpected id resolution result: len=%d desc=%q direct=%t", len(matches), description, allowDirectID)
	}

	matches, description, allowDirectID, err = resolveBreakpointRulesForEdit(rules, "missing")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(matches) != 0 || description != "missing" || !allowDirectID {
		t.Fatalf("unexpected fallback resolution result: len=%d desc=%q direct=%t", len(matches), description, allowDirectID)
	}
}

func TestGetOptionalBoolFlag(t *testing.T) {
	cmd := &cobra.Command{Use: "test"}
	cmd.Flags().String("enabled", "", "")
	cmd.Flags().Lookup("enabled").NoOptDefVal = "true"

	if err := cmd.Flags().Parse([]string{"--enabled", "true"}); err != nil {
		t.Fatalf("unexpected parse error for explicit true: %v", err)
	}

	enabled, changed, err := getOptionalBoolFlag(cmd, "enabled", nil)
	if err != nil {
		t.Fatalf("unexpected getOptionalBoolFlag error: %v", err)
	}
	if !changed || !enabled {
		t.Fatalf("expected enabled=true changed=true, got enabled=%t changed=%t", enabled, changed)
	}

	cmd = &cobra.Command{Use: "test"}
	cmd.Flags().String("enabled", "", "")
	cmd.Flags().Lookup("enabled").NoOptDefVal = "true"
	if err := cmd.Flags().Parse([]string{"--enabled", "false"}); err != nil {
		t.Fatalf("unexpected parse error for explicit false: %v", err)
	}

	enabled, changed, err = getOptionalBoolFlag(cmd, "enabled", []string{"false"})
	if err != nil {
		t.Fatalf("unexpected getOptionalBoolFlag error: %v", err)
	}
	if !changed || enabled {
		t.Fatalf("expected enabled=false changed=true, got enabled=%t changed=%t", enabled, changed)
	}

	cmd = &cobra.Command{Use: "test"}
	cmd.Flags().String("enabled", "", "")
	cmd.Flags().Lookup("enabled").NoOptDefVal = "true"
	if err := cmd.Flags().Parse([]string{"--enabled"}); err != nil {
		t.Fatalf("unexpected parse error for implicit true: %v", err)
	}

	enabled, changed, err = getOptionalBoolFlag(cmd, "enabled", nil)
	if err != nil {
		t.Fatalf("unexpected getOptionalBoolFlag error: %v", err)
	}
	if !changed || !enabled {
		t.Fatalf("expected enabled=true changed=true, got enabled=%t changed=%t", enabled, changed)
	}
}

package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/dynatrace-oss/dtctl/pkg/output"
	"github.com/dynatrace-oss/dtctl/pkg/prompt"
	"github.com/dynatrace-oss/dtctl/pkg/resources/anomalydetector"
	"github.com/dynatrace-oss/dtctl/pkg/safety"
)

// getAnomalyDetectorsCmd retrieves anomaly detectors
var getAnomalyDetectorsCmd = &cobra.Command{
	Use:     "anomaly-detectors [id]",
	Aliases: []string{"anomaly-detector", "ad"},
	Short:   "Get custom anomaly detectors",
	Long: `Get custom anomaly detectors (builtin:davis.anomaly-detectors).

Examples:
  # List all anomaly detectors
  dtctl get anomaly-detectors

  # List only enabled detectors
  dtctl get anomaly-detectors --enabled

  # List only disabled detectors
  dtctl get anomaly-detectors --enabled=false

  # Get a specific detector by object ID
  dtctl get anomaly-detector <object-id>

  # Output as JSON
  dtctl get anomaly-detectors -o json

  # Wide output with object IDs
  dtctl get anomaly-detectors -o wide
`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := LoadConfig()
		if err != nil {
			return err
		}

		c, err := NewClientFromConfig(cfg)
		if err != nil {
			return err
		}

		handler := anomalydetector.NewHandler(c)
		printer := NewPrinter()

		// Get specific detector if ID provided
		if len(args) > 0 {
			ad, err := resolveAnomalyDetector(handler, args[0])
			if err != nil {
				return err
			}

			enrichAgent(printer, "get", "anomaly-detector")
			return printer.Print(ad)
		}

		// List all detectors
		opts := anomalydetector.ListOptions{}

		// Handle tri-state --enabled flag
		if cmd.Flags().Changed("enabled") {
			enabled, _ := cmd.Flags().GetBool("enabled")
			opts.Enabled = &enabled
		}

		detectors, err := handler.List(opts)
		if err != nil {
			return err
		}

		ap := enrichAgent(printer, "get", "anomaly-detector")
		if ap != nil {
			ap.Context().Suggestions = []string{
				"dtctl describe anomaly-detector <title> -- view full configuration and recent problems",
				"dtctl get anomaly-detectors --enabled -- list only active detectors",
				"dtctl edit anomaly-detector <title> -- modify detector configuration",
			}
		}
		return printer.PrintList(detectors)
	},
}

// deleteAnomalyDetectorCmd deletes an anomaly detector
var deleteAnomalyDetectorCmd = &cobra.Command{
	Use:     "anomaly-detector <id-or-title>",
	Aliases: []string{"ad"},
	Short:   "Delete a custom anomaly detector",
	Long: `Delete a custom anomaly detector by object ID or title.

Examples:
  # Delete by object ID
  dtctl delete anomaly-detector <object-id>

  # Delete by title
  dtctl delete anomaly-detector "High CPU on production hosts"

  # Delete without confirmation
  dtctl delete anomaly-detector <object-id> -y
`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		identifier := args[0]

		cfg, err := LoadConfig()
		if err != nil {
			return err
		}

		// Safety check
		checker, err := NewSafetyChecker(cfg)
		if err != nil {
			return err
		}
		if err := checker.CheckError(safety.OperationDelete, safety.OwnershipUnknown); err != nil {
			return err
		}

		c, err := NewClientFromConfig(cfg)
		if err != nil {
			return err
		}

		handler := anomalydetector.NewHandler(c)

		// Resolve identifier (could be objectID or title)
		ad, err := resolveAnomalyDetector(handler, identifier)
		if err != nil {
			return err
		}

		// Confirm deletion unless --yes or --plain
		if !forceDelete && !plainMode {
			if !prompt.ConfirmDeletion("anomaly detector", ad.Title, ad.ObjectID) {
				fmt.Println("Deletion cancelled")
				return nil
			}
		}

		if err := handler.Delete(ad.ObjectID); err != nil {
			return err
		}

		output.PrintSuccess("Anomaly detector %q deleted", ad.Title)
		return nil
	},
}

func init() {
	// --enabled flag: tri-state (absent=all, --enabled=true, --enabled=false)
	getAnomalyDetectorsCmd.Flags().Bool("enabled", true, "Filter by enabled state (--enabled for enabled only, --enabled=false for disabled only)")

	// Delete confirmation flags
	deleteAnomalyDetectorCmd.Flags().BoolVarP(&forceDelete, "yes", "y", false, "Skip confirmation prompt")

	// Suppress the default value in help output for the tri-state flag
	_ = getAnomalyDetectorsCmd.Flags().SetAnnotation("enabled", "cobra_annotation_bash_completion_custom", []string{})
}

// resolveAnomalyDetector tries to find a detector by ID or title, used by describe/edit/delete commands.
func resolveAnomalyDetector(handler *anomalydetector.Handler, identifier string) (*anomalydetector.AnomalyDetector, error) {
	// Try by object ID first
	ad, err := handler.Get(identifier)
	if err == nil {
		return ad, nil
	}

	// Fall back to title match
	ad, err = handler.FindByName(identifier)
	if err == nil {
		return ad, nil
	}

	return nil, fmt.Errorf("anomaly detector %q not found", identifier)
}

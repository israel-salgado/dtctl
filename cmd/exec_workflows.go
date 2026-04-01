package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/dynatrace-oss/dtctl/pkg/client"
	"github.com/dynatrace-oss/dtctl/pkg/exec"
	"github.com/dynatrace-oss/dtctl/pkg/output"
	workflowpkg "github.com/dynatrace-oss/dtctl/pkg/resources/workflow"
)

// execWorkflowResult is the structured response for agent mode.
type execWorkflowResult struct {
	ExecutionID string                   `json:"executionId"`
	WorkflowID  string                   `json:"workflowId"`
	State       string                   `json:"state"`
	StateInfo   *string                  `json:"stateInfo,omitempty"`
	Duration    string                   `json:"duration,omitempty"`
	Tasks       []execWorkflowTaskResult `json:"tasks,omitempty"`
}

// execWorkflowTaskResult is a per-task entry in the agent response.
type execWorkflowTaskResult struct {
	Name   string `json:"name"`
	State  string `json:"state"`
	Result any    `json:"result,omitempty"`
}

// execWorkflowCmd executes a workflow
var execWorkflowCmd = &cobra.Command{
	Use:     "workflow <workflow-id>",
	Aliases: []string{"wf"},
	Short:   "Execute a workflow",
	Long:    `Execute an automation workflow.`,
	Example: `  # Execute workflow
  dtctl exec workflow my-workflow-id

  # Execute with parameters
  dtctl exec workflow my-workflow-id --params severity=high --params env=prod

  # Execute and wait for completion
  dtctl exec workflow my-workflow-id --wait

  # Execute with custom timeout
  dtctl exec workflow my-workflow-id --wait --timeout 10m

  # Execute, wait, and print each task's return value when done
  dtctl exec workflow my-workflow-id --wait --show-results`,
	Args: cobra.ExactArgs(1),
	PreRunE: func(cmd *cobra.Command, args []string) error {
		showResults, _ := cmd.Flags().GetBool("show-results")
		wait, _ := cmd.Flags().GetBool("wait")
		if showResults && !wait {
			return fmt.Errorf("--show-results requires --wait")
		}
		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		workflowID := args[0]

		_, c, err := SetupClient()
		if err != nil {
			return err
		}

		executor := exec.NewWorkflowExecutor(c)

		paramStrings, _ := cmd.Flags().GetStringSlice("params")
		params, err := exec.ParseParams(paramStrings)
		if err != nil {
			return err
		}

		result, err := executor.Execute(workflowID, params)
		if err != nil {
			return err
		}

		// Agent mode: collect everything into a structured envelope
		printer := NewPrinter()
		ap := enrichAgent(printer, "exec", "workflow")
		if ap != nil {
			return execWorkflowAgent(cmd, c, executor, result, ap)
		}

		// Human mode: interactive output
		fmt.Printf("Workflow execution started\n")
		fmt.Printf("Execution ID: %s\n", result.ID)
		fmt.Printf("State: %s\n", result.State)

		// Handle --wait flag
		wait, _ := cmd.Flags().GetBool("wait")
		if wait {
			fmt.Printf("\nWaiting for execution to complete...\n")

			status, err := execWorkflowWait(cmd, executor, result.ID)
			if err != nil {
				return err
			}

			fmt.Printf("\nExecution completed\n")
			fmt.Printf("Final State: %s\n", status.State)
			if status.StateInfo != nil && *status.StateInfo != "" {
				fmt.Printf("State Info: %s\n", *status.StateInfo)
			}
			fmt.Printf("Duration: %s\n", formatExecutionDuration(status.Runtime))

			// Print task results if --show-results is set
			showResults, _ := cmd.Flags().GetBool("show-results")
			if showResults {
				if err := execWorkflowShowResults(c, result.ID, printer); err != nil {
					return err
				}
			}

			// Return error if execution failed
			if status.State == "ERROR" {
				return fmt.Errorf("workflow execution failed")
			}
		}

		return nil
	},
}

// execWorkflowWait handles the --wait polling loop and returns the final status.
func execWorkflowWait(cmd *cobra.Command, executor *exec.WorkflowExecutor, executionID string) (*exec.ExecutionStatus, error) {
	timeout, _ := cmd.Flags().GetDuration("timeout")
	if timeout == 0 {
		timeout = 30 * time.Minute
	}

	opts := exec.WaitOptions{
		PollInterval: 2 * time.Second,
		Timeout:      timeout,
	}

	return executor.WaitForCompletion(context.Background(), executionID, opts)
}

// execWorkflowShowResults prints per-task results in human-readable format.
func execWorkflowShowResults(c *client.Client, executionID string, printer output.Printer) error {
	execHandler := workflowpkg.NewExecutionHandler(c)
	tasks, err := execHandler.ListTasks(executionID)
	if err != nil {
		return fmt.Errorf("failed to list tasks: %w", err)
	}
	if len(tasks) > 0 {
		fmt.Printf("\nTask Results:\n")
		for _, task := range tasks {
			fmt.Printf("\n--- %s [%s] ---\n", task.Name, task.State)
			if task.Result == nil {
				fmt.Printf("(no structured return value)\n")
				continue
			}
			if err := printer.Print(task.Result); err != nil {
				fmt.Printf("(failed to print result: %v)\n", err)
			}
		}
	}
	return nil
}

// execWorkflowAgent handles the entire exec workflow command in agent mode,
// producing a single structured JSON envelope.
func execWorkflowAgent(
	cmd *cobra.Command,
	c *client.Client,
	executor *exec.WorkflowExecutor,
	result *exec.WorkflowExecutionResponse,
	ap *output.AgentPrinter,
) error {
	resp := execWorkflowResult{
		ExecutionID: result.ID,
		WorkflowID:  result.Workflow,
		State:       result.State,
	}

	wait, _ := cmd.Flags().GetBool("wait")
	if wait {
		status, err := execWorkflowWait(cmd, executor, result.ID)
		if err != nil {
			return err
		}
		resp.State = status.State
		resp.StateInfo = status.StateInfo
		resp.Duration = formatExecutionDuration(status.Runtime)
		ap.SetDuration(resp.Duration)

		showResults, _ := cmd.Flags().GetBool("show-results")
		if showResults {
			execHandler := workflowpkg.NewExecutionHandler(c)
			tasks, err := execHandler.ListTasks(result.ID)
			if err != nil {
				return fmt.Errorf("failed to list tasks: %w", err)
			}
			for _, task := range tasks {
				resp.Tasks = append(resp.Tasks, execWorkflowTaskResult{
					Name:   task.Name,
					State:  task.State,
					Result: task.Result,
				})
			}
		}

		if status.State == "ERROR" {
			ap.SetWarnings([]string{"workflow execution failed"})
		}
	}

	ap.SetSuggestions([]string{
		fmt.Sprintf("Run 'dtctl get wfe %s' to view the execution details", result.ID),
		fmt.Sprintf("Run 'dtctl logs wfe %s' to view execution logs", result.ID),
	})

	return ap.Print(resp)
}

// formatExecutionDuration formats seconds into a human-readable duration
func formatExecutionDuration(seconds int) string {
	if seconds < 60 {
		return fmt.Sprintf("%ds", seconds)
	}
	if seconds < 3600 {
		m := seconds / 60
		s := seconds % 60
		if s == 0 {
			return fmt.Sprintf("%dm", m)
		}
		return fmt.Sprintf("%dm%ds", m, s)
	}
	h := seconds / 3600
	m := (seconds % 3600) / 60
	if m == 0 {
		return fmt.Sprintf("%dh", h)
	}
	return fmt.Sprintf("%dh%dm", h, m)
}

func init() {
	// Workflow flags
	execWorkflowCmd.Flags().StringSlice("params", []string{}, "workflow parameters (key=value)")
	execWorkflowCmd.Flags().Bool("wait", false, "wait for workflow execution to complete")
	execWorkflowCmd.Flags().Duration("timeout", 30*time.Minute, "timeout when waiting for completion")
	execWorkflowCmd.Flags().Bool("show-results", false, "print the result of each task after execution completes (requires --wait)")
}

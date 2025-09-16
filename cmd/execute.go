package cmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/hacker65536/aft-pipeline-tool/internal/aws"
	"github.com/hacker65536/aft-pipeline-tool/internal/cache"
	"github.com/hacker65536/aft-pipeline-tool/internal/config"
	"github.com/hacker65536/aft-pipeline-tool/internal/models"
	"github.com/hacker65536/aft-pipeline-tool/internal/utils"
	"github.com/hacker65536/aft-pipeline-tool/pkg/aft"
)

var (
	executeCmd = &cobra.Command{
		Use:   "execute",
		Short: "List AFT pipelines with execution information (default) or execute pipeline operations",
		Long: `List all AFT pipelines across accounts and display their current execution status and configurations by default.
Also provides subcommands for starting, stopping, and monitoring pipeline executions.`,
		RunE: runExecuteDefault,
	}

	startCmd = &cobra.Command{
		Use:   "start [pipeline-name]",
		Short: "Start pipeline execution",
		Long:  `Start execution of one or more AFT pipelines.`,
		Args:  cobra.MaximumNArgs(1),
		RunE:  runStartPipeline,
	}

	startFromFileCmd = &cobra.Command{
		Use:   "start-from-file <file-path>",
		Short: "Start pipeline executions from file",
		Long:  `Start execution of pipelines listed in a file. The file should contain one pipeline name per line.`,
		Args:  cobra.ExactArgs(1),
		RunE:  runStartPipelineFromFile,
	}

	stopCmd = &cobra.Command{
		Use:   "stop [pipeline-name] [execution-id]",
		Short: "Stop pipeline execution",
		Long:  `Stop a running pipeline execution.`,
		Args:  cobra.RangeArgs(1, 2),
		RunE:  runStopPipeline,
	}

	historyCmd = &cobra.Command{
		Use:   "history [pipeline-name]",
		Short: "Show pipeline execution history",
		Long:  `Show the execution history of pipelines.`,
		Args:  cobra.MaximumNArgs(1),
		RunE:  runPipelineHistory,
	}
)

var (
	executeAll        bool
	executeWait       bool
	executeTimeout    int
	stopReason        string
	maxResults        int32
	executionId       string
	showDetails       bool
	forceExecute      bool
	executeSequential bool
	// Execute list command variables
	executeListFormat        string
	executeListAccountFilter string
	executeListShowDetails   bool
)

func init() {
	rootCmd.AddCommand(executeCmd)
	executeCmd.AddCommand(startCmd)
	executeCmd.AddCommand(startFromFileCmd)
	executeCmd.AddCommand(stopCmd)
	executeCmd.AddCommand(historyCmd)

	// Start command flags
	startCmd.Flags().BoolVar(&executeAll, "all", false, "Start execution for all AFT pipelines")
	startCmd.Flags().BoolVar(&executeWait, "wait", false, "Wait for execution to complete")
	startCmd.Flags().IntVar(&executeTimeout, "timeout", 3600, "Timeout in seconds when waiting for execution (default: 3600)")
	startCmd.Flags().BoolVar(&forceExecute, "force", false, "Force execution even if pipeline is already running")

	// Start from file command flags
	startFromFileCmd.Flags().BoolVar(&executeWait, "wait", false, "Wait for execution to complete")
	startFromFileCmd.Flags().IntVar(&executeTimeout, "timeout", 3600, "Timeout in seconds when waiting for execution (default: 3600)")
	startFromFileCmd.Flags().BoolVar(&forceExecute, "force", false, "Force execution even if pipeline is already running")
	startFromFileCmd.Flags().BoolVar(&executeSequential, "sequential", false, "Execute pipelines sequentially (wait for completion before next)")

	// Stop command flags
	stopCmd.Flags().StringVar(&stopReason, "reason", "", "Reason for stopping the execution")
	stopCmd.Flags().StringVar(&executionId, "execution-id", "", "Specific execution ID to stop (if not provided, stops the latest running execution)")

	// History command flags
	historyCmd.Flags().Int32Var(&maxResults, "max-results", 10, "Maximum number of executions to show (default: 10)")
	historyCmd.Flags().BoolVar(&showDetails, "details", false, "Show detailed execution information")

	// Execute command flags (for default list behavior)
	executeCmd.Flags().StringVarP(&executeListFormat, "format", "f", "table", "Output format (table, json, csv)")
	executeCmd.Flags().StringVar(&executeListAccountFilter, "account-filter", "", "Filter by account ID pattern")
	executeCmd.Flags().BoolVar(&executeListShowDetails, "show-details", false, "Show detailed information (Account ID, Pipeline Type, Trigger, Last Updated)")
}

func runStartPipeline(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Load configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Create AWS client
	awsClient, err := aws.NewClient(ctx, viper.GetString("aws.region"), viper.GetString("aws.profile"))
	if err != nil {
		return fmt.Errorf("failed to create AWS client: %w", err)
	}

	// Initialize cache and AFT manager
	fileCache := cache.NewFileCache(cfg.Cache.Directory)
	manager := aft.NewManager(awsClient, fileCache, cfg)

	var pipelinesToExecute []models.Pipeline

	if executeAll {
		// Get all AFT pipelines
		pipelines, err := manager.GetPipelines(ctx)
		if err != nil {
			return fmt.Errorf("failed to get pipelines: %w", err)
		}
		pipelinesToExecute = pipelines
	} else if len(args) > 0 {
		// Get specific pipeline
		pipelineName := args[0]
		pipeline, err := awsClient.GetPipelineDetails(ctx, pipelineName)
		if err != nil {
			return fmt.Errorf("failed to get pipeline details: %w", err)
		}
		pipelinesToExecute = []models.Pipeline{*pipeline}
	} else {
		return fmt.Errorf("please specify a pipeline name or use --all flag")
	}

	if len(pipelinesToExecute) == 0 {
		fmt.Println(utils.Warning("No pipelines found to execute"))
		return nil
	}

	fmt.Printf("%s Starting execution for %d pipeline(s)...\n", utils.Info("INFO"), len(pipelinesToExecute))

	var executionResults []ExecutionResult
	for _, pipeline := range pipelinesToExecute {
		result := ExecutionResult{
			PipelineName: pipeline.GetName(),
		}

		fmt.Printf("Checking execution status for pipeline: %s\n", utils.Highlight(pipeline.GetName()))

		// Check if pipeline is already running (unless force flag is set)
		if !forceExecute {
			isRunning, runningExecutionId, err := isPipelineRunning(ctx, awsClient, pipeline.GetName())
			if err != nil {
				result.Error = fmt.Errorf("failed to check pipeline status: %w", err)
				fmt.Printf("  %s Failed to check pipeline status: %v\n", utils.Error("ERROR"), err)
				executionResults = append(executionResults, result)
				continue
			}

			if isRunning {
				result.Error = fmt.Errorf("pipeline is already running (execution ID: %s). Use --force to override", runningExecutionId)
				fmt.Printf("  %s Pipeline is already running (execution ID: %s). Skipping...\n", utils.Warning("SKIPPED"), runningExecutionId)
				executionResults = append(executionResults, result)
				continue
			}
		}

		fmt.Printf("Starting execution for pipeline: %s\n", utils.Highlight(pipeline.GetName()))

		output, err := awsClient.StartPipelineExecution(ctx, pipeline.GetName())
		if err != nil {
			result.Error = err
			fmt.Printf("  %s Failed to start execution: %v\n", utils.Error("ERROR"), err)
		} else {
			result.ExecutionId = *output.PipelineExecutionId
			fmt.Printf("  %s Execution started with ID: %s\n", utils.Success("SUCCESS"), *output.PipelineExecutionId)
		}

		executionResults = append(executionResults, result)
	}

	// Wait for executions to complete if requested
	if executeWait {
		fmt.Printf("\n%s Waiting for executions to complete (timeout: %d seconds)...\n", utils.Info("INFO"), executeTimeout)
		err := waitForExecutions(ctx, awsClient, executionResults, executeTimeout)
		if err != nil {
			return fmt.Errorf("error while waiting for executions: %w", err)
		}
	}

	// Print summary
	printExecutionSummary(executionResults)

	return nil
}

func runStopPipeline(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	pipelineName := args[0]
	var targetExecutionId string

	if len(args) > 1 {
		targetExecutionId = args[1]
	} else if executionId != "" {
		targetExecutionId = executionId
	}

	// Create AWS client
	awsClient, err := aws.NewClient(ctx, viper.GetString("aws.region"), viper.GetString("aws.profile"))
	if err != nil {
		return fmt.Errorf("failed to create AWS client: %w", err)
	}

	// If no execution ID provided, find the latest running execution
	if targetExecutionId == "" {
		executions, err := awsClient.ListPipelineExecutions(ctx, pipelineName, 10)
		if err != nil {
			return fmt.Errorf("failed to list pipeline executions: %w", err)
		}

		for _, exec := range executions.PipelineExecutionSummaries {
			if string(exec.Status) == "InProgress" {
				targetExecutionId = *exec.PipelineExecutionId
				break
			}
		}

		if targetExecutionId == "" {
			return fmt.Errorf("no running execution found for pipeline %s", pipelineName)
		}
	}

	fmt.Printf("Stopping execution %s for pipeline %s...\n", utils.Highlight(targetExecutionId), utils.Highlight(pipelineName))

	_, err = awsClient.StopPipelineExecution(ctx, pipelineName, targetExecutionId, stopReason)
	if err != nil {
		return fmt.Errorf("failed to stop pipeline execution: %w", err)
	}

	fmt.Printf("%s Pipeline execution stopped successfully\n", utils.Success("SUCCESS"))

	return nil
}

func runPipelineHistory(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Create AWS client
	awsClient, err := aws.NewClient(ctx, viper.GetString("aws.region"), viper.GetString("aws.profile"))
	if err != nil {
		return fmt.Errorf("failed to create AWS client: %w", err)
	}

	if len(args) > 0 {
		// Show history for specific pipeline
		pipelineName := args[0]
		return showPipelineHistory(ctx, awsClient, pipelineName, maxResults, showDetails)
	} else {
		// Show history for all AFT pipelines
		cfg, err := config.LoadConfig()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		fileCache := cache.NewFileCache(cfg.Cache.Directory)
		manager := aft.NewManager(awsClient, fileCache, cfg)

		pipelines, err := manager.GetPipelines(ctx)
		if err != nil {
			return fmt.Errorf("failed to get pipelines: %w", err)
		}

		for _, pipeline := range pipelines {
			fmt.Printf("\n%s\n", utils.Highlight(pipeline.GetName()))
			err := showPipelineHistory(ctx, awsClient, pipeline.GetName(), maxResults, showDetails)
			if err != nil {
				fmt.Printf("  %s Failed to get history: %v\n", utils.Error("ERROR"), err)
			}
		}
	}

	return nil
}

// ExecutionResult represents the result of a pipeline execution start
type ExecutionResult struct {
	PipelineName string
	ExecutionId  string
	Error        error
	Status       string
	StartTime    *time.Time
	EndTime      *time.Time
}

func waitForExecutions(ctx context.Context, client *aws.Client, results []ExecutionResult, timeoutSeconds int) error {
	var deadline time.Time
	hasTimeout := timeoutSeconds > 0
	if hasTimeout {
		timeout := time.Duration(timeoutSeconds) * time.Second
		deadline = time.Now().Add(timeout)
	}

	ticker := time.NewTicker(30 * time.Second) // Check every 30 seconds
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			allCompleted := true
			for i, result := range results {
				if result.Error != nil || result.Status == "Succeeded" || result.Status == "Failed" || result.Status == "Stopped" {
					continue
				}

				execution, err := client.GetPipelineExecution(ctx, result.PipelineName, result.ExecutionId)
				if err != nil {
					fmt.Printf("Failed to get execution status for %s: %v\n", result.PipelineName, err)
					continue
				}

				results[i].Status = string(execution.PipelineExecution.Status)

				status := string(execution.PipelineExecution.Status)
				if status != "Succeeded" && status != "Failed" && status != "Stopped" {
					allCompleted = false
				}

				fmt.Printf("Pipeline %s: %s\n", utils.Highlight(result.PipelineName), formatExecutionStatus(status))
			}

			if allCompleted {
				fmt.Printf("\n%s All executions completed\n", utils.Success("SUCCESS"))
				return nil
			}

			if hasTimeout && time.Now().After(deadline) {
				fmt.Printf("\n%s Timeout reached, some executions may still be running\n", utils.Warning("WARNING"))
				return nil
			}
		}
	}
}

func showPipelineHistory(ctx context.Context, client *aws.Client, pipelineName string, maxResults int32, detailed bool) error {
	executions, err := client.ListPipelineExecutions(ctx, pipelineName, maxResults)
	if err != nil {
		return fmt.Errorf("failed to list executions: %w", err)
	}

	if len(executions.PipelineExecutionSummaries) == 0 {
		fmt.Printf("  %s No executions found\n", utils.Info("INFO"))
		return nil
	}

	fmt.Printf("  Recent executions:\n")
	for _, exec := range executions.PipelineExecutionSummaries {
		status := formatExecutionStatus(string(exec.Status))
		startTime := "N/A"
		if exec.StartTime != nil {
			startTime = exec.StartTime.Format("2006-01-02 15:04:05")
		}

		fmt.Printf("    %s | %s | %s\n",
			*exec.PipelineExecutionId,
			status,
			startTime)

		if detailed && exec.StatusSummary != nil && *exec.StatusSummary != "" {
			fmt.Printf("      Summary: %s\n", *exec.StatusSummary)
		}
	}

	return nil
}

func formatExecutionStatus(status string) string {
	switch strings.ToLower(status) {
	case "succeeded":
		return utils.Success(status)
	case "failed":
		return utils.Error(status)
	case "inprogress":
		return utils.Info(status)
	case "stopped":
		return utils.Warning(status)
	default:
		return status
	}
}

func runStartPipelineFromFile(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	filePath := args[0]

	// Load configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Read pipeline names from file
	pipelineNames, err := readPipelineNamesFromFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read pipeline names from file: %w", err)
	}

	if len(pipelineNames) == 0 {
		fmt.Println(utils.Warning("No pipeline names found in file"))
		return nil
	}

	// Apply limits from configuration
	maxPipelines := cfg.Execution.MaxPipelines
	if maxPipelines > 0 && len(pipelineNames) > maxPipelines {
		fmt.Printf("%s Limiting pipelines to %d (configured max_pipelines)\n", utils.Warning("WARNING"), maxPipelines)
		pipelineNames = pipelineNames[:maxPipelines]
	}

	fmt.Printf("%s Found %d pipeline(s) to execute from file: %s\n", utils.Info("INFO"), len(pipelineNames), filePath)

	// Create AWS client
	awsClient, err := aws.NewClient(ctx, viper.GetString("aws.region"), viper.GetString("aws.profile"))
	if err != nil {
		return fmt.Errorf("failed to create AWS client: %w", err)
	}

	// Determine execution mode: sequential from flag or config
	sequential := executeSequential || cfg.Execution.Sequential

	var executionResults []ExecutionResult
	if sequential {
		fmt.Printf("%s Executing pipelines sequentially (waiting for completion before next)...\n", utils.Info("INFO"))
		executionResults = executePipelinesSequentially(ctx, awsClient, pipelineNames)
	} else {
		// Execute pipelines with concurrency control
		maxConcurrent := cfg.Execution.MaxConcurrent
		if maxConcurrent <= 0 {
			maxConcurrent = 5 // Default fallback
		}
		fmt.Printf("%s Executing pipelines with concurrency (max: %d)...\n", utils.Info("INFO"), maxConcurrent)
		executionResults = executePipelinesWithConcurrency(ctx, awsClient, pipelineNames, maxConcurrent)
	}

	// Wait for executions to complete if requested
	if executeWait {
		timeout := executeTimeout
		if timeout <= 0 {
			timeout = cfg.Execution.Timeout
		}
		fmt.Printf("\n%s Waiting for executions to complete (timeout: %d seconds)...\n", utils.Info("INFO"), timeout)
		err := waitForExecutions(ctx, awsClient, executionResults, timeout)
		if err != nil {
			return fmt.Errorf("error while waiting for executions: %w", err)
		}
	}

	// Print summary
	printExecutionSummary(executionResults)

	return nil
}

func readPipelineNamesFromFile(filePath string) ([]string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			fmt.Printf("Warning: failed to close file: %v\n", closeErr)
		}
	}()

	var pipelineNames []string
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		// Skip empty lines and comments
		if line != "" && !strings.HasPrefix(line, "#") {
			pipelineNames = append(pipelineNames, line)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading file: %w", err)
	}

	return pipelineNames, nil
}

func executePipelinesSequentially(ctx context.Context, client *aws.Client, pipelineNames []string) []ExecutionResult {
	var results []ExecutionResult

	for i, pipelineName := range pipelineNames {
		fmt.Printf("\n%s Processing pipeline %d/%d: %s\n", utils.Info("INFO"), i+1, len(pipelineNames), utils.Highlight(pipelineName))

		result := ExecutionResult{
			PipelineName: pipelineName,
		}

		// Check if pipeline is already running (unless force flag is set)
		if !forceExecute {
			isRunning, runningExecutionId, err := isPipelineRunning(ctx, client, pipelineName)
			if err != nil {
				result.Error = fmt.Errorf("failed to check pipeline status: %w", err)
				fmt.Printf("  %s Failed to check pipeline status: %v\n", utils.Error("ERROR"), err)
				results = append(results, result)
				continue
			}

			if isRunning {
				result.Error = fmt.Errorf("pipeline is already running (execution ID: %s). Use --force to override", runningExecutionId)
				fmt.Printf("  %s Pipeline is already running (execution ID: %s). Skipping...\n", utils.Warning("SKIPPED"), runningExecutionId)
				results = append(results, result)
				continue
			}
		}

		// Start pipeline execution
		fmt.Printf("Starting execution for pipeline: %s\n", utils.Highlight(pipelineName))
		output, err := client.StartPipelineExecution(ctx, pipelineName)
		if err != nil {
			result.Error = err
			fmt.Printf("  %s Failed to start execution: %v\n", utils.Error("ERROR"), err)
			results = append(results, result)
			continue
		}

		result.ExecutionId = *output.PipelineExecutionId
		fmt.Printf("  %s Execution started with ID: %s\n", utils.Success("SUCCESS"), *output.PipelineExecutionId)

		// Wait for this pipeline to complete before starting the next one
		fmt.Printf("  %s Waiting for pipeline %s to complete...\n", utils.Info("INFO"), pipelineName)

		// Create a single-item slice for waitForExecutions
		singleResult := []ExecutionResult{result}
		err = waitForExecutions(ctx, client, singleResult, 0) // 0 means no timeout for individual pipeline
		if err != nil {
			fmt.Printf("  %s Error while waiting for pipeline %s: %v\n", utils.Warning("WARNING"), pipelineName, err)
		}

		// Update the result with final status
		if len(singleResult) > 0 {
			result.Status = singleResult[0].Status
		}

		results = append(results, result)

		// Show completion status
		switch result.Status {
		case "Succeeded":
			fmt.Printf("  %s Pipeline %s completed successfully\n", utils.Success("COMPLETED"), pipelineName)
		case "Failed":
			fmt.Printf("  %s Pipeline %s failed\n", utils.Error("FAILED"), pipelineName)
		case "Stopped":
			fmt.Printf("  %s Pipeline %s was stopped\n", utils.Warning("STOPPED"), pipelineName)
		default:
			fmt.Printf("  %s Pipeline %s status: %s\n", utils.Info("STATUS"), pipelineName, result.Status)
		}
	}

	return results
}

func executePipelinesWithConcurrency(ctx context.Context, client *aws.Client, pipelineNames []string, maxConcurrent int) []ExecutionResult {
	var allResults []ExecutionResult

	// Process pipelines in batches of maxConcurrent
	for i := 0; i < len(pipelineNames); i += maxConcurrent {
		end := i + maxConcurrent
		if end > len(pipelineNames) {
			end = len(pipelineNames)
		}

		batch := pipelineNames[i:end]
		fmt.Printf("\n%s Processing batch %d/%d (%d pipelines)...\n",
			utils.Info("INFO"),
			(i/maxConcurrent)+1,
			(len(pipelineNames)+maxConcurrent-1)/maxConcurrent,
			len(batch))

		// Start pipelines in current batch concurrently
		batchResults := startPipelineBatch(ctx, client, batch)
		allResults = append(allResults, batchResults...)

		// Wait for all pipelines in current batch to complete before starting next batch
		if i+maxConcurrent < len(pipelineNames) { // Don't wait after the last batch
			fmt.Printf("%s Waiting for current batch to complete before starting next batch...\n", utils.Info("INFO"))
			err := waitForBatchCompletion(ctx, client, batchResults)
			if err != nil {
				fmt.Printf("%s Error while waiting for batch completion: %v\n", utils.Warning("WARNING"), err)
			}
		}
	}

	return allResults
}

// startPipelineBatch starts a batch of pipelines concurrently
func startPipelineBatch(ctx context.Context, client *aws.Client, pipelineNames []string) []ExecutionResult {
	var wg sync.WaitGroup
	var mu sync.Mutex
	var results []ExecutionResult

	for _, pipelineName := range pipelineNames {
		wg.Add(1)
		go func(name string) {
			defer wg.Done()

			result := ExecutionResult{
				PipelineName: name,
			}

			fmt.Printf("Checking execution status for pipeline: %s\n", utils.Highlight(name))

			// Check if pipeline is already running (unless force flag is set)
			if !forceExecute {
				isRunning, runningExecutionId, err := isPipelineRunning(ctx, client, name)
				if err != nil {
					result.Error = fmt.Errorf("failed to check pipeline status: %w", err)
					fmt.Printf("  %s Failed to check pipeline status for %s: %v\n", utils.Error("ERROR"), name, err)
					// Thread-safe append to results
					mu.Lock()
					results = append(results, result)
					mu.Unlock()
					return
				}

				if isRunning {
					result.Error = fmt.Errorf("pipeline is already running (execution ID: %s). Use --force to override", runningExecutionId)
					fmt.Printf("  %s Pipeline %s is already running (execution ID: %s). Skipping...\n", utils.Warning("SKIPPED"), name, runningExecutionId)
					// Thread-safe append to results
					mu.Lock()
					results = append(results, result)
					mu.Unlock()
					return
				}
			}

			fmt.Printf("Starting execution for pipeline: %s\n", utils.Highlight(name))

			output, err := client.StartPipelineExecution(ctx, name)
			if err != nil {
				result.Error = err
				fmt.Printf("  %s Failed to start execution for %s: %v\n", utils.Error("ERROR"), name, err)
			} else {
				result.ExecutionId = *output.PipelineExecutionId
				fmt.Printf("  %s Execution started for %s with ID: %s\n", utils.Success("SUCCESS"), name, *output.PipelineExecutionId)
			}

			// Thread-safe append to results
			mu.Lock()
			results = append(results, result)
			mu.Unlock()
		}(pipelineName)
	}

	wg.Wait()
	return results
}

// waitForBatchCompletion waits for all pipelines in a batch to complete
func waitForBatchCompletion(ctx context.Context, client *aws.Client, batchResults []ExecutionResult) error {
	// Filter out results with errors (failed to start)
	var activeExecutions []ExecutionResult
	for _, result := range batchResults {
		if result.Error == nil && result.ExecutionId != "" {
			activeExecutions = append(activeExecutions, result)
		}
	}

	if len(activeExecutions) == 0 {
		fmt.Printf("  %s No active executions to wait for in this batch\n", utils.Info("INFO"))
		return nil
	}

	fmt.Printf("  %s Monitoring %d active executions...\n", utils.Info("INFO"), len(activeExecutions))

	ticker := time.NewTicker(30 * time.Second) // Check every 30 seconds
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			allCompleted := true
			completedCount := 0

			for i, result := range activeExecutions {
				if result.Status == "Succeeded" || result.Status == "Failed" || result.Status == "Stopped" {
					completedCount++
					continue
				}

				execution, err := client.GetPipelineExecution(ctx, result.PipelineName, result.ExecutionId)
				if err != nil {
					fmt.Printf("    %s Failed to get execution status for %s: %v\n", utils.Warning("WARNING"), result.PipelineName, err)
					continue
				}

				status := string(execution.PipelineExecution.Status)
				activeExecutions[i].Status = status

				if status == "Succeeded" || status == "Failed" || status == "Stopped" {
					completedCount++
					fmt.Printf("    %s Pipeline %s: %s\n",
						getStatusIcon(status),
						utils.Highlight(result.PipelineName),
						formatExecutionStatus(status))
				} else {
					allCompleted = false
				}
			}

			fmt.Printf("  %s Batch progress: %d/%d completed\n", utils.Info("INFO"), completedCount, len(activeExecutions))

			if allCompleted {
				fmt.Printf("  %s All pipelines in batch completed\n", utils.Success("SUCCESS"))
				return nil
			}
		}
	}
}

// getStatusIcon returns an appropriate icon for the execution status
func getStatusIcon(status string) string {
	switch strings.ToLower(status) {
	case "succeeded":
		return utils.Success("✓")
	case "failed":
		return utils.Error("✗")
	case "stopped":
		return utils.Warning("⏹")
	default:
		return utils.Info("●")
	}
}

func printExecutionSummary(results []ExecutionResult) {
	fmt.Printf("\n%s Execution Summary:\n", utils.Info("INFO"))

	succeeded := 0
	failed := 0
	skipped := 0

	for _, result := range results {
		if result.Error != nil {
			if strings.Contains(result.Error.Error(), "already running") {
				skipped++
				fmt.Printf("  %s %s: %v\n", utils.Warning("SKIPPED"), result.PipelineName, result.Error)
			} else {
				failed++
				fmt.Printf("  %s %s: %v\n", utils.Error("FAILED"), result.PipelineName, result.Error)
			}
		} else {
			succeeded++
			status := result.Status
			if status == "" {
				status = "Started"
			}
			fmt.Printf("  %s %s: %s (ID: %s)\n", utils.Success("SUCCESS"), result.PipelineName, status, result.ExecutionId)
		}
	}

	fmt.Printf("\nTotal: %d, Succeeded: %d, Failed: %d, Skipped: %d\n", len(results), succeeded, failed, skipped)
}

// isPipelineRunning checks if a pipeline is currently running
func isPipelineRunning(ctx context.Context, client *aws.Client, pipelineName string) (bool, string, error) {
	// Get the latest executions for the pipeline
	executions, err := client.ListPipelineExecutions(ctx, pipelineName, 5)
	if err != nil {
		return false, "", fmt.Errorf("failed to list pipeline executions: %w", err)
	}

	// Check if any execution is currently in progress
	for _, exec := range executions.PipelineExecutionSummaries {
		if string(exec.Status) == "InProgress" {
			return true, *exec.PipelineExecutionId, nil
		}
	}

	return false, "", nil
}

// runExecuteDefault implements the default behavior for execute command (same as list)
func runExecuteDefault(cmd *cobra.Command, args []string) error {
	// Use the same logic as runExecuteList for the default behavior
	return runExecuteList(cmd, args)
}

// runExecuteList implements the execute list command with state information always included
func runExecuteList(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// 設定読み込み
	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// AWS クライアント初期化
	awsClient, err := aws.NewClient(ctx, cfg.AWS.Region, cfg.AWS.Profile)
	if err != nil {
		return fmt.Errorf("failed to create AWS client: %w", err)
	}

	// キャッシュ初期化
	fileCache := cache.NewFileCache(cfg.Cache.Directory)

	// AFT マネージャー初期化
	manager := aft.NewManager(awsClient, fileCache, cfg)

	// パイプライン詳細取得（state情報を含む詳細取得を常に実行）
	var pipelines []models.Pipeline
	shouldShowState := true

	if debug {
		// --debug指定時：詳細な進捗表示
		pipelines, err = manager.GetPipelineDetailsWithStateAndDetailedProgress(ctx)
	} else {
		// デフォルト：簡易な進捗表示
		pipelines, err = manager.GetPipelineDetailsWithStateAndProgress(ctx, true)
	}
	if err != nil {
		return fmt.Errorf("failed to get pipeline details with state: %w", err)
	}

	// フィルタリング
	if executeListAccountFilter != "" {
		pipelines = utils.FilterPipelinesByAccount(pipelines, executeListAccountFilter)
	}

	// state情報がある場合は、LATEST STAGE UPDATEの新しい順にソート
	if shouldShowState {
		utils.SortPipelinesByLatestStageUpdate(pipelines)
	}

	// 出力フォーマットを設定から取得（フラグで上書き可能）
	format := executeListFormat
	if format == "table" && cfg.Output.Format != "" {
		format = cfg.Output.Format
	}

	// キャッシュ使用状況を取得
	cacheUsage := manager.GetCacheUsage()

	// キャッシュディレクトリと使用状況を表示
	fmt.Printf("%s: %s\n", utils.Info("Cache Directory"), utils.Highlight(cfg.Cache.Directory))
	fmt.Printf("%s: Accounts=%s, Pipelines=%s, PipelineDetails=%s, PipelineStates=%s\n",
		utils.Info("Cache Status"),
		utils.GetCacheStatusColorText(cacheUsage.AccountsFromCache),
		utils.GetCacheStatusColorText(cacheUsage.PipelinesFromCache),
		utils.GetCacheStatusColorText(cacheUsage.PipelineDetailsFromCache),
		utils.GetCacheStatusColorText(cacheUsage.PipelineStatesFromCache))
	fmt.Println()

	// 出力
	formatter := utils.NewFormatterWithOptions(format, executeListShowDetails, shouldShowState)
	if err := formatter.FormatPipelines(pipelines, os.Stdout); err != nil {
		return fmt.Errorf("failed to format output: %w", err)
	}

	return nil
}

package utils

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/hacker65536/aft-pipeline-tool/internal/models"
)

// Formatter handles different output formats
type Formatter struct {
	format      string
	showDetails bool
	withState   bool
}

// NewFormatter creates a new formatter instance
func NewFormatter(format string) *Formatter {
	return &Formatter{format: format}
}

// NewFormatterWithOptions creates a new formatter instance with options
func NewFormatterWithOptions(format string, showDetails bool, withState bool) *Formatter {
	return &Formatter{
		format:      format,
		showDetails: showDetails,
		withState:   withState,
	}
}

// FormatPipelines formats pipeline data according to the specified format
func (f *Formatter) FormatPipelines(pipelines []models.Pipeline, w io.Writer) error {
	switch f.format {
	case "json":
		return f.formatJSON(pipelines, w)
	case "csv":
		return f.formatCSV(pipelines, w)
	case "table":
		return f.formatTable(pipelines, w)
	default:
		return fmt.Errorf("unsupported format: %s", f.format)
	}
}

// formatTable formats pipelines as a table
func (f *Formatter) formatTable(pipelines []models.Pipeline, w io.Writer) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	defer func() {
		if err := tw.Flush(); err != nil {
			// Log the error but don't fail the operation
			// as this is typically not critical
			_ = err // Explicitly ignore the error
		}
	}()

	// ヘッダー
	if f.showDetails {
		// 詳細表示モード：すべての項目を表示
		if f.withState {
			if _, err := fmt.Fprintln(tw, "ACCOUNT ID\tACCOUNT NAME\tPIPELINE NAME\tPIPELINE TYPE\tTRIGGER\tSTATE\tLAST UPDATED\tLATEST STAGE UPDATE"); err != nil {
				return err
			}
			if _, err := fmt.Fprintln(tw, "----------\t------------\t-------------\t-------------\t-------\t-----\t------------\t-------------------"); err != nil {
				return err
			}
		} else {
			if _, err := fmt.Fprintln(tw, "ACCOUNT ID\tACCOUNT NAME\tPIPELINE NAME\tPIPELINE TYPE\tTRIGGER\tLAST UPDATED"); err != nil {
				return err
			}
			if _, err := fmt.Fprintln(tw, "----------\t------------\t-------------\t-------------\t-------\t------------"); err != nil {
				return err
			}
		}
	} else {
		// シンプル表示モード：ACCOUNT NAMEとPIPELINE NAMEのみ
		if f.withState {
			if _, err := fmt.Fprintln(tw, "ACCOUNT NAME\tPIPELINE NAME\tSTATE\tLATEST STAGE UPDATE"); err != nil {
				return err
			}
			if _, err := fmt.Fprintln(tw, "------------\t-------------\t-----\t-------------------"); err != nil {
				return err
			}
		} else {
			if _, err := fmt.Fprintln(tw, "ACCOUNT NAME\tPIPELINE NAME"); err != nil {
				return err
			}
			if _, err := fmt.Fprintln(tw, "------------\t-------------"); err != nil {
				return err
			}
		}
	}

	// データ行
	for _, pipeline := range pipelines {
		if f.showDetails {
			// 詳細表示モード
			triggerDetails := getTriggerDetails(pipeline)
			pipelineType := getPipelineType(pipeline)

			if f.withState {
				stateInfo := getPipelineStateInfo(pipeline)
				latestStageUpdate := getLatestStageUpdateTime(pipeline)
				if _, err := fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
					pipeline.AccountID,
					pipeline.AccountName,
					pipeline.GetName(),
					pipelineType,
					triggerDetails,
					stateInfo,
					pipeline.GetUpdated().Local().Format("2006-01-02 15:04-0700"),
					latestStageUpdate,
				); err != nil {
					return err
				}
			} else {
				if _, err := fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n",
					pipeline.AccountID,
					pipeline.AccountName,
					pipeline.GetName(),
					pipelineType,
					triggerDetails,
					pipeline.GetUpdated().Local().Format("2006-01-02 15:04-0700"),
				); err != nil {
					return err
				}
			}
		} else {
			// シンプル表示モード
			if f.withState {
				stateInfo := getPipelineStateInfo(pipeline)
				latestStageUpdate := getLatestStageUpdateTime(pipeline)
				if _, err := fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n",
					pipeline.AccountName,
					pipeline.GetName(),
					stateInfo,
					latestStageUpdate,
				); err != nil {
					return err
				}
			} else {
				if _, err := fmt.Fprintf(tw, "%s\t%s\n",
					pipeline.AccountName,
					pipeline.GetName(),
				); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

// formatJSON formats pipelines as JSON
func (f *Formatter) formatJSON(pipelines []models.Pipeline, w io.Writer) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(pipelines)
}

// formatCSV formats pipelines as CSV
func (f *Formatter) formatCSV(pipelines []models.Pipeline, w io.Writer) error {
	writer := csv.NewWriter(w)
	defer writer.Flush()

	// ヘッダー
	if err := writer.Write([]string{
		"AccountID", "AccountName", "PipelineName", "PipelineType",
		"Trigger", "LastUpdated",
	}); err != nil {
		return err
	}

	// データ行
	for _, pipeline := range pipelines {
		triggerDetails := getTriggerDetails(pipeline)
		pipelineType := getPipelineType(pipeline)
		record := []string{
			pipeline.AccountID,
			pipeline.AccountName,
			pipeline.GetName(),
			pipelineType,
			triggerDetails,
			pipeline.GetUpdated().Local().Format("2006-01-02 15:04:05-0700"),
		}

		if err := writer.Write(record); err != nil {
			return err
		}
	}

	return nil
}

// getTriggerDetails returns detailed trigger information from pipeline triggers
func getTriggerDetails(pipeline models.Pipeline) string {
	if len(pipeline.Pipeline.Triggers) == 0 {
		return "No triggers"
	}

	var details []string
	for _, trigger := range pipeline.Pipeline.Triggers {
		if trigger.GitConfiguration != nil {
			sourceActionName := trigger.GitConfiguration.SourceActionName

			// Validate trigger configuration and show only the status
			configStatus := validateTriggerConfiguration(pipeline, trigger)
			if configStatus != "" {
				details = append(details, configStatus)
			} else {
				// If no validation applies, show the action name
				details = append(details, fmt.Sprintf("Action:%s", sourceActionName))
			}
		}
	}

	if len(details) == 0 {
		return "No git triggers"
	}

	return strings.Join(details, " | ")
}

// validateTriggerConfiguration validates trigger configuration based on sourceActionName
func validateTriggerConfiguration(pipeline models.Pipeline, trigger models.Trigger) string {
	sourceActionName := trigger.GitConfiguration.SourceActionName

	// Check if sourceActionName is one of the expected values
	if sourceActionName != "aft-global-customizations" && sourceActionName != "aft-account-customizations" {
		return ""
	}

	// For aft-account-customizations, validate file path patterns
	if sourceActionName == "aft-account-customizations" {
		accountName := pipeline.AccountName
		expectedPattern := fmt.Sprintf("%s/terraform/", accountName)

		for _, push := range trigger.GitConfiguration.Push {
			if push.FilePaths != nil && len(push.FilePaths.Includes) > 0 {
				hasValidPattern := false
				for _, filePath := range push.FilePaths.Includes {
					// Check if the file path matches the expected pattern: account名/terraform/*.tf
					if strings.HasPrefix(filePath, expectedPattern) && strings.HasSuffix(filePath, ".tf") {
						hasValidPattern = true
						break
					}
				}
				if !hasValidPattern {
					return "[INVALID_CONFIG]"
				}
			} else {
				// No file path filters configured for aft-account-customizations
				return "[INVALID_CONFIG]"
			}
		}
	}

	return "[VALID_CONFIG]"
}

// getPipelineType returns the pipeline type or "Unknown" if not set
func getPipelineType(pipeline models.Pipeline) string {
	if pipeline.Pipeline.PipelineType == "" {
		return "Unknown"
	}
	return pipeline.Pipeline.PipelineType
}

// getPipelineStateInfo returns a summary of pipeline state information
func getPipelineStateInfo(pipeline models.Pipeline) string {
	if pipeline.State == nil {
		return "No state"
	}

	if len(pipeline.State.StageStates) == 0 {
		return "No stages"
	}

	// 各ステージの状態を確認し、最初にSucceededではないものを見つける
	for i, stage := range pipeline.State.StageStates {
		if stage.LatestExecution != nil {
			status := stage.LatestExecution.Status
			if status != "Succeeded" {
				// 最初にSucceededではないステージのindex番号とstatusを表示
				return fmt.Sprintf("%d:%s", i, status)
			}
		} else {
			// 実行情報がない場合も最初の非Succeededとして扱う
			return fmt.Sprintf("%d:NoExecution", i)
		}
	}

	// すべてのステージがSucceededの場合、pipelineExecutionIdが全て同じかチェック
	var pipelineExecutionId string
	allSamePipelineExecutionId := true

	for i, stage := range pipeline.State.StageStates {
		if stage.LatestExecution != nil {
			if i == 0 {
				pipelineExecutionId = stage.LatestExecution.PipelineExecutionId
			} else if stage.LatestExecution.PipelineExecutionId != pipelineExecutionId {
				allSamePipelineExecutionId = false
				break
			}
		} else {
			allSamePipelineExecutionId = false
			break
		}
	}

	// すべてのステージがSucceededかつpipelineExecutionIdが全て同じ場合
	if allSamePipelineExecutionId {
		return "Succeeded"
	}

	// pipelineExecutionIdが異なる場合は、最初の異なるステージの情報を表示
	for i, stage := range pipeline.State.StageStates {
		if stage.LatestExecution != nil {
			if i == 0 {
				pipelineExecutionId = stage.LatestExecution.PipelineExecutionId
			} else if stage.LatestExecution.PipelineExecutionId != pipelineExecutionId {
				return fmt.Sprintf("%d:DiffExecId", i)
			}
		}
	}

	return "Succeeded"
}

// getLatestStageUpdateTime returns the latest stage update time formatted as string in local timezone with timezone offset
func getLatestStageUpdateTime(pipeline models.Pipeline) string {
	latestTime := pipeline.GetLatestStageUpdateTime()
	if latestTime == nil {
		return "N/A"
	}
	// Convert to local timezone before formatting with timezone offset (ISO8601-like)
	localTime := latestTime.Local()
	return localTime.Format("2006-01-02 15:04-0700")
}

// FilterPipelinesByAccount filters pipelines by account ID pattern
func FilterPipelinesByAccount(pipelines []models.Pipeline, pattern string) []models.Pipeline {
	if pattern == "" {
		return pipelines
	}

	var filtered []models.Pipeline
	for _, pipeline := range pipelines {
		if strings.Contains(pipeline.AccountID, pattern) {
			filtered = append(filtered, pipeline)
		}
	}

	return filtered
}

// FilterPipelinesByNameAccountIDOrName filters pipelines by pipeline name, account ID, or account name
func FilterPipelinesByNameAccountIDOrName(pipelines []models.Pipeline, pattern string) []models.Pipeline {
	if pattern == "" {
		return pipelines
	}

	var filtered []models.Pipeline
	lowerPattern := strings.ToLower(pattern)

	for _, pipeline := range pipelines {
		// Check pipeline name
		if strings.Contains(strings.ToLower(pipeline.GetName()), lowerPattern) {
			filtered = append(filtered, pipeline)
			continue
		}

		// Check account ID
		if strings.Contains(strings.ToLower(pipeline.AccountID), lowerPattern) {
			filtered = append(filtered, pipeline)
			continue
		}

		// Check account name
		if strings.Contains(strings.ToLower(pipeline.AccountName), lowerPattern) {
			filtered = append(filtered, pipeline)
			continue
		}
	}

	return filtered
}

// FilterPipelinesByState filters pipelines by their current state
func FilterPipelinesByState(pipelines []models.Pipeline, stateFilter string) []models.Pipeline {
	if stateFilter == "" {
		return pipelines
	}

	var filtered []models.Pipeline
	lowerStateFilter := strings.ToLower(stateFilter)

	for _, pipeline := range pipelines {
		pipelineState := getPipelineStateInfo(pipeline)
		actualStatus := getActualPipelineStatus(pipeline)

		// Handle different state representations
		switch lowerStateFilter {
		case "succeeded", "success":
			if pipelineState == "Succeeded" {
				filtered = append(filtered, pipeline)
			}
		case "failed", "failure", "fail":
			// Check for actual failed status
			if actualStatus == "Failed" || strings.Contains(actualStatus, "Failed") {
				filtered = append(filtered, pipeline)
			}
		case "inprogress", "in-progress", "running":
			// Check for actual in-progress status
			if actualStatus == "InProgress" || strings.Contains(actualStatus, "InProgress") {
				filtered = append(filtered, pipeline)
			}
		default:
			// Direct string match for custom states
			if strings.Contains(strings.ToLower(pipelineState), lowerStateFilter) ||
				strings.Contains(strings.ToLower(actualStatus), lowerStateFilter) {
				filtered = append(filtered, pipeline)
			}
		}
	}

	return filtered
}

// getActualPipelineStatus returns the actual pipeline execution status
func getActualPipelineStatus(pipeline models.Pipeline) string {
	if pipeline.State == nil || len(pipeline.State.StageStates) == 0 {
		return "No state"
	}

	// Check the latest execution status from stages
	for _, stage := range pipeline.State.StageStates {
		if stage.LatestExecution != nil {
			status := stage.LatestExecution.Status
			// Return the first non-Succeeded status found
			if status != "Succeeded" {
				return status
			}
		}
	}

	// If all stages are succeeded, return "Succeeded"
	return "Succeeded"
}

// GetCacheStatusColorText returns colored cache status text
func GetCacheStatusColorText(fromCache bool) string {
	if fromCache {
		return Success("Cache")
	}
	return Warning("API")
}

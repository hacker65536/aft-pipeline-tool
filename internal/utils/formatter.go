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
					pipeline.GetUpdated().Format("2006-01-02 15:04"),
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
					pipeline.GetUpdated().Format("2006-01-02 15:04"),
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
			pipeline.GetUpdated().Format("2006-01-02 15:04:05"),
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

	// 各ステージの状態を確認
	var failedStageIndexes []string
	allSucceeded := true
	hasStages := false

	for i, stage := range pipeline.State.StageStates {
		hasStages = true
		if stage.LatestExecution != nil {
			status := stage.LatestExecution.Status
			// すべてのステージがSucceededかどうかをチェック
			if status != "Succeeded" {
				allSucceeded = false
				// 失敗したステージのインデックス（1ベース）を記録
				failedStageIndexes = append(failedStageIndexes, fmt.Sprintf("%d", i+1))
			}
		} else {
			allSucceeded = false
			// 実行情報がないステージのインデックス（1ベース）を記録
			failedStageIndexes = append(failedStageIndexes, fmt.Sprintf("%d", i+1))
		}
	}

	if !hasStages {
		return "No stages"
	}

	// すべてのステージがSucceededの場合は"Succeeded"を表示
	if allSucceeded {
		return "Succeeded"
	}

	// 失敗したステージのインデックスを表示
	return strings.Join(failedStageIndexes, ",")
}

// getLatestStageUpdateTime returns the latest stage update time formatted as string
func getLatestStageUpdateTime(pipeline models.Pipeline) string {
	latestTime := pipeline.GetLatestStageUpdateTime()
	if latestTime == nil {
		return "N/A"
	}
	return latestTime.Format("2006-01-02 15:04")
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

// GetCacheStatusColorText returns colored cache status text
func GetCacheStatusColorText(fromCache bool) string {
	if fromCache {
		return Success("Cache")
	}
	return Warning("API")
}

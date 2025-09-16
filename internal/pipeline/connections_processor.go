package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/hacker65536/aft-pipeline-tool/internal/aws"
	"github.com/hacker65536/aft-pipeline-tool/internal/cache"
	"github.com/hacker65536/aft-pipeline-tool/internal/models"
	"github.com/hacker65536/aft-pipeline-tool/internal/utils"
)

// ConnectionsProcessor handles pipeline connection fixes
type ConnectionsProcessor struct {
	newConnectionArn string
	sourceActionName string
}

// NewConnectionsProcessor creates a new connections processor
func NewConnectionsProcessor(newConnectionArn, sourceActionName string) *ConnectionsProcessor {
	return &ConnectionsProcessor{
		newConnectionArn: newConnectionArn,
		sourceActionName: sourceActionName,
	}
}

// ProcessPipeline processes a single pipeline for connection fixes
func (p *ConnectionsProcessor) ProcessPipeline(ctx context.Context, awsClient *aws.Client, pipeline *models.Pipeline, dryRun bool, fileCache *cache.FileCache) error {
	// Source stageからactionsを分析
	sourceActions := p.analyzeSourceActionsForConnections(pipeline)
	if len(sourceActions) == 0 {
		fmt.Println("  No CodeStarSourceConnection actions found, skipping...")
		return nil
	}

	fmt.Printf("  Found %d CodeStarSourceConnection actions:\n", len(sourceActions))
	for _, action := range sourceActions {
		currentConnectionArn := ""
		if connArn, exists := action.Configuration["ConnectionArn"]; exists {
			if connArnStr, ok := connArn.(string); ok {
				currentConnectionArn = connArnStr
			}
		}
		fmt.Printf("    - %s (current: %s)\n", action.Name, currentConnectionArn)
	}

	return ProcessPipelineWithCommonLogic(ctx, awsClient, pipeline, dryRun, fileCache, p, func(ctx context.Context, awsClient *aws.Client, pipeline *models.Pipeline) error {
		return awsClient.UpdatePipelineConnectionsV2(ctx, pipeline.GetName(), p.newConnectionArn, p.sourceActionName)
	})
}

// ShowDiff displays the differences for dry-run mode
func (p *ConnectionsProcessor) ShowDiff(pipeline *models.Pipeline) error {
	sourceActions := p.analyzeSourceActionsForConnections(pipeline)
	return p.showConnectionsDiff(pipeline, sourceActions)
}

// ShouldUpdate determines if the pipeline needs to be updated
func (p *ConnectionsProcessor) ShouldUpdate(pipeline *models.Pipeline) bool {
	sourceActions := p.analyzeSourceActionsForConnections(pipeline)
	return p.shouldUpdateConnections(sourceActions)
}

// GetProcessorName returns the name of the processor for logging
func (p *ConnectionsProcessor) GetProcessorName() string {
	return "connections"
}

func (p *ConnectionsProcessor) analyzeSourceActionsForConnections(pipeline *models.Pipeline) []models.Action {
	var sourceActions []models.Action

	for _, stage := range pipeline.Pipeline.Stages {
		if stage.Name == "Source" {
			for _, action := range stage.Actions {
				// CodeStarSourceConnectionのactionのみを対象とする
				if action.ActionTypeId.Provider == "CodeStarSourceConnection" {
					// 特定のsource actionが指定されている場合はそれのみを対象とする
					if p.sourceActionName != "" && action.Name != p.sourceActionName {
						continue
					}
					sourceActions = append(sourceActions, action)
				}
			}
			break
		}
	}

	return sourceActions
}

func (p *ConnectionsProcessor) shouldUpdateConnections(sourceActions []models.Action) bool {
	for _, action := range sourceActions {
		currentConnectionArn := ""
		if connArn, exists := action.Configuration["ConnectionArn"]; exists {
			if connArnStr, ok := connArn.(string); ok {
				currentConnectionArn = connArnStr
			}
		}

		// 現在のConnectionArnと新しいConnectionArnが異なる場合は更新が必要
		if currentConnectionArn != p.newConnectionArn {
			return true
		}
	}
	return false
}

func (p *ConnectionsProcessor) showConnectionsDiff(pipeline *models.Pipeline, sourceActions []models.Action) error {
	// Check if there are any differences
	hasDifferences := p.shouldUpdateConnections(sourceActions)

	if !hasDifferences {
		fmt.Printf("  %s\n", utils.Success("✓ No changes needed - ConnectionArn values are already properly configured"))
		return nil
	}

	fmt.Printf("\n    %s\n", utils.Highlight("=== CONNECTION ARN DIFF ==="))

	for _, action := range sourceActions {
		currentConnectionArn := ""
		if connArn, exists := action.Configuration["ConnectionArn"]; exists {
			if connArnStr, ok := connArn.(string); ok {
				currentConnectionArn = connArnStr
			}
		}

		if currentConnectionArn != p.newConnectionArn {
			fmt.Printf("    %s:\n", utils.Warning(fmt.Sprintf("Action: %s", action.Name)))
			fmt.Printf("      %s: %s\n", "Current ConnectionArn", utils.Colorize(utils.Red, currentConnectionArn))
			fmt.Printf("      %s: %s\n", "New ConnectionArn", utils.Colorize(utils.Green, p.newConnectionArn))
		}
	}

	// Show JSON diff
	fmt.Printf("\n    %s\n", utils.Highlight("=== CONFIGURATION DIFF ==="))

	for _, action := range sourceActions {
		currentConnectionArn := ""
		if connArn, exists := action.Configuration["ConnectionArn"]; exists {
			if connArnStr, ok := connArn.(string); ok {
				currentConnectionArn = connArnStr
			}
		}

		if currentConnectionArn != p.newConnectionArn {
			fmt.Printf("    %s:\n", utils.Warning(fmt.Sprintf("Action: %s", action.Name)))

			// Show current configuration
			fmt.Printf("      %s\n", utils.Error("CURRENT (-):"))
			currentConfig := map[string]interface{}{
				"ConnectionArn": currentConnectionArn,
			}
			currentJSON, err := json.MarshalIndent(currentConfig, "        ", "  ")
			if err != nil {
				return fmt.Errorf("failed to marshal current configuration: %w", err)
			}
			fmt.Printf("        %s\n", utils.Colorize(utils.Red, string(currentJSON)))

			// Show new configuration
			fmt.Printf("      %s\n", utils.Success("NEW (+):"))
			newConfig := map[string]interface{}{
				"ConnectionArn": p.newConnectionArn,
			}
			newJSON, err := json.MarshalIndent(newConfig, "        ", "  ")
			if err != nil {
				return fmt.Errorf("failed to marshal new configuration: %w", err)
			}
			fmt.Printf("        %s\n", utils.Colorize(utils.Green, string(newJSON)))
		}
	}

	return nil
}

// ValidateConnectionFlags validates the connection-related flags
func ValidateConnectionFlags(newConnectionArn, sourceActionName string) error {
	// ConnectionArnが指定されているかチェック
	if newConnectionArn == "" {
		return fmt.Errorf("connection-arn is required")
	}

	// ConnectionArnの形式をチェック（基本的な検証）
	if !strings.HasPrefix(newConnectionArn, "arn:aws:codeconnections:") {
		return fmt.Errorf("invalid connection-arn format. Expected format: arn:aws:codeconnections:region:account:connection/connection-id")
	}

	// source-actionが指定されている場合の検証
	if sourceActionName != "" {
		validSourceActions := []string{"aft-global-customizations", "aft-account-customizations"}
		isValidSourceAction := false
		for _, action := range validSourceActions {
			if sourceActionName == action {
				isValidSourceAction = true
				break
			}
		}
		if !isValidSourceAction {
			return fmt.Errorf("invalid source-action '%s'. Valid options: %v", sourceActionName, validSourceActions)
		}
	}

	return nil
}

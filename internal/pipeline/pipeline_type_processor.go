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

// PipelineTypeProcessor handles pipeline type fixes
type PipelineTypeProcessor struct {
	newPipelineType string
}

// NewPipelineTypeProcessor creates a new pipeline type processor
func NewPipelineTypeProcessor(newPipelineType string) *PipelineTypeProcessor {
	return &PipelineTypeProcessor{
		newPipelineType: newPipelineType,
	}
}

// ProcessPipeline processes a single pipeline for pipeline type fixes
func (p *PipelineTypeProcessor) ProcessPipeline(ctx context.Context, awsClient *aws.Client, pipeline *models.Pipeline, dryRun bool, fileCache *cache.FileCache) error {
	currentPipelineType := pipeline.Pipeline.PipelineType
	fmt.Printf("  Current pipeline type: %s\n", currentPipelineType)
	fmt.Printf("  Target pipeline type: %s\n", p.newPipelineType)

	return ProcessPipelineWithCommonLogic(ctx, awsClient, pipeline, dryRun, fileCache, p, func(ctx context.Context, awsClient *aws.Client, pipeline *models.Pipeline) error {
		return awsClient.UpdatePipelineTypeV2(ctx, pipeline.GetName(), p.newPipelineType)
	})
}

// ShowDiff displays the differences for dry-run mode
func (p *PipelineTypeProcessor) ShowDiff(pipeline *models.Pipeline) error {
	return p.showPipelineTypeDiff(pipeline)
}

// ShouldUpdate determines if the pipeline needs to be updated
func (p *PipelineTypeProcessor) ShouldUpdate(pipeline *models.Pipeline) bool {
	currentPipelineType := pipeline.Pipeline.PipelineType
	// 現在のPipelineTypeと新しいPipelineTypeが異なる場合は更新が必要
	return currentPipelineType != p.newPipelineType
}

// GetProcessorName returns the name of the processor for logging
func (p *PipelineTypeProcessor) GetProcessorName() string {
	return "pipeline type"
}

func (p *PipelineTypeProcessor) showPipelineTypeDiff(pipeline *models.Pipeline) error {
	currentPipelineType := pipeline.Pipeline.PipelineType

	// Check if there are any differences
	hasDifferences := p.ShouldUpdate(pipeline)

	if !hasDifferences {
		fmt.Printf("  %s\n", utils.Success("✓ No changes needed - pipeline type is already properly configured"))
		return nil
	}

	fmt.Printf("\n    %s\n", utils.Highlight("=== PIPELINE TYPE DIFF ==="))
	fmt.Printf("    %s: %s\n", "Current PipelineType", utils.Colorize(utils.Red, currentPipelineType))
	fmt.Printf("    %s: %s\n", "New PipelineType", utils.Colorize(utils.Green, p.newPipelineType))

	// Show JSON diff
	fmt.Printf("\n    %s\n", utils.Highlight("=== CONFIGURATION DIFF ==="))

	// Show current configuration
	fmt.Printf("    %s\n", utils.Error("CURRENT (-):"))
	currentConfig := map[string]interface{}{
		"pipelineType": currentPipelineType,
	}
	currentJSON, err := json.MarshalIndent(currentConfig, "      ", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal current configuration: %w", err)
	}
	fmt.Printf("      %s\n", utils.Colorize(utils.Red, string(currentJSON)))

	// Show new configuration
	fmt.Printf("    %s\n", utils.Success("NEW (+):"))
	newConfig := map[string]interface{}{
		"pipelineType": p.newPipelineType,
	}
	newJSON, err := json.MarshalIndent(newConfig, "      ", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal new configuration: %w", err)
	}
	fmt.Printf("      %s\n", utils.Colorize(utils.Green, string(newJSON)))

	return nil
}

// ValidatePipelineTypeFlags validates the pipeline type flags
func ValidatePipelineTypeFlags(newPipelineType *string) error {
	// PipelineTypeが指定されているかチェック
	if *newPipelineType == "" {
		return fmt.Errorf("pipeline-type is required")
	}

	// PipelineTypeの値をチェック
	validPipelineTypes := []string{"V1", "V2"}
	isValidPipelineType := false
	for _, validType := range validPipelineTypes {
		if strings.EqualFold(*newPipelineType, validType) {
			// 大文字小文字を統一
			*newPipelineType = validType
			isValidPipelineType = true
			break
		}
	}

	if !isValidPipelineType {
		return fmt.Errorf("invalid pipeline-type '%s'. Valid options: %v", *newPipelineType, validPipelineTypes)
	}

	return nil
}

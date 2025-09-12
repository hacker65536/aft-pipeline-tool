package pipeline

import (
	"context"
	"fmt"
	"strings"

	"github.com/hacker65536/aft-pipeline-tool/internal/aws"
	"github.com/hacker65536/aft-pipeline-tool/internal/cache"
	"github.com/hacker65536/aft-pipeline-tool/internal/models"
	"github.com/hacker65536/aft-pipeline-tool/pkg/aft"
)

// ProcessorFunc defines the function signature for pipeline processing
type ProcessorFunc func(ctx context.Context, awsClient *aws.Client, pipeline *models.Pipeline, dryRun bool, fileCache *cache.FileCache) error

// Processor handles common pipeline processing operations
type Processor struct {
	manager   *aft.Manager
	awsClient *aws.Client
	fileCache *cache.FileCache
}

// NewProcessor creates a new processor instance
func NewProcessor(manager *aft.Manager, awsClient *aws.Client, fileCache *cache.FileCache) *Processor {
	return &Processor{
		manager:   manager,
		awsClient: awsClient,
		fileCache: fileCache,
	}
}

// ProcessSinglePipeline processes a single pipeline with the given processor function
func (p *Processor) ProcessSinglePipeline(ctx context.Context, pipelineName string, dryRun bool, processorFunc ProcessorFunc) error {
	fmt.Printf("Processing pipeline: %s\n", pipelineName)

	// パイプライン詳細を取得
	pipeline, err := p.awsClient.GetPipelineDetails(ctx, pipelineName)
	if err != nil {
		return fmt.Errorf("failed to get pipeline details: %w", err)
	}

	// AccountIDを抽出
	accountID := extractAccountIDFromPipelineName(pipelineName)
	pipeline.AccountID = accountID

	// AccountNameを設定するためにアカウント一覧を取得
	accounts, err := p.manager.GetAccounts(ctx)
	if err != nil {
		return fmt.Errorf("failed to get accounts: %w", err)
	}

	// AccountIDからAccountNameを取得
	for _, account := range accounts {
		if account.ID == accountID {
			pipeline.AccountName = account.Name
			break
		}
	}

	return processorFunc(ctx, p.awsClient, pipeline, dryRun, p.fileCache)
}

// ProcessAllPipelines processes all pipelines with the given processor function
func (p *Processor) ProcessAllPipelines(ctx context.Context, dryRun bool, processorFunc ProcessorFunc) error {
	fmt.Println("Processing all AFT pipelines...")

	// すべてのパイプライン詳細を取得
	pipelines, err := p.manager.GetPipelineDetails(ctx)
	if err != nil {
		return fmt.Errorf("failed to get pipeline details: %w", err)
	}

	fmt.Printf("Found %d pipelines to process\n", len(pipelines))

	var errors []error
	for _, pipeline := range pipelines {
		fmt.Printf("\nProcessing pipeline: %s\n", pipeline.GetName())
		if err := processorFunc(ctx, p.awsClient, &pipeline, dryRun, p.fileCache); err != nil {
			errors = append(errors, fmt.Errorf("failed to process %s: %w", pipeline.GetName(), err))
			fmt.Printf("Error: %v\n", err)
		}
	}

	if len(errors) > 0 {
		fmt.Printf("\nCompleted with %d errors\n", len(errors))
		for _, err := range errors {
			fmt.Printf("- %v\n", err)
		}
		return fmt.Errorf("processing completed with %d errors", len(errors))
	}

	fmt.Println("\nAll pipelines processed successfully")
	return nil
}

// ClearPipelineCache clears cache for a pipeline after successful update
func (p *Processor) ClearPipelineCache(pipelineName string) error {
	if p.fileCache == nil {
		return nil
	}

	fmt.Println("  Clearing cache for updated pipeline...")
	if err := p.fileCache.DeletePipelineCache(pipelineName); err != nil {
		fmt.Printf("  Warning: Failed to clear cache for pipeline %s: %v\n", pipelineName, err)
		// Don't return error as cache deletion failure shouldn't fail the main operation
		return nil
	}
	fmt.Println("  Pipeline cache cleared successfully")
	return nil
}

// extractAccountIDFromPipelineName extracts account ID from pipeline name
func extractAccountIDFromPipelineName(pipelineName string) string {
	// AFT pipeline naming convention: aft-account-{account-id}-{type}
	parts := strings.Split(pipelineName, "-")
	if len(parts) >= 3 && parts[0] == "aft" && parts[1] == "account" {
		return parts[2]
	}
	return ""
}

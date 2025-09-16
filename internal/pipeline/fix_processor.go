package pipeline

import (
	"context"
	"fmt"

	"github.com/hacker65536/aft-pipeline-tool/internal/aws"
	"github.com/hacker65536/aft-pipeline-tool/internal/cache"
	"github.com/hacker65536/aft-pipeline-tool/internal/config"
	"github.com/hacker65536/aft-pipeline-tool/internal/models"
	"github.com/hacker65536/aft-pipeline-tool/internal/utils"
	"github.com/hacker65536/aft-pipeline-tool/pkg/aft"
)

// FixProcessor defines the interface for pipeline fix operations
type FixProcessor interface {
	// ProcessPipeline processes a single pipeline and returns whether it needs update and any error
	ProcessPipeline(ctx context.Context, awsClient *aws.Client, pipeline *models.Pipeline, dryRun bool, fileCache *cache.FileCache) error

	// ShowDiff displays the differences for dry-run mode
	ShowDiff(pipeline *models.Pipeline) error

	// ShouldUpdate determines if the pipeline needs to be updated
	ShouldUpdate(pipeline *models.Pipeline) bool

	// GetProcessorName returns the name of the processor for logging
	GetProcessorName() string
}

// BaseFixProcessor provides common functionality for all fix processors
type BaseFixProcessor struct {
	processor FixProcessor
}

// NewBaseFixProcessor creates a new base fix processor
func NewBaseFixProcessor(processor FixProcessor) *BaseFixProcessor {
	return &BaseFixProcessor{
		processor: processor,
	}
}

// ProcessSinglePipeline processes a single pipeline with common setup and error handling
func (b *BaseFixProcessor) ProcessSinglePipeline(ctx context.Context, manager *aft.Manager, awsClient *aws.Client, pipelineName string, dryRun bool, fileCache *cache.FileCache) error {
	fmt.Printf("Processing pipeline: %s\n", pipelineName)

	// パイプライン詳細を取得
	pipeline, err := awsClient.GetPipelineDetails(ctx, pipelineName)
	if err != nil {
		return fmt.Errorf("failed to get pipeline details: %w", err)
	}

	// AccountIDとAccountNameを設定（共通化された関数を使用）
	if err := utils.SetPipelineAccountInfo(ctx, pipeline, manager); err != nil {
		return err
	}

	return b.processor.ProcessPipeline(ctx, awsClient, pipeline, dryRun, fileCache)
}

// ProcessAllPipelines processes all pipelines with common setup and error handling
func (b *BaseFixProcessor) ProcessAllPipelines(ctx context.Context, manager *aft.Manager, awsClient *aws.Client, dryRun bool, fileCache *cache.FileCache) error {
	fmt.Printf("Processing all AFT pipelines using %s...\n", b.processor.GetProcessorName())

	// すべてのパイプライン詳細を取得
	pipelines, err := manager.GetPipelineDetails(ctx)
	if err != nil {
		return fmt.Errorf("failed to get pipeline details: %w", err)
	}

	fmt.Printf("Found %d pipelines to process\n", len(pipelines))

	var errors []error
	for _, pipeline := range pipelines {
		fmt.Printf("\nProcessing pipeline: %s\n", pipeline.GetName())
		if err := b.processor.ProcessPipeline(ctx, awsClient, &pipeline, dryRun, fileCache); err != nil {
			errors = append(errors, fmt.Errorf("failed to process %s: %w", pipeline.GetName(), err))
			fmt.Printf("Error: %v\n", err)
		}
	}

	if len(errors) > 0 {
		fmt.Printf("\nCompleted with %d errors\n", len(errors))
		for _, err := range errors {
			fmt.Printf("- %v\n", err)
		}
		return fmt.Errorf("%s completed with %d errors", b.processor.GetProcessorName(), len(errors))
	}

	fmt.Println("\nAll pipelines processed successfully")
	return nil
}

// CommonSetup provides common initialization for all fix commands
type CommonSetup struct {
	Config    *config.Config
	AWSClient *aws.Client
	FileCache *cache.FileCache
	Manager   *aft.Manager
}

// NewCommonSetup initializes common components for fix commands
func NewCommonSetup(ctx context.Context) (*CommonSetup, error) {
	// 設定読み込み
	cfg, err := config.LoadConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	// AWS クライアント初期化
	awsClient, err := aws.NewClient(ctx, cfg.AWS.Region, cfg.AWS.Profile)
	if err != nil {
		return nil, fmt.Errorf("failed to create AWS client: %w", err)
	}

	// キャッシュ初期化（AWS接続情報ごとに分離）
	awsContext := cache.NewAWSContext(cfg.AWS.Region, cfg.AWS.Profile)
	fileCache := cache.NewFileCacheWithContext(cfg.Cache.Directory, awsContext)

	// AFT マネージャー初期化
	manager := aft.NewManager(awsClient, fileCache, cfg)

	return &CommonSetup{
		Config:    cfg,
		AWSClient: awsClient,
		FileCache: fileCache,
		Manager:   manager,
	}, nil
}

// ProcessPipelineWithCommonLogic provides common pipeline processing logic
func ProcessPipelineWithCommonLogic(ctx context.Context, awsClient *aws.Client, pipeline *models.Pipeline, dryRun bool, fileCache *cache.FileCache, processor FixProcessor, updateFunc func(ctx context.Context, awsClient *aws.Client, pipeline *models.Pipeline) error) error {
	// 更新が必要かどうかを判定
	needsUpdate := processor.ShouldUpdate(pipeline)

	if dryRun {
		fmt.Printf("  DRY RUN - Showing current and expected %s:\n", processor.GetProcessorName())
		err := processor.ShowDiff(pipeline)
		if err != nil {
			fmt.Printf("  Error generating diff: %v\n", err)
		}

		if !needsUpdate {
			fmt.Printf("  %s is already properly configured, no changes needed.\n", processor.GetProcessorName())
		} else {
			fmt.Printf("  Changes would be made to fix %s.\n", processor.GetProcessorName())
		}
		return nil
	}

	if !needsUpdate {
		fmt.Printf("  %s is already properly configured, skipping...\n", processor.GetProcessorName())
		// Cache is maintained for skipped pipelines (no cache deletion)
		return nil
	}

	// パイプラインを更新
	fmt.Printf("  Updating %s...\n", processor.GetProcessorName())
	err := updateFunc(ctx, awsClient, pipeline)
	if err != nil {
		return fmt.Errorf("failed to update %s: %w", processor.GetProcessorName(), err)
	}

	fmt.Printf("  %s updated successfully\n", processor.GetProcessorName())

	// Delete cache for updated pipeline（共通化された関数を使用）
	utils.ClearPipelineCache(pipeline.GetName(), fileCache)

	return nil
}

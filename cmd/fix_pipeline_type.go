package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/hacker65536/aft-pipeline-tool/internal/aws"
	"github.com/hacker65536/aft-pipeline-tool/internal/cache"
	"github.com/hacker65536/aft-pipeline-tool/internal/config"
	"github.com/hacker65536/aft-pipeline-tool/internal/models"
	"github.com/hacker65536/aft-pipeline-tool/internal/utils"
	"github.com/hacker65536/aft-pipeline-tool/pkg/aft"
)

var fixPipelineTypeCmd = &cobra.Command{
	Use:   "fix-pipeline-type",
	Short: "Fix pipeline type configuration",
	Long: `Analyze and fix pipeline type configuration.
Update PipelineType field for CodePipeline pipelines.
Valid pipeline types: V1, V2`,
	RunE: runFixPipelineType,
}

var (
	fixPipelineTypePipeline string
	fixPipelineTypeDryRun   bool
	newPipelineType         string
)

func init() {
	rootCmd.AddCommand(fixPipelineTypeCmd)

	fixPipelineTypeCmd.Flags().StringVar(&fixPipelineTypePipeline, "pipeline", "", "Pipeline name to fix (if not specified, all pipelines will be processed)")
	fixPipelineTypeCmd.Flags().BoolVar(&fixPipelineTypeDryRun, "dry-run", false, "Show what would be changed without making actual changes")
	fixPipelineTypeCmd.Flags().StringVar(&newPipelineType, "pipeline-type", "", "New PipelineType to set (V1 or V2)")

	// Mark pipeline-type as required
	if err := fixPipelineTypeCmd.MarkFlagRequired("pipeline-type"); err != nil {
		fmt.Printf("Error marking pipeline-type flag as required: %v\n", err)
	}
}

func runFixPipelineType(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// 入力検証
	if err := validatePipelineTypeFlags(); err != nil {
		return err
	}

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
	// キャッシュ初期化（AWS接続情報ごとに分離）
	awsContext := cache.NewAWSContext(cfg.AWS.Region, cfg.AWS.Profile)
	fileCache := cache.NewFileCacheWithContext(cfg.Cache.Directory, awsContext)

	// AFT マネージャー初期化
	manager := aft.NewManager(awsClient, fileCache, cfg)

	if fixPipelineTypePipeline != "" {
		// 特定のパイプラインを処理
		return processSinglePipelineType(ctx, manager, awsClient, fixPipelineTypePipeline, fixPipelineTypeDryRun, fileCache)
	} else {
		// すべてのパイプラインを処理
		return processAllPipelinesType(ctx, manager, awsClient, fixPipelineTypeDryRun, fileCache)
	}
}

func processSinglePipelineType(ctx context.Context, manager *aft.Manager, awsClient *aws.Client, pipelineName string, dryRun bool, fileCache *cache.FileCache) error {
	fmt.Printf("Processing pipeline: %s\n", pipelineName)

	// パイプライン詳細を取得
	pipeline, err := awsClient.GetPipelineDetails(ctx, pipelineName)
	if err != nil {
		return fmt.Errorf("failed to get pipeline details: %w", err)
	}

	// AccountIDを抽出
	accountID := extractAccountIDFromPipelineName(pipelineName)
	pipeline.AccountID = accountID

	// AccountNameを設定するためにアカウント一覧を取得
	accounts, err := manager.GetAccounts(ctx)
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

	return fixPipelineTypeConfigurationWithCache(ctx, awsClient, pipeline, dryRun, fileCache)
}

func processAllPipelinesType(ctx context.Context, manager *aft.Manager, awsClient *aws.Client, dryRun bool, fileCache *cache.FileCache) error {
	fmt.Println("Processing all AFT pipelines...")

	// すべてのパイプライン詳細を取得
	pipelines, err := manager.GetPipelineDetails(ctx)
	if err != nil {
		return fmt.Errorf("failed to get pipeline details: %w", err)
	}

	fmt.Printf("Found %d pipelines to process\n", len(pipelines))

	var errors []error
	for _, pipeline := range pipelines {
		fmt.Printf("\nProcessing pipeline: %s\n", pipeline.GetName())
		if err := fixPipelineTypeConfigurationWithCache(ctx, awsClient, &pipeline, dryRun, fileCache); err != nil {
			errors = append(errors, fmt.Errorf("failed to fix pipeline type for %s: %w", pipeline.GetName(), err))
			fmt.Printf("Error: %v\n", err)
		}
	}

	if len(errors) > 0 {
		fmt.Printf("\nCompleted with %d errors\n", len(errors))
		for _, err := range errors {
			fmt.Printf("- %v\n", err)
		}
		return fmt.Errorf("fix-pipeline-type completed with %d errors", len(errors))
	}

	fmt.Println("\nAll pipelines processed successfully")
	return nil
}

func fixPipelineTypeConfigurationWithCache(ctx context.Context, awsClient *aws.Client, pipeline *models.Pipeline, dryRun bool, fileCache *cache.FileCache) error {
	currentPipelineType := pipeline.Pipeline.PipelineType
	fmt.Printf("  Current pipeline type: %s\n", currentPipelineType)
	fmt.Printf("  Target pipeline type: %s\n", newPipelineType)

	// 更新が必要かどうかを判定
	needsUpdate := shouldUpdatePipelineType(pipeline)

	if dryRun {
		fmt.Println("  DRY RUN - Showing current and expected pipeline type:")
		err := showPipelineTypeDiff(pipeline)
		if err != nil {
			fmt.Printf("  Error generating diff: %v\n", err)
		}

		if !needsUpdate {
			fmt.Println("  Pipeline type is already properly configured, no changes needed.")
		} else {
			fmt.Println("  Changes would be made to fix pipeline type.")
		}
		return nil
	}

	if !needsUpdate {
		fmt.Println("  Pipeline type is already properly configured, skipping...")
		// Cache is maintained for skipped pipelines (no cache deletion)
		return nil
	}

	// パイプラインを更新
	fmt.Println("  Updating pipeline type...")
	err := awsClient.UpdatePipelineTypeV2(ctx, pipeline.GetName(), newPipelineType)
	if err != nil {
		return fmt.Errorf("failed to update pipeline type: %w", err)
	}

	fmt.Println("  Pipeline type updated successfully")

	// Delete cache for updated pipeline
	if fileCache != nil {
		fmt.Println("  Clearing cache for updated pipeline...")
		if err := fileCache.DeletePipelineCache(pipeline.GetName()); err != nil {
			fmt.Printf("  Warning: Failed to clear cache for pipeline %s: %v\n", pipeline.GetName(), err)
			// Don't return error as cache deletion failure shouldn't fail the main operation
		} else {
			fmt.Println("  Pipeline cache cleared successfully")
		}
	}

	return nil
}

func shouldUpdatePipelineType(pipeline *models.Pipeline) bool {
	currentPipelineType := pipeline.Pipeline.PipelineType

	// 現在のPipelineTypeと新しいPipelineTypeが異なる場合は更新が必要
	return currentPipelineType != newPipelineType
}

func showPipelineTypeDiff(pipeline *models.Pipeline) error {
	currentPipelineType := pipeline.Pipeline.PipelineType

	// Check if there are any differences
	hasDifferences := shouldUpdatePipelineType(pipeline)

	if !hasDifferences {
		fmt.Printf("  %s\n", utils.Success("✓ No changes needed - pipeline type is already properly configured"))
		return nil
	}

	fmt.Printf("\n    %s\n", utils.Highlight("=== PIPELINE TYPE DIFF ==="))
	fmt.Printf("    %s: %s\n", "Current PipelineType", utils.Colorize(utils.Red, currentPipelineType))
	fmt.Printf("    %s: %s\n", "New PipelineType", utils.Colorize(utils.Green, newPipelineType))

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
		"pipelineType": newPipelineType,
	}
	newJSON, err := json.MarshalIndent(newConfig, "      ", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal new configuration: %w", err)
	}
	fmt.Printf("      %s\n", utils.Colorize(utils.Green, string(newJSON)))

	return nil
}

func validatePipelineTypeFlags() error {
	// PipelineTypeが指定されているかチェック
	if newPipelineType == "" {
		return fmt.Errorf("pipeline-type is required")
	}

	// PipelineTypeの値をチェック
	validPipelineTypes := []string{"V1", "V2"}
	isValidPipelineType := false
	for _, validType := range validPipelineTypes {
		if strings.EqualFold(newPipelineType, validType) {
			// 大文字小文字を統一
			newPipelineType = validType
			isValidPipelineType = true
			break
		}
	}

	if !isValidPipelineType {
		return fmt.Errorf("invalid pipeline-type '%s'. Valid options: %v", newPipelineType, validPipelineTypes)
	}

	return nil
}

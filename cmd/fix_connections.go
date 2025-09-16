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

var fixConnectionsCmd = &cobra.Command{
	Use:   "fix-connections",
	Short: "Fix pipeline source action ConnectionArn values",
	Long: `Analyze pipeline stages and source actions to fix ConnectionArn values.
Update ConnectionArn for CodeStarSourceConnection actions in pipeline source stages.`,
	RunE: runFixConnections,
}

var (
	fixConnectionsPipeline string
	fixConnectionsDryRun   bool
	newConnectionArn       string
	sourceActionName       string
)

func init() {
	rootCmd.AddCommand(fixConnectionsCmd)

	fixConnectionsCmd.Flags().StringVar(&fixConnectionsPipeline, "pipeline", "", "Pipeline name to fix (if not specified, all pipelines will be processed)")
	fixConnectionsCmd.Flags().BoolVar(&fixConnectionsDryRun, "dry-run", false, "Show what would be changed without making actual changes")
	fixConnectionsCmd.Flags().StringVar(&newConnectionArn, "connection-arn", "", "New ConnectionArn to set for source actions")
	fixConnectionsCmd.Flags().StringVar(&sourceActionName, "source-action", "", "Specific source action name to update (if not specified, all source actions will be updated)")

	// Mark connection-arn as required
	if err := fixConnectionsCmd.MarkFlagRequired("connection-arn"); err != nil {
		fmt.Printf("Error marking connection-arn flag as required: %v\n", err)
	}
}

func runFixConnections(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// 入力検証
	if err := validateConnectionFlags(); err != nil {
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

	// キャッシュ初期化（AWS接続情報ごとに分離）
	awsContext := cache.NewAWSContext(cfg.AWS.Region, cfg.AWS.Profile)
	fileCache := cache.NewFileCacheWithContext(cfg.Cache.Directory, awsContext)

	// AFT マネージャー初期化
	manager := aft.NewManager(awsClient, fileCache, cfg)

	if fixConnectionsPipeline != "" {
		// 特定のパイプラインを処理
		return processSinglePipelineConnections(ctx, manager, awsClient, fixConnectionsPipeline, fixConnectionsDryRun, fileCache)
	} else {
		// すべてのパイプラインを処理
		return processAllPipelinesConnections(ctx, manager, awsClient, fixConnectionsDryRun, fileCache)
	}
}

func processSinglePipelineConnections(ctx context.Context, manager *aft.Manager, awsClient *aws.Client, pipelineName string, dryRun bool, fileCache *cache.FileCache) error {
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

	return fixPipelineConnectionsWithCache(ctx, awsClient, pipeline, dryRun, fileCache)
}

func processAllPipelinesConnections(ctx context.Context, manager *aft.Manager, awsClient *aws.Client, dryRun bool, fileCache *cache.FileCache) error {
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
		if err := fixPipelineConnectionsWithCache(ctx, awsClient, &pipeline, dryRun, fileCache); err != nil {
			errors = append(errors, fmt.Errorf("failed to fix connections for %s: %w", pipeline.GetName(), err))
			fmt.Printf("Error: %v\n", err)
		}
	}

	if len(errors) > 0 {
		fmt.Printf("\nCompleted with %d errors\n", len(errors))
		for _, err := range errors {
			fmt.Printf("- %v\n", err)
		}
		return fmt.Errorf("fix-connections completed with %d errors", len(errors))
	}

	fmt.Println("\nAll pipelines processed successfully")
	return nil
}

func fixPipelineConnectionsWithCache(ctx context.Context, awsClient *aws.Client, pipeline *models.Pipeline, dryRun bool, fileCache *cache.FileCache) error {
	// Source stageからactionsを分析
	sourceActions := analyzeSourceActionsForConnections(pipeline)
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

	// 更新が必要かどうかを判定
	needsUpdate := shouldUpdateConnections(sourceActions)

	if dryRun {
		fmt.Println("  DRY RUN - Showing current and expected ConnectionArn values:")
		err := showConnectionsDiff(pipeline, sourceActions)
		if err != nil {
			fmt.Printf("  Error generating diff: %v\n", err)
		}

		if !needsUpdate {
			fmt.Println("  ConnectionArn values are already properly configured, no changes needed.")
		} else {
			fmt.Println("  Changes would be made to fix ConnectionArn values.")
		}
		return nil
	}

	if !needsUpdate {
		fmt.Println("  ConnectionArn values are already properly configured, skipping...")
		// Cache is maintained for skipped pipelines (no cache deletion)
		return nil
	}

	// パイプラインを更新
	fmt.Println("  Updating pipeline ConnectionArn values...")
	err := awsClient.UpdatePipelineConnectionsV2(ctx, pipeline.GetName(), newConnectionArn, sourceActionName)
	if err != nil {
		return fmt.Errorf("failed to update pipeline connections: %w", err)
	}

	fmt.Println("  Pipeline ConnectionArn values updated successfully")

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

func analyzeSourceActionsForConnections(pipeline *models.Pipeline) []models.Action {
	var sourceActions []models.Action

	for _, stage := range pipeline.Pipeline.Stages {
		if stage.Name == "Source" {
			for _, action := range stage.Actions {
				// CodeStarSourceConnectionのactionのみを対象とする
				if action.ActionTypeId.Provider == "CodeStarSourceConnection" {
					// 特定のsource actionが指定されている場合はそれのみを対象とする
					if sourceActionName != "" && action.Name != sourceActionName {
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

func shouldUpdateConnections(sourceActions []models.Action) bool {
	for _, action := range sourceActions {
		currentConnectionArn := ""
		if connArn, exists := action.Configuration["ConnectionArn"]; exists {
			if connArnStr, ok := connArn.(string); ok {
				currentConnectionArn = connArnStr
			}
		}

		// 現在のConnectionArnと新しいConnectionArnが異なる場合は更新が必要
		if currentConnectionArn != newConnectionArn {
			return true
		}
	}
	return false
}

func showConnectionsDiff(pipeline *models.Pipeline, sourceActions []models.Action) error {
	// Check if there are any differences
	hasDifferences := shouldUpdateConnections(sourceActions)

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

		if currentConnectionArn != newConnectionArn {
			fmt.Printf("    %s:\n", utils.Warning(fmt.Sprintf("Action: %s", action.Name)))
			fmt.Printf("      %s: %s\n", "Current ConnectionArn", utils.Colorize(utils.Red, currentConnectionArn))
			fmt.Printf("      %s: %s\n", "New ConnectionArn", utils.Colorize(utils.Green, newConnectionArn))
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

		if currentConnectionArn != newConnectionArn {
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
				"ConnectionArn": newConnectionArn,
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

func validateConnectionFlags() error {
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

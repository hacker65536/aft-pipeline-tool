package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/hacker65536/aft-pipeline-tool/internal/aws"
	"github.com/hacker65536/aft-pipeline-tool/internal/cache"
	"github.com/hacker65536/aft-pipeline-tool/internal/config"
	"github.com/hacker65536/aft-pipeline-tool/internal/models"
	"github.com/hacker65536/aft-pipeline-tool/pkg/aft"
)

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update trigger settings for a specific pipeline",
	Long:  `Update Git trigger settings for a specific AFT pipeline.`,
	RunE:  runUpdate,
}

var (
	updatePipeline string
	updateBranch   string
	updateTrigger  string
	updatePolling  bool
	updateDryRun   bool
)

func init() {
	rootCmd.AddCommand(updateCmd)

	updateCmd.Flags().StringVar(&updatePipeline, "pipeline", "", "Pipeline name to update (required)")
	updateCmd.Flags().StringVar(&updateBranch, "branch", "", "Git branch name")
	updateCmd.Flags().StringVar(&updateTrigger, "trigger", "", "Trigger type (webhook, polling, none)")
	updateCmd.Flags().BoolVar(&updatePolling, "polling", false, "Enable polling for source changes")
	updateCmd.Flags().BoolVar(&updateDryRun, "dry-run", false, "Show what would be changed without making actual changes")

	if err := updateCmd.MarkFlagRequired("pipeline"); err != nil {
		fmt.Printf("Error marking pipeline flag as required: %v\n", err)
	}
}

func runUpdate(cmd *cobra.Command, args []string) error {
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

	// キャッシュ初期化（AWS接続情報ごとに分離）
	awsContext := cache.NewAWSContext(cfg.AWS.Region, cfg.AWS.Profile)
	fileCache := cache.NewFileCacheWithContext(cfg.Cache.Directory, awsContext)

	// AFT マネージャー初期化
	manager := aft.NewManager(awsClient, fileCache, cfg)

	// トリガー設定を構築
	settings := models.TriggerSettings{
		Branch:         updateBranch,
		TriggerEnabled: false,
		PollingEnabled: updatePolling,
	}

	// トリガータイプの設定
	switch updateTrigger {
	case "webhook":
		settings.TriggerEnabled = true
		settings.PollingEnabled = false
	case "polling":
		settings.TriggerEnabled = false
		settings.PollingEnabled = true
	case "none":
		settings.TriggerEnabled = false
		settings.PollingEnabled = false
	case "":
		// トリガータイプが指定されていない場合はpollingフラグを使用
	default:
		return fmt.Errorf("invalid trigger type: %s (valid options: webhook, polling, none)", updateTrigger)
	}

	if updateDryRun {
		fmt.Println("DRY RUN - No changes will be made")
		fmt.Printf("Pipeline: %s\n", updatePipeline)
		if updateBranch != "" {
			fmt.Printf("Branch: %s\n", updateBranch)
		}
		fmt.Printf("Trigger Enabled: %t\n", settings.TriggerEnabled)
		fmt.Printf("Polling Enabled: %t\n", settings.PollingEnabled)
		return nil
	}

	// パイプライン更新実行
	fmt.Printf("Updating pipeline: %s\n", updatePipeline)
	if err := manager.UpdatePipelineTrigger(ctx, updatePipeline, settings); err != nil {
		return fmt.Errorf("failed to update pipeline: %w", err)
	}

	fmt.Println("Pipeline updated successfully")
	return nil
}

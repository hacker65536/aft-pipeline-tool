package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/hacker65536/aft-pipeline-tool/internal/aws"
	"github.com/hacker65536/aft-pipeline-tool/internal/cache"
	"github.com/hacker65536/aft-pipeline-tool/internal/config"
	"github.com/hacker65536/aft-pipeline-tool/internal/models"
	"github.com/hacker65536/aft-pipeline-tool/pkg/aft"
)

var batchUpdateCmd = &cobra.Command{
	Use:   "batch-update",
	Short: "Update trigger settings for multiple pipelines",
	Long:  `Update Git trigger settings for multiple AFT pipelines across accounts.`,
	RunE:  runBatchUpdate,
}

var (
	batchUpdateAccounts string
	batchUpdateBranch   string
	batchUpdateTrigger  string
	batchUpdatePolling  bool
	batchUpdateDryRun   bool
)

func init() {
	rootCmd.AddCommand(batchUpdateCmd)

	batchUpdateCmd.Flags().StringVar(&batchUpdateAccounts, "accounts", "", "Comma-separated list of account IDs to target (empty for all)")
	batchUpdateCmd.Flags().StringVar(&batchUpdateBranch, "branch", "", "Git branch name")
	batchUpdateCmd.Flags().StringVar(&batchUpdateTrigger, "trigger", "", "Trigger type (webhook, polling, none)")
	batchUpdateCmd.Flags().BoolVar(&batchUpdatePolling, "polling", false, "Enable polling for source changes")
	batchUpdateCmd.Flags().BoolVar(&batchUpdateDryRun, "dry-run", false, "Show what would be changed without making actual changes")
}

func runBatchUpdate(cmd *cobra.Command, args []string) error {
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

	// 対象アカウントの解析
	var targetAccounts []string
	if batchUpdateAccounts != "" {
		targetAccounts = strings.Split(batchUpdateAccounts, ",")
		for i, account := range targetAccounts {
			targetAccounts[i] = strings.TrimSpace(account)
		}
	}

	// トリガー設定を構築
	settings := models.TriggerSettings{
		Branch:         batchUpdateBranch,
		TriggerEnabled: false,
		PollingEnabled: batchUpdatePolling,
	}

	// トリガータイプの設定
	switch batchUpdateTrigger {
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
		return fmt.Errorf("invalid trigger type: %s (valid options: webhook, polling, none)", batchUpdateTrigger)
	}

	// 設定からドライランフラグを取得（コマンドラインフラグで上書き可能）
	dryRun := batchUpdateDryRun
	if !dryRun && cfg.BatchUpdate.DryRun {
		dryRun = true
	}

	// バッチ更新実行
	fmt.Printf("Starting batch update...\n")
	if len(targetAccounts) > 0 {
		fmt.Printf("Target accounts: %s\n", strings.Join(targetAccounts, ", "))
	} else {
		fmt.Printf("Target: All accounts\n")
	}

	if err := manager.BatchUpdateTriggers(ctx, targetAccounts, settings, dryRun); err != nil {
		return fmt.Errorf("batch update failed: %w", err)
	}

	if !dryRun {
		fmt.Println("Batch update completed successfully")
	}

	return nil
}

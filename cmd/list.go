package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/hacker65536/aft-pipeline-tool/internal/aws"
	"github.com/hacker65536/aft-pipeline-tool/internal/cache"
	"github.com/hacker65536/aft-pipeline-tool/internal/config"
	"github.com/hacker65536/aft-pipeline-tool/internal/models"
	"github.com/hacker65536/aft-pipeline-tool/internal/utils"
	"github.com/hacker65536/aft-pipeline-tool/pkg/aft"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List AFT pipelines and their Git trigger settings",
	Long:  `List all AFT pipelines across accounts and display their current Git trigger configurations.`,
	RunE:  runList,
}

var (
	listFormat        string
	listAccountFilter string
	listShowDetails   bool
)

func init() {
	rootCmd.AddCommand(listCmd)

	listCmd.Flags().StringVarP(&listFormat, "format", "f", "table", "Output format (table, json, csv)")
	listCmd.Flags().StringVar(&listAccountFilter, "account-filter", "", "Filter by account ID pattern")
	listCmd.Flags().BoolVar(&listShowDetails, "show-details", false, "Show detailed information (Account ID, Pipeline Type, Trigger, Last Updated)")
}

func runList(cmd *cobra.Command, args []string) error {
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

	// キャッシュ初期化
	fileCache := cache.NewFileCache(cfg.Cache.Directory)

	// AFT マネージャー初期化
	manager := aft.NewManager(awsClient, fileCache, cfg)

	// パイプライン詳細取得（--debugフラグに応じて進捗表示を切り替え）
	var pipelines []models.Pipeline

	if debug {
		// --debug指定時：詳細な進捗表示
		pipelines, err = manager.GetPipelineDetailsWithDetailedProgress(ctx)
	} else {
		// デフォルト：簡易な進捗表示
		pipelines, err = manager.GetPipelineDetailsWithProgress(ctx, true)
	}
	if err != nil {
		return fmt.Errorf("failed to get pipeline details: %w", err)
	}

	// フィルタリング
	if listAccountFilter != "" {
		pipelines = utils.FilterPipelinesByAccount(pipelines, listAccountFilter)
	}

	// 出力フォーマットを設定から取得（フラグで上書き可能）
	format := listFormat
	if format == "table" && cfg.Output.Format != "" {
		format = cfg.Output.Format
	}

	// キャッシュ使用状況を取得
	cacheUsage := manager.GetCacheUsage()

	// キャッシュディレクトリと使用状況を表示
	fmt.Printf("%s: %s\n", utils.Info("Cache Directory"), utils.Highlight(cfg.Cache.Directory))
	fmt.Printf("%s: Accounts=%s, Pipelines=%s, pipelineDetails=%s\n",
		utils.Info("Cache Status"),
		utils.GetCacheStatusColorText(cacheUsage.AccountsFromCache),
		utils.GetCacheStatusColorText(cacheUsage.PipelinesFromCache),
		utils.GetCacheStatusColorText(cacheUsage.PipelineDetailsFromCache))
	fmt.Println()

	// 出力
	formatter := utils.NewFormatterWithOptions(format, listShowDetails, false)
	if err := formatter.FormatPipelines(pipelines, os.Stdout); err != nil {
		return fmt.Errorf("failed to format output: %w", err)
	}

	return nil
}

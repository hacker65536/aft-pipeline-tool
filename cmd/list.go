package cmd

import (
	"context"
	"fmt"
	"os"
	"sort"

	"github.com/spf13/cobra"

	"aft-pipeline-tool/internal/aws"
	"aft-pipeline-tool/internal/cache"
	"aft-pipeline-tool/internal/config"
	"aft-pipeline-tool/internal/models"
	"aft-pipeline-tool/internal/utils"
	"aft-pipeline-tool/pkg/aft"
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
	listWithState     bool
	listShowDetails   bool
)

func init() {
	rootCmd.AddCommand(listCmd)

	listCmd.Flags().StringVarP(&listFormat, "format", "f", "table", "Output format (table, json, csv)")
	listCmd.Flags().StringVar(&listAccountFilter, "account-filter", "", "Filter by account ID pattern")
	listCmd.Flags().BoolVar(&listWithState, "with-state", false, "Include pipeline state information")
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
	var shouldShowState bool

	if listWithState {
		// --with-state指定時：state情報を含む詳細取得
		shouldShowState = true
		if debug {
			// --debug指定時：詳細な進捗表示
			pipelines, err = manager.GetPipelineDetailsWithStateAndDetailedProgress(ctx)
		} else {
			// デフォルト：簡易な進捗表示
			pipelines, err = manager.GetPipelineDetailsWithStateAndProgress(ctx, true)
		}
		if err != nil {
			return fmt.Errorf("failed to get pipeline details with state: %w", err)
		}
	} else {
		// --with-stateが指定されていない場合でも、キャッシュされたstate情報があれば使用
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

		// キャッシュされたstate情報があるかチェックし、あれば追加
		pipelines, shouldShowState = addCachedStateIfAvailable(pipelines, fileCache)
	}

	// フィルタリング
	if listAccountFilter != "" {
		pipelines = utils.FilterPipelinesByAccount(pipelines, listAccountFilter)
	}

	// state情報がある場合は、LATEST STAGE UPDATEの新しい順にソート
	if shouldShowState {
		sortPipelinesByLatestStageUpdate(pipelines)
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
	fmt.Printf("%s: Accounts=%s, Pipelines=%s, PipelineDetails=%s\n",
		utils.Info("Cache Status"),
		getCacheStatusColorText(cacheUsage.AccountsFromCache),
		getCacheStatusColorText(cacheUsage.PipelinesFromCache),
		getCacheStatusColorText(cacheUsage.PipelineDetailsFromCache))
	fmt.Println()

	// 出力
	formatter := utils.NewFormatterWithOptions(format, listShowDetails, shouldShowState)
	if err := formatter.FormatPipelines(pipelines, os.Stdout); err != nil {
		return fmt.Errorf("failed to format output: %w", err)
	}

	return nil
}

// addCachedStateIfAvailable checks for cached state information and adds it to pipelines if available
func addCachedStateIfAvailable(pipelines []models.Pipeline, fileCache *cache.FileCache) ([]models.Pipeline, bool) {
	hasAnyState := false

	for i := range pipelines {
		pipelineName := pipelines[i].GetName()

		// キャッシュされたstate情報をチェック
		if cachedState, err := fileCache.GetPipelineState(pipelineName); err == nil {
			// キャッシュからstate情報を取得
			pipelines[i].State = cachedState.State
			hasAnyState = true
		}
	}

	return pipelines, hasAnyState
}

// getCacheStatusText returns human-readable cache status
func getCacheStatusText(fromCache bool) string {
	if fromCache {
		return "Cache"
	}
	return "API"
}

// getCacheStatusColorText returns colored cache status text
func getCacheStatusColorText(fromCache bool) string {
	if fromCache {
		return utils.Success("Cache")
	}
	return utils.Warning("API")
}

// sortPipelinesByLatestStageUpdate sorts pipelines by latest stage update time in descending order (newest first)
func sortPipelinesByLatestStageUpdate(pipelines []models.Pipeline) {
	sort.Slice(pipelines, func(i, j int) bool {
		timeI := pipelines[i].GetLatestStageUpdateTime()
		timeJ := pipelines[j].GetLatestStageUpdateTime()

		// Handle nil cases: pipelines with no stage update time go to the end
		if timeI == nil && timeJ == nil {
			// If both are nil, maintain original order (stable sort)
			return false
		}
		if timeI == nil {
			// i has no time, j should come first
			return false
		}
		if timeJ == nil {
			// j has no time, i should come first
			return true
		}

		// Both have times, sort by newest first (descending order)
		return timeI.After(*timeJ)
	})
}

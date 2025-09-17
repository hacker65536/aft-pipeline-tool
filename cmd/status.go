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

var statusCmd = &cobra.Command{
	Use:   "status [pipeline-name]",
	Short: "Show pipeline status information with execution state",
	Long: `Show pipeline status information including execution state.
When no pipeline name is provided, shows all AFT pipelines with their current execution status.
When a pipeline name is provided, shows detailed status for that specific pipeline.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runStatus,
}

var (
	statusFormat         string
	statusAccountFilter  string
	statusShowDetails    bool
	statusDetails        bool
	statusStateFilter    string
	statusPipelineFilter string
	statusRefreshCache   bool
)

func init() {
	rootCmd.AddCommand(statusCmd)

	statusCmd.Flags().StringVarP(&statusFormat, "format", "f", "table", "Output format (table, json, csv)")
	statusCmd.Flags().StringVar(&statusAccountFilter, "account-filter", "", "Filter by account ID pattern")
	statusCmd.Flags().BoolVar(&statusShowDetails, "show-details", false, "Show detailed information (Account ID, Pipeline Type, Trigger, Last Updated)")
	statusCmd.Flags().BoolVar(&statusDetails, "details", false, "Show detailed execution information (for individual pipeline)")
	statusCmd.Flags().StringVar(&statusStateFilter, "state-filter", "", "Filter by pipeline state (Succeeded, Failed, InProgress, etc.)")
	statusCmd.Flags().StringVar(&statusPipelineFilter, "pipeline-filter", "", "Filter by pipeline name, account ID, or account name")
	statusCmd.Flags().BoolVar(&statusRefreshCache, "refresh-cache", false, "Refresh cache before showing status")
}

func runStatus(cmd *cobra.Command, args []string) error {
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
	// キャッシュ初期化（AWS接続情報ごとに分離）
	awsContext := cache.NewAWSContext(cfg.AWS.Region, cfg.AWS.Profile)
	fileCache := cache.NewFileCacheWithContext(cfg.Cache.Directory, awsContext)

	// AFT マネージャー初期化
	manager := aft.NewManager(awsClient, fileCache, cfg)

	if len(args) > 0 {
		// 個別パイプラインの詳細状態確認
		pipelineName := args[0]

		// キャッシュ更新が要求された場合（特定パイプラインのみ）
		if statusRefreshCache {
			fmt.Printf("%s Refreshing cache for pipeline: %s\n", utils.Info("INFO"), utils.Highlight(pipelineName))
			if err := fileCache.DeletePipelineCache(pipelineName); err != nil {
				fmt.Printf("%s Warning: Failed to clear pipeline cache: %v\n", utils.Warning("WARNING"), err)
			} else {
				fmt.Printf("%s Pipeline cache refreshed successfully\n", utils.Success("SUCCESS"))
			}
		}

		return showIndividualPipelineStatus(ctx, awsClient, pipelineName, statusDetails)
	}

	// 全パイプライン一覧と実行状態を表示
	return showAllPipelinesStatus(ctx, manager, cfg, fileCache)
}

func showIndividualPipelineStatus(ctx context.Context, client *aws.Client, pipelineName string, detailed bool) error {
	fmt.Printf("%s %s\n", utils.Info("Pipeline Status:"), utils.Highlight(pipelineName))

	// Get latest execution
	executions, err := client.ListPipelineExecutions(ctx, pipelineName, 1)
	if err != nil {
		return fmt.Errorf("failed to list executions: %w", err)
	}

	if len(executions.PipelineExecutionSummaries) == 0 {
		fmt.Printf("  %s No executions found\n", utils.Info("INFO"))
		return nil
	}

	latest := executions.PipelineExecutionSummaries[0]
	status := formatExecutionStatus(string(latest.Status))

	fmt.Printf("  Status: %s\n", status)
	fmt.Printf("  Execution ID: %s\n", *latest.PipelineExecutionId)

	if latest.StartTime != nil {
		fmt.Printf("  Started: %s\n", latest.StartTime.Local().Format("2006-01-02 15:04:05-0700"))
	}

	if latest.LastUpdateTime != nil {
		fmt.Printf("  Last Updated: %s\n", latest.LastUpdateTime.Local().Format("2006-01-02 15:04:05-0700"))
	}

	if detailed {
		execution, err := client.GetPipelineExecution(ctx, pipelineName, *latest.PipelineExecutionId)
		if err != nil {
			return fmt.Errorf("failed to get execution details: %w", err)
		}

		if execution.PipelineExecution.StatusSummary != nil && *execution.PipelineExecution.StatusSummary != "" {
			fmt.Printf("  Summary: %s\n", *execution.PipelineExecution.StatusSummary)
		}

		// Get pipeline state for stage-level details
		state, err := client.GetPipelineState(ctx, pipelineName)
		if err != nil {
			fmt.Printf("  %s Failed to get pipeline state: %v\n", utils.Warning("WARNING"), err)
		} else {
			fmt.Printf("  Stages:\n")
			for _, stage := range state.StageStates {
				stageStatus := "Unknown"
				if stage.LatestExecution != nil {
					stageStatus = string(stage.LatestExecution.Status)
				}
				fmt.Printf("    %s: %s\n", *stage.StageName, formatExecutionStatus(stageStatus))
			}
		}
	}

	return nil
}

// performSelectiveCacheRefresh performs selective cache refresh for filtered pipelines
func performSelectiveCacheRefresh(ctx context.Context, manager *aft.Manager, fileCache cache.Cache) error {
	fmt.Printf("%s Analyzing existing cache and identifying target pipelines...\n", utils.Info("INFO"))

	// Step 1: 既存のキャッシュ状況を確認
	cacheStats := analyzeCacheStatus(fileCache)
	fmt.Printf("%s Current cache status: %s\n", utils.Info("INFO"), cacheStats)

	// Step 2: 現在のパイプライン詳細を取得（AccountName情報が必要なため）
	// showAllPipelinesStatusと同じメソッドを使用して一貫性を保つ
	pipelines, err := manager.GetPipelineDetailsWithStateAndDetailedProgress(ctx)
	if err != nil {
		return fmt.Errorf("failed to get pipeline details: %w", err)
	}

	// Step 3: フィルタリングを適用してターゲットパイプラインを特定
	originalCount := len(pipelines)
	if statusAccountFilter != "" {
		pipelines = utils.FilterPipelinesByAccount(pipelines, statusAccountFilter)
	}
	if statusPipelineFilter != "" {
		pipelines = utils.FilterPipelinesByNameAccountIDOrName(pipelines, statusPipelineFilter)
	}

	if len(pipelines) == 0 {
		fmt.Printf("%s No pipelines match the filter criteria\n", utils.Warning("WARNING"))
		return nil
	}

	fmt.Printf("%s Found %d target pipeline(s) out of %d total pipelines\n",
		utils.Info("INFO"), len(pipelines), originalCount)

	// Step 4: ターゲットパイプラインの詳細キャッシュのみを削除（pipelines.jsonは保持）
	fmt.Printf("%s Refreshing cache for %d filtered pipeline(s)...\n", utils.Info("INFO"), len(pipelines))

	refreshedCount := 0
	errorCount := 0

	for i, pipeline := range pipelines {
		pipelineName := pipeline.GetName()

		// 個別のパイプライン詳細キャッシュのみを削除（pipelines.jsonは保持）
		if err := deletePipelineDetailCacheOnly(fileCache, pipelineName); err != nil {
			fmt.Printf("%s Warning: Failed to clear cache for pipeline %s: %v\n",
				utils.Warning("WARNING"), pipelineName, err)
			errorCount++
		} else {
			refreshedCount++
		}

		// 進捗表示
		if i%10 == 9 || i == len(pipelines)-1 {
			fmt.Printf("\r%s Processing: %d/%d pipelines (refreshed: %d, errors: %d)",
				utils.Info("INFO"), i+1, len(pipelines), refreshedCount, errorCount)
		}
	}

	fmt.Printf("\n%s Cache refresh completed: %d pipelines refreshed",
		utils.Success("SUCCESS"), refreshedCount)

	if errorCount > 0 {
		fmt.Printf(" (%d errors)", errorCount)
	}
	fmt.Println()

	// Step 5: 削除されたパイプライン名をログ出力
	fmt.Printf("%s Cache deletion completed for filtered pipelines:\n", utils.Info("INFO"))
	for _, pipeline := range pipelines {
		fmt.Printf("  - %s\n", utils.Highlight(pipeline.GetName()))
	}
	fmt.Printf("%s Next data access will fetch latest information from AWS API\n", utils.Success("SUCCESS"))

	return nil
}

// deletePipelineDetailCacheOnly deletes only pipeline detail and state cache files, preserving pipelines.json
func deletePipelineDetailCacheOnly(fileCache cache.Cache, pipelineName string) error {
	// 重要：pipelines.jsonは保持し、個別のキャッシュファイルのみを削除する
	// これにより、次回のGetPipelineDetailsWithProgress呼び出し時に
	// AWS APIから最新データを取得させる

	// パイプライン詳細キャッシュが存在するかチェック
	if _, err := fileCache.GetPipelineDetail(pipelineName); err != nil {
		// キャッシュが存在しない場合は何もしない
		return nil
	}

	// 個別キャッシュファイルを直接削除（pipelines.jsonは保持）
	// これは少しハックですが、確実にキャッシュを無効化する方法です

	// キャッシュのTTLを0に設定することで期限切れにする方法を試す
	// または、直接ファイルシステムから削除する

	// 一時的にDeletePipelineCacheを使用するが、その後pipelines.jsonを復元する
	// まず現在のpipelines.jsonの内容を保存
	pipelinesCache, err := fileCache.GetPipelines()
	if err != nil {
		return fmt.Errorf("failed to get pipelines cache: %w", err)
	}

	// 個別キャッシュを削除
	if err := fileCache.DeletePipelineCache(pipelineName); err != nil {
		return fmt.Errorf("failed to delete pipeline cache for %s: %w", pipelineName, err)
	}

	// pipelines.jsonを復元（TTLは元のまま）
	if err := fileCache.SetPipelines(pipelinesCache.Pipelines, pipelinesCache.TTL); err != nil {
		return fmt.Errorf("failed to restore pipelines cache: %w", err)
	}

	return nil
}

// analyzeCacheStatus analyzes current cache status and returns a summary string
func analyzeCacheStatus(fileCache cache.Cache) string {
	var status []string

	// アカウントキャッシュをチェック
	if _, err := fileCache.GetAccounts(); err == nil {
		status = append(status, "Accounts✓")
	} else {
		status = append(status, "Accounts✗")
	}

	// パイプラインキャッシュをチェック
	if _, err := fileCache.GetPipelines(); err == nil {
		status = append(status, "Pipelines✓")
	} else {
		status = append(status, "Pipelines✗")
	}

	if len(status) == 0 {
		return "No cache available"
	}

	return fmt.Sprintf("%s", status)
}

func showAllPipelinesStatus(ctx context.Context, manager *aft.Manager, cfg *config.Config, fileCache cache.Cache) error {
	// キャッシュ更新処理
	if statusRefreshCache {
		if statusPipelineFilter != "" || statusAccountFilter != "" {
			// フィルターが指定されている場合、選択的キャッシュ更新を実行
			err := performSelectiveCacheRefresh(ctx, manager, fileCache)
			if err != nil {
				return fmt.Errorf("failed to perform selective cache refresh: %w", err)
			}
		} else {
			// フィルターが指定されていない場合は全キャッシュを更新
			fmt.Printf("%s Refreshing all cache...\n", utils.Info("INFO"))
			if err := fileCache.ClearCache(); err != nil {
				return fmt.Errorf("failed to clear cache: %w", err)
			}
			fmt.Printf("%s All cache refreshed successfully\n", utils.Success("SUCCESS"))
		}
	}

	// パイプライン詳細取得（state情報を含む詳細取得を常に実行）
	var pipelines []models.Pipeline

	var err error
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

	// フィルタリング
	if statusAccountFilter != "" {
		pipelines = utils.FilterPipelinesByAccount(pipelines, statusAccountFilter)
	}

	// 新しいフィルタリング機能
	if statusPipelineFilter != "" {
		pipelines = utils.FilterPipelinesByNameAccountIDOrName(pipelines, statusPipelineFilter)
	}

	if statusStateFilter != "" {
		pipelines = utils.FilterPipelinesByState(pipelines, statusStateFilter)
	}

	// LATEST STAGE UPDATEの新しい順でのソート
	utils.SortPipelinesByLatestStageUpdate(pipelines)

	// 出力フォーマットを設定から取得（フラグで上書き可能）
	format := statusFormat
	if format == "table" && cfg.Output.Format != "" {
		format = cfg.Output.Format
	}

	// キャッシュ使用状況を取得
	cacheUsage := manager.GetCacheUsage()

	// キャッシュディレクトリと使用状況を表示
	fmt.Printf("%s: %s\n", utils.Info("Cache Directory"), utils.Highlight(cfg.Cache.Directory))
	fmt.Printf("%s: Accounts=%s, Pipelines=%s, PipelineDetails=%s, PipelineStates=%s\n",
		utils.Info("Cache Status"),
		utils.GetCacheStatusColorText(cacheUsage.AccountsFromCache),
		utils.GetCacheStatusColorText(cacheUsage.PipelinesFromCache),
		utils.GetCacheStatusColorText(cacheUsage.PipelineDetailsFromCache),
		utils.GetCacheStatusColorText(cacheUsage.PipelineStatesFromCache))
	fmt.Println()

	// 出力（state情報を常に含める）
	formatter := utils.NewFormatterWithOptions(format, statusShowDetails, true)
	if err := formatter.FormatPipelines(pipelines, os.Stdout); err != nil {
		return fmt.Errorf("failed to format output: %w", err)
	}

	return nil
}

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
	statusFormat        string
	statusAccountFilter string
	statusShowDetails   bool
	statusDetails       bool
)

func init() {
	rootCmd.AddCommand(statusCmd)

	statusCmd.Flags().StringVarP(&statusFormat, "format", "f", "table", "Output format (table, json, csv)")
	statusCmd.Flags().StringVar(&statusAccountFilter, "account-filter", "", "Filter by account ID pattern")
	statusCmd.Flags().BoolVar(&statusShowDetails, "show-details", false, "Show detailed information (Account ID, Pipeline Type, Trigger, Last Updated)")
	statusCmd.Flags().BoolVar(&statusDetails, "details", false, "Show detailed execution information (for individual pipeline)")
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
		return showIndividualPipelineStatus(ctx, awsClient, pipelineName, statusDetails)
	}

	// 全パイプライン一覧と実行状態を表示
	return showAllPipelinesStatus(ctx, manager, cfg)
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
		fmt.Printf("  Started: %s\n", latest.StartTime.Format("2006-01-02 15:04:05"))
	}

	if latest.LastUpdateTime != nil {
		fmt.Printf("  Last Updated: %s\n", latest.LastUpdateTime.Format("2006-01-02 15:04:05"))
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

func showAllPipelinesStatus(ctx context.Context, manager *aft.Manager, cfg *config.Config) error {
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

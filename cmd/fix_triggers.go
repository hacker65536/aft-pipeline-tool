package cmd

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/hacker65536/aft-pipeline-tool/internal/pipeline"
)

var fixTriggersCmd = &cobra.Command{
	Use:   "fix-triggers",
	Short: "Fix pipeline triggers based on source actions",
	Long: `Analyze pipeline stages and source actions to fix triggers.
If triggers are missing, they will be added based on the source action names.`,
	RunE: runFixTriggers,
}

var (
	fixTriggersPipeline string
	fixTriggersDryRun   bool
	triggerMode         string
	sourceAction        string
)

func init() {
	rootCmd.AddCommand(fixTriggersCmd)

	fixTriggersCmd.Flags().StringVar(&fixTriggersPipeline, "pipeline", "", "Pipeline name to fix (if not specified, all pipelines will be processed)")
	fixTriggersCmd.Flags().BoolVar(&fixTriggersDryRun, "dry-run", false, "Show what would be changed without making actual changes")
	fixTriggersCmd.Flags().StringVar(&triggerMode, "trigger-mode", "auto", "Trigger configuration mode: 'auto' (both sources), 'single' (one source), 'none' (remove all triggers)")
	fixTriggersCmd.Flags().StringVar(&sourceAction, "source-action", "", "Specific source action name when using 'single' mode (aft-global-customizations or aft-account-customizations)")
}

func runFixTriggers(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// 入力検証
	if err := pipeline.ValidateTriggerModeFlags(triggerMode, sourceAction); err != nil {
		return err
	}

	// 共通セットアップ
	setup, err := pipeline.NewCommonSetup(ctx)
	if err != nil {
		return err
	}

	// プロセッサー作成
	processor := pipeline.NewTriggersProcessor(triggerMode, sourceAction)
	baseProcessor := pipeline.NewBaseFixProcessor(processor)

	if fixTriggersPipeline != "" {
		// 特定のパイプラインを処理
		return baseProcessor.ProcessSinglePipeline(ctx, setup.Manager, setup.AWSClient, fixTriggersPipeline, fixTriggersDryRun, setup.FileCache)
	} else {
		// すべてのパイプラインを処理
		return baseProcessor.ProcessAllPipelines(ctx, setup.Manager, setup.AWSClient, fixTriggersDryRun, setup.FileCache)
	}
}

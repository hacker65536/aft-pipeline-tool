package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/hacker65536/aft-pipeline-tool/internal/pipeline"
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
	if err := pipeline.ValidatePipelineTypeFlags(&newPipelineType); err != nil {
		return err
	}

	// 共通セットアップ
	setup, err := pipeline.NewCommonSetup(ctx)
	if err != nil {
		return err
	}

	// プロセッサー作成
	processor := pipeline.NewPipelineTypeProcessor(newPipelineType)
	baseProcessor := pipeline.NewBaseFixProcessor(processor)

	if fixPipelineTypePipeline != "" {
		// 特定のパイプラインを処理
		return baseProcessor.ProcessSinglePipeline(ctx, setup.Manager, setup.AWSClient, fixPipelineTypePipeline, fixPipelineTypeDryRun, setup.FileCache)
	} else {
		// すべてのパイプラインを処理
		return baseProcessor.ProcessAllPipelines(ctx, setup.Manager, setup.AWSClient, fixPipelineTypeDryRun, setup.FileCache)
	}
}

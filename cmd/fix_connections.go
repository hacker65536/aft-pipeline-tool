package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/hacker65536/aft-pipeline-tool/internal/pipeline"
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
	if err := pipeline.ValidateConnectionFlags(newConnectionArn, sourceActionName); err != nil {
		return err
	}

	// 共通セットアップ
	setup, err := pipeline.NewCommonSetup(ctx)
	if err != nil {
		return err
	}

	// プロセッサー作成
	processor := pipeline.NewConnectionsProcessor(newConnectionArn, sourceActionName)
	baseProcessor := pipeline.NewBaseFixProcessor(processor)

	if fixConnectionsPipeline != "" {
		// 特定のパイプラインを処理
		return baseProcessor.ProcessSinglePipeline(ctx, setup.Manager, setup.AWSClient, fixConnectionsPipeline, fixConnectionsDryRun, setup.FileCache)
	} else {
		// すべてのパイプラインを処理
		return baseProcessor.ProcessAllPipelines(ctx, setup.Manager, setup.AWSClient, fixConnectionsDryRun, setup.FileCache)
	}
}

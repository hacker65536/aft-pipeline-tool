package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/hacker65536/aft-pipeline-tool/internal/models"
	"github.com/hacker65536/aft-pipeline-tool/internal/pipeline"
)

var testTriggersCmd = &cobra.Command{
	Use:   "test-triggers",
	Short: "Test trigger fix logic using local pipeline JSON file",
	Long: `Test the trigger fix logic using a local pipeline JSON file.
This command is useful for testing the logic without connecting to AWS.`,
	RunE: runTestTriggers,
}

var (
	testTriggersFile   string
	testTriggersDryRun bool
)

func init() {
	rootCmd.AddCommand(testTriggersCmd)

	testTriggersCmd.Flags().StringVar(&testTriggersFile, "file", "pipeline.json", "Pipeline JSON file to test")
	testTriggersCmd.Flags().BoolVar(&testTriggersDryRun, "dry-run", true, "Show what would be changed (always true for test mode)")
	testTriggersCmd.Flags().StringVar(&triggerMode, "trigger-mode", "auto", "Trigger configuration mode: 'auto' (both sources), 'single' (one source), 'none' (remove all triggers)")
	testTriggersCmd.Flags().StringVar(&sourceAction, "source-action", "", "Specific source action name when using 'single' mode (aft-global-customizations or aft-account-customizations)")
}

func runTestTriggers(cmd *cobra.Command, args []string) error {
	fmt.Printf("Testing trigger fix logic with file: %s\n", testTriggersFile)

	// 入力検証
	if err := pipeline.ValidateTriggerModeFlags(triggerMode, sourceAction); err != nil {
		return err
	}

	// JSONファイルを読み込み
	data, err := os.ReadFile(testTriggersFile)
	if err != nil {
		return fmt.Errorf("failed to read file %s: %w", testTriggersFile, err)
	}

	// JSONをパース
	var pipelineData struct {
		Pipeline    models.PipelineDeclaration `json:"pipeline"`
		Metadata    models.PipelineMetadata    `json:"metadata"`
		AccountID   string                     `json:"account_id"`
		AccountName string                     `json:"account_name"`
	}

	if err := json.Unmarshal(data, &pipelineData); err != nil {
		return fmt.Errorf("failed to parse JSON: %w", err)
	}

	// Pipelineオブジェクトを構築
	pipelineObj := &models.Pipeline{
		Pipeline:    pipelineData.Pipeline,
		Metadata:    pipelineData.Metadata,
		AccountID:   pipelineData.AccountID,
		AccountName: pipelineData.AccountName,
	}

	fmt.Printf("Pipeline: %s\n", pipelineObj.Pipeline.Name)
	fmt.Printf("Account ID: %s\n", pipelineObj.AccountID)
	fmt.Printf("Account Name: %s\n", pipelineObj.AccountName)

	// テスト実行
	return testFixPipelineTriggers(pipelineObj, true)
}

func testFixPipelineTriggers(pipelineObj *models.Pipeline, dryRun bool) error {
	fmt.Println("\n=== Testing Trigger Fix Logic ===")

	// プロセッサー作成
	processor := pipeline.NewTriggersProcessor(triggerMode, sourceAction)

	// 分析結果を表示
	fmt.Printf("  Pipeline: %s\n", pipelineObj.Pipeline.Name)
	fmt.Printf("  Current triggers: %d\n", len(pipelineObj.Pipeline.Triggers))

	// 更新が必要かどうかを判定
	needsUpdate := processor.ShouldUpdate(pipelineObj)
	fmt.Printf("  Needs update: %t\n", needsUpdate)

	// 差分を表示
	fmt.Println("\n=== Trigger Diff Analysis ===")
	err := processor.ShowDiff(pipelineObj)
	if err != nil {
		fmt.Printf("  Error generating diff: %v\n", err)
	}

	return nil
}

package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/hacker65536/aft-pipeline-tool/internal/models"
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
	if err := validateTriggerModeFlags(); err != nil {
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
	pipeline := &models.Pipeline{
		Pipeline:    pipelineData.Pipeline,
		Metadata:    pipelineData.Metadata,
		AccountID:   pipelineData.AccountID,
		AccountName: pipelineData.AccountName,
	}

	fmt.Printf("Pipeline: %s\n", pipeline.Pipeline.Name)
	fmt.Printf("Account ID: %s\n", pipeline.AccountID)
	fmt.Printf("Account Name: %s\n", pipeline.AccountName)

	// テスト実行
	return testFixPipelineTriggers(pipeline, true)
}

func testFixPipelineTriggers(pipeline *models.Pipeline, dryRun bool) error {
	fmt.Println("\n=== Testing Trigger Fix Logic ===")

	// Source stageからactionsを分析
	sourceActions := analyzeSourceActions(pipeline)
	if len(sourceActions) == 0 {
		fmt.Println("  No source actions found")
		return nil
	}

	fmt.Printf("  Found %d source actions:\n", len(sourceActions))
	for _, action := range sourceActions {
		fmt.Printf("    - %s (Provider: %s)\n", action.Name, action.ActionTypeId.Provider)
	}

	// 現在のtriggersを分析
	existingTriggers := pipeline.Pipeline.Triggers
	fmt.Printf("\n  Current triggers: %d\n", len(existingTriggers))

	// 新しいtriggersを生成
	newTriggers := generateTriggers(sourceActions, pipeline)
	fmt.Printf("  Generated %d new triggers\n", len(newTriggers))

	// triggersが必要かどうかを判定
	needsTriggers := shouldAddTriggers(sourceActions, existingTriggers, pipeline)

	// 差分を表示（改良版）
	fmt.Println("\n=== Trigger Diff Analysis ===")
	err := showTriggersDiff(pipeline, newTriggers)
	if err != nil {
		fmt.Printf("  Error generating diff: %v\n", err)
	}

	if needsTriggers {
		// AWS API UpdatePipelineInput の内容を出力（変更が必要な場合のみ）
		fmt.Println("\n=== AWS API UpdatePipelineInput Content ===")
		err = showUpdatePipelineInput(pipeline, newTriggers)
		if err != nil {
			fmt.Printf("Error generating UpdatePipelineInput: %v\n", err)
		}
	}

	return nil
}

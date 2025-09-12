package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/hacker65536/aft-pipeline-tool/internal/models"
)

var testConnectionsCmd = &cobra.Command{
	Use:   "test-connections",
	Short: "Test fix-connections functionality with sample pipeline data",
	Long: `Test the fix-connections command using sample pipeline JSON files.
This command helps validate the ConnectionArn update logic without making actual AWS API calls.`,
	RunE: runTestConnections,
}

var (
	testConnectionsFile  string
	testNewConnectionArn string
	testSourceActionName string
)

func init() {
	rootCmd.AddCommand(testConnectionsCmd)

	testConnectionsCmd.Flags().StringVar(&testConnectionsFile, "file", "pipeline.json", "Pipeline JSON file to test with")
	testConnectionsCmd.Flags().StringVar(&testNewConnectionArn, "connection-arn", "", "New ConnectionArn to test with")
	testConnectionsCmd.Flags().StringVar(&testSourceActionName, "source-action", "", "Specific source action name to test (optional)")

	// Mark connection-arn as required
	if err := testConnectionsCmd.MarkFlagRequired("connection-arn"); err != nil {
		fmt.Printf("Error marking connection-arn flag as required: %v\n", err)
	}
}

func runTestConnections(cmd *cobra.Command, args []string) error {
	// ファイルを読み込み
	fmt.Printf("Loading pipeline from file: %s\n", testConnectionsFile)
	data, err := os.ReadFile(testConnectionsFile)
	if err != nil {
		return fmt.Errorf("failed to read file %s: %w", testConnectionsFile, err)
	}

	// JSONをパース
	var pipeline models.Pipeline
	if err := json.Unmarshal(data, &pipeline); err != nil {
		return fmt.Errorf("failed to parse JSON: %w", err)
	}

	fmt.Printf("Pipeline: %s\n", pipeline.GetName())

	// Source actionsを分析
	sourceActions := analyzeSourceActionsForConnectionsTest(&pipeline)
	if len(sourceActions) == 0 {
		fmt.Println("No CodeStarSourceConnection actions found")
		return nil
	}

	fmt.Printf("Found %d CodeStarSourceConnection actions:\n", len(sourceActions))
	for _, action := range sourceActions {
		currentConnectionArn := ""
		if connArn, exists := action.Configuration["ConnectionArn"]; exists {
			if connArnStr, ok := connArn.(string); ok {
				currentConnectionArn = connArnStr
			}
		}
		fmt.Printf("  - %s (current: %s)\n", action.Name, currentConnectionArn)
	}

	// 更新が必要かどうかを判定
	needsUpdate := shouldUpdateConnectionsTest(sourceActions, testNewConnectionArn)
	fmt.Printf("Needs update: %t\n", needsUpdate)

	if needsUpdate {
		fmt.Println("\nChanges that would be made:")
		for _, action := range sourceActions {
			// 特定のsource actionが指定されている場合はそれのみを対象とする
			if testSourceActionName != "" && action.Name != testSourceActionName {
				continue
			}

			currentConnectionArn := ""
			if connArn, exists := action.Configuration["ConnectionArn"]; exists {
				if connArnStr, ok := connArn.(string); ok {
					currentConnectionArn = connArnStr
				}
			}

			if currentConnectionArn != testNewConnectionArn {
				fmt.Printf("  Action: %s\n", action.Name)
				fmt.Printf("    Current: %s\n", currentConnectionArn)
				fmt.Printf("    New:     %s\n", testNewConnectionArn)
			}
		}

		// 更新後のパイプライン構造を表示
		fmt.Println("\nUpdated pipeline structure (simulation):")
		updatedPipeline := simulateConnectionUpdate(&pipeline, testNewConnectionArn, testSourceActionName)
		updatedJSON, err := json.MarshalIndent(updatedPipeline, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal updated pipeline: %w", err)
		}
		fmt.Println(string(updatedJSON))
	} else {
		fmt.Println("No changes needed - ConnectionArn values are already correct")
	}

	return nil
}

func analyzeSourceActionsForConnectionsTest(pipeline *models.Pipeline) []models.Action {
	var sourceActions []models.Action

	for _, stage := range pipeline.Pipeline.Stages {
		if stage.Name == "Source" {
			for _, action := range stage.Actions {
				// CodeStarSourceConnectionのactionのみを対象とする
				if action.ActionTypeId.Provider == "CodeStarSourceConnection" {
					sourceActions = append(sourceActions, action)
				}
			}
			break
		}
	}

	return sourceActions
}

func shouldUpdateConnectionsTest(sourceActions []models.Action, newConnectionArn string) bool {
	for _, action := range sourceActions {
		// 特定のsource actionが指定されている場合はそれのみを対象とする
		if testSourceActionName != "" && action.Name != testSourceActionName {
			continue
		}

		currentConnectionArn := ""
		if connArn, exists := action.Configuration["ConnectionArn"]; exists {
			if connArnStr, ok := connArn.(string); ok {
				currentConnectionArn = connArnStr
			}
		}

		// 現在のConnectionArnと新しいConnectionArnが異なる場合は更新が必要
		if currentConnectionArn != newConnectionArn {
			return true
		}
	}
	return false
}

func simulateConnectionUpdate(pipeline *models.Pipeline, newConnectionArn, sourceActionName string) *models.Pipeline {
	// パイプラインのコピーを作成
	updatedPipeline := *pipeline

	// Source stageのactionsを更新
	for stageIdx, stage := range updatedPipeline.Pipeline.Stages {
		if stage.Name == "Source" {
			for actionIdx, action := range stage.Actions {
				// CodeStarSourceConnectionのactionのみを対象とする
				if action.ActionTypeId.Provider == "CodeStarSourceConnection" {
					// 特定のsource actionが指定されている場合はそれのみを対象とする
					if sourceActionName != "" && action.Name != sourceActionName {
						continue
					}

					// ConnectionArnを更新
					if action.Configuration == nil {
						action.Configuration = make(map[string]interface{})
					}
					action.Configuration["ConnectionArn"] = newConnectionArn
					updatedPipeline.Pipeline.Stages[stageIdx].Actions[actionIdx] = action
				}
			}
			break
		}
	}

	return &updatedPipeline
}

package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"aft-pipeline-tool/internal/aws"
	"aft-pipeline-tool/internal/cache"
	"aft-pipeline-tool/internal/config"
	"aft-pipeline-tool/internal/models"
	"aft-pipeline-tool/internal/utils"
	"aft-pipeline-tool/pkg/aft"
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
	if err := validateTriggerModeFlags(); err != nil {
		return err
	}

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

	if fixTriggersPipeline != "" {
		// 特定のパイプラインを処理
		return processSinglePipeline(ctx, manager, awsClient, fixTriggersPipeline, fixTriggersDryRun, fileCache)
	} else {
		// すべてのパイプラインを処理
		return processAllPipelines(ctx, manager, awsClient, fixTriggersDryRun, fileCache)
	}
}

func processSinglePipeline(ctx context.Context, manager *aft.Manager, awsClient *aws.Client, pipelineName string, dryRun bool, fileCache *cache.FileCache) error {
	fmt.Printf("Processing pipeline: %s\n", pipelineName)

	// パイプライン詳細を取得
	pipeline, err := awsClient.GetPipelineDetails(ctx, pipelineName)
	if err != nil {
		return fmt.Errorf("failed to get pipeline details: %w", err)
	}

	// AccountIDを抽出
	accountID := extractAccountIDFromPipelineName(pipelineName)
	pipeline.AccountID = accountID

	// AccountNameを設定するためにアカウント一覧を取得
	accounts, err := manager.GetAccounts(ctx)
	if err != nil {
		return fmt.Errorf("failed to get accounts: %w", err)
	}

	// AccountIDからAccountNameを取得
	for _, account := range accounts {
		if account.ID == accountID {
			pipeline.AccountName = account.Name
			break
		}
	}

	// Use the cache instance that's already available
	return fixPipelineTriggersWithCache(ctx, awsClient, pipeline, dryRun, fileCache)
}

func processAllPipelines(ctx context.Context, manager *aft.Manager, awsClient *aws.Client, dryRun bool, fileCache *cache.FileCache) error {
	fmt.Println("Processing all AFT pipelines...")

	// すべてのパイプライン詳細を取得
	pipelines, err := manager.GetPipelineDetails(ctx)
	if err != nil {
		return fmt.Errorf("failed to get pipeline details: %w", err)
	}

	fmt.Printf("Found %d pipelines to process\n", len(pipelines))

	var errors []error
	for _, pipeline := range pipelines {
		fmt.Printf("\nProcessing pipeline: %s\n", pipeline.GetName())
		if err := fixPipelineTriggersWithCache(ctx, awsClient, &pipeline, dryRun, fileCache); err != nil {
			errors = append(errors, fmt.Errorf("failed to fix triggers for %s: %w", pipeline.GetName(), err))
			fmt.Printf("Error: %v\n", err)
		}
	}

	if len(errors) > 0 {
		fmt.Printf("\nCompleted with %d errors\n", len(errors))
		for _, err := range errors {
			fmt.Printf("- %v\n", err)
		}
		return fmt.Errorf("fix-triggers completed with %d errors", len(errors))
	}

	fmt.Println("\nAll pipelines processed successfully")
	return nil
}

func fixPipelineTriggers(ctx context.Context, awsClient *aws.Client, pipeline *models.Pipeline, dryRun bool) error {
	return fixPipelineTriggersWithCache(ctx, awsClient, pipeline, dryRun, nil)
}

func fixPipelineTriggersWithCache(ctx context.Context, awsClient *aws.Client, pipeline *models.Pipeline, dryRun bool, fileCache *cache.FileCache) error {
	// Source stageからactionsを分析
	sourceActions := analyzeSourceActions(pipeline)
	if len(sourceActions) == 0 {
		fmt.Println("  No source actions found, skipping...")
		return nil
	}

	fmt.Printf("  Found %d source actions:\n", len(sourceActions))
	for _, action := range sourceActions {
		fmt.Printf("    - %s\n", action.Name)
	}

	// 現在のtriggersを分析
	existingTriggers := pipeline.Pipeline.Triggers
	fmt.Printf("  Current triggers: %d\n", len(existingTriggers))

	// 新しいtriggersを生成（比較のため）
	newTriggers := generateTriggers(sourceActions, pipeline)
	fmt.Printf("  Generated %d new triggers\n", len(newTriggers))

	// triggersが必要かどうかを判定
	needsTriggers := shouldAddTriggers(sourceActions, existingTriggers, pipeline)

	if dryRun {
		fmt.Println("  DRY RUN - Showing current and expected triggers:")
		err := showTriggersDiff(pipeline, newTriggers)
		if err != nil {
			fmt.Printf("  Error generating diff: %v\n", err)
		}

		if !needsTriggers {
			fmt.Println("  Triggers are already properly configured, no changes needed.")
		} else {
			fmt.Println("  Changes would be made to fix triggers.")
		}
		return nil
	}

	if !needsTriggers {
		fmt.Println("  Triggers are already properly configured, skipping...")
		// Cache is maintained for skipped pipelines (no cache deletion)
		return nil
	}

	// パイプラインを更新
	fmt.Println("  Updating pipeline triggers...")
	err := awsClient.UpdatePipelineTriggersV2(ctx, pipeline.GetName(), newTriggers)
	if err != nil {
		return fmt.Errorf("failed to update pipeline triggers: %w", err)
	}

	fmt.Println("  Pipeline triggers updated successfully")

	// Delete cache for updated pipeline
	if fileCache != nil {
		fmt.Println("  Clearing cache for updated pipeline...")
		if err := fileCache.DeletePipelineCache(pipeline.GetName()); err != nil {
			fmt.Printf("  Warning: Failed to clear cache for pipeline %s: %v\n", pipeline.GetName(), err)
			// Don't return error as cache deletion failure shouldn't fail the main operation
		} else {
			fmt.Println("  Pipeline cache cleared successfully")
		}
	}

	return nil
}

func analyzeSourceActions(pipeline *models.Pipeline) []models.Action {
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

func shouldAddTriggers(sourceActions []models.Action, existingTriggers []models.Trigger, pipeline *models.Pipeline) bool {
	// 期待されるtriggersを生成
	expectedTriggers := generateTriggers(sourceActions, pipeline)

	// 既存のtriggersと期待されるtriggersを比較
	return !triggersEqual(existingTriggers, expectedTriggers)
}

// stringSlicesEqual compares two string slices for equality
func stringSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i, v := range a {
		if v != b[i] {
			return false
		}
	}
	return true
}

func generateTriggers(sourceActions []models.Action, pipeline *models.Pipeline) []models.Trigger {
	var triggers []models.Trigger

	// trigger-mode に基づいてtriggerを生成
	switch triggerMode {
	case "none":
		// noneモードの場合は空のtrigger配列を返す
		fmt.Printf("  Trigger mode: none - removing all triggers\n")
		return triggers

	case "single":
		// singleモードの場合は指定されたsource actionのみ
		fmt.Printf("  Trigger mode: single - configuring trigger for '%s'\n", sourceAction)
		for _, action := range sourceActions {
			if action.Name == sourceAction {
				trigger := models.Trigger{
					ProviderType: "CodeStarSourceConnection",
					GitConfiguration: &models.GitConfiguration{
						SourceActionName: action.Name,
						Push: []models.PushFilter{
							{
								Branches: &models.BranchFilter{
									Includes: []string{"main"},
								},
								FilePaths: &models.FilePathFilter{
									Includes: getFilePathsForAction(action.Name, pipeline),
								},
							},
						},
					},
				}
				triggers = append(triggers, trigger)
				break
			}
		}

	case "auto":
		fallthrough
	default:
		// autoモード（デフォルト）の場合はすべてのsource actionに対してtriggerを生成
		fmt.Printf("  Trigger mode: auto - configuring triggers for all source actions\n")
		for _, action := range sourceActions {
			trigger := models.Trigger{
				ProviderType: "CodeStarSourceConnection",
				GitConfiguration: &models.GitConfiguration{
					SourceActionName: action.Name,
					Push: []models.PushFilter{
						{
							Branches: &models.BranchFilter{
								Includes: []string{"main"},
							},
							FilePaths: &models.FilePathFilter{
								Includes: getFilePathsForAction(action.Name, pipeline),
							},
						},
					},
				},
			}
			triggers = append(triggers, trigger)
		}
	}

	return triggers
}

func getFilePathsForAction(actionName string, pipeline *models.Pipeline) []string {
	// actionNameに基づいてfile pathsを決定
	switch actionName {
	case "aft-global-customizations":
		return []string{"*.tf"}
	case "aft-account-customizations":
		// pipelineからaccount nameを取得
		accountName := extractAccountNameFromPipeline(pipeline)
		if accountName != "" {
			return []string{fmt.Sprintf("%s/terraform/*.tf", accountName)}
		}
		// フォールバックとしてデフォルトパターンを返す
		return []string{"network-dev/terraform/*.tf"}
	default:
		// その他のactionの場合はデフォルトパターン
		return []string{"*.tf"}
	}
}

// extractAccountNameFromPipeline extracts account name from pipeline
func extractAccountNameFromPipeline(pipeline *models.Pipeline) string {
	// AccountNameが設定されている場合はそれを使用（これが正しいaccount name）
	if pipeline.AccountName != "" {
		return pipeline.AccountName
	}

	// AccountNameが設定されていない場合は空文字を返す
	// Account IDは数字なので、ファイルパスには適さない
	return ""
}

// extractAccountIDFromPipelineName extracts account ID from pipeline name
func extractAccountIDFromPipelineName(pipelineName string) string {
	parts := strings.Split(pipelineName, "-")
	if len(parts) > 0 {
		return parts[0]
	}
	return ""
}

// showUpdatePipelineInput displays the AWS API UpdatePipelineInput content
func showUpdatePipelineInput(pipeline *models.Pipeline, newTriggers []models.Trigger) error {
	// AWS SDK用のトリガーに変換
	awsTriggers := convertTriggersToAWSForDisplay(newTriggers)

	// UpdatePipelineInputの構造を模擬
	updateInput := struct {
		Pipeline struct {
			Name          string                   `json:"name"`
			RoleArn       string                   `json:"roleArn"`
			ArtifactStore models.ArtifactStore     `json:"artifactStore"`
			Stages        []models.Stage           `json:"stages"`
			Version       int32                    `json:"version"`
			ExecutionMode string                   `json:"executionMode"`
			PipelineType  string                   `json:"pipelineType"`
			Triggers      []map[string]interface{} `json:"triggers"`
		} `json:"pipeline"`
	}{
		Pipeline: struct {
			Name          string                   `json:"name"`
			RoleArn       string                   `json:"roleArn"`
			ArtifactStore models.ArtifactStore     `json:"artifactStore"`
			Stages        []models.Stage           `json:"stages"`
			Version       int32                    `json:"version"`
			ExecutionMode string                   `json:"executionMode"`
			PipelineType  string                   `json:"pipelineType"`
			Triggers      []map[string]interface{} `json:"triggers"`
		}{
			Name:          pipeline.Pipeline.Name,
			RoleArn:       pipeline.Pipeline.RoleArn,
			ArtifactStore: pipeline.Pipeline.ArtifactStore,
			Stages:        pipeline.Pipeline.Stages,
			Version:       pipeline.Pipeline.Version,
			ExecutionMode: pipeline.Pipeline.ExecutionMode,
			PipelineType:  pipeline.Pipeline.PipelineType,
			Triggers:      awsTriggers,
		},
	}

	// JSON形式で出力
	jsonBytes, err := json.MarshalIndent(updateInput, "  ", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal UpdatePipelineInput: %w", err)
	}

	fmt.Printf("  UpdatePipelineInput JSON:\n%s\n", string(jsonBytes))
	return nil
}

// showTriggersDiff displays the difference between current and new triggers
func showTriggersDiff(pipeline *models.Pipeline, newTriggers []models.Trigger) error {
	// Check if there are any differences
	hasDifferences := !triggersEqual(pipeline.Pipeline.Triggers, newTriggers)

	if !hasDifferences {
		fmt.Printf("  %s\n", utils.Success("✓ No changes needed - triggers are already properly configured"))
		return nil
	}

	fmt.Printf("\n    %s\n", utils.Highlight("=== TRIGGERS DIFF ==="))

	// Show only the differences
	addedTriggers, removedTriggers, modifiedTriggers := compareTriggers(pipeline.Pipeline.Triggers, newTriggers)

	// Show removed triggers
	if len(removedTriggers) > 0 {
		fmt.Printf("    %s\n", utils.Error("REMOVED (-):"))
		for i, trigger := range removedTriggers {
			fmt.Printf("      %s\n", utils.Error(fmt.Sprintf("- Trigger %d:", i+1)))
			printTriggerDetailsColored(trigger, "          ", utils.Red)
		}
	}

	// Show added triggers
	if len(addedTriggers) > 0 {
		fmt.Printf("    %s\n", utils.Success("ADDED (+):"))
		for i, trigger := range addedTriggers {
			fmt.Printf("      %s\n", utils.Success(fmt.Sprintf("+ Trigger %d:", i+1)))
			printTriggerDetailsColored(trigger, "          ", utils.Green)
		}
	}

	// Show modified triggers
	if len(modifiedTriggers) > 0 {
		fmt.Printf("    %s\n", utils.Warning("MODIFIED (~):"))
		for i, modification := range modifiedTriggers {
			fmt.Printf("      %s\n", utils.Warning(fmt.Sprintf("~ Trigger %d (%s):", i+1, modification.SourceAction)))
			if len(modification.Changes) > 0 {
				for _, change := range modification.Changes {
					fmt.Printf("          %s: %s -> %s\n",
						change.Field,
						utils.Colorize(utils.Red, change.Old),
						utils.Colorize(utils.Green, change.New))
				}
			}
		}
	}

	// Show JSON diff only if there are significant changes
	if len(addedTriggers) > 0 || len(removedTriggers) > 0 {
		fmt.Printf("\n    %s\n", utils.Highlight("=== JSON DIFF ==="))

		if len(removedTriggers) > 0 {
			fmt.Printf("    %s\n", utils.Error("REMOVED JSON (-):"))
			removedJSON, err := json.MarshalIndent(removedTriggers, "      ", "  ")
			if err != nil {
				return fmt.Errorf("failed to marshal removed triggers: %w", err)
			}
			fmt.Printf("      %s\n", utils.Colorize(utils.Red, string(removedJSON)))
		}

		if len(addedTriggers) > 0 {
			fmt.Printf("    %s\n", utils.Success("ADDED JSON (+):"))
			addedJSON, err := json.MarshalIndent(addedTriggers, "      ", "  ")
			if err != nil {
				return fmt.Errorf("failed to marshal added triggers: %w", err)
			}
			fmt.Printf("      %s\n", utils.Colorize(utils.Green, string(addedJSON)))
		}
	}

	return nil
}

// printTriggerDetailsColored prints the details of a trigger with the given indent and color
func printTriggerDetailsColored(trigger models.Trigger, indent, color string) {
	fmt.Printf("%s%s\n", indent, utils.Colorize(color, fmt.Sprintf("Provider: %s", trigger.ProviderType)))
	if trigger.GitConfiguration != nil {
		fmt.Printf("%s%s\n", indent, utils.Colorize(color, fmt.Sprintf("Source Action: %s", trigger.GitConfiguration.SourceActionName)))
		if len(trigger.GitConfiguration.Push) > 0 {
			push := trigger.GitConfiguration.Push[0]
			if push.Branches != nil && len(push.Branches.Includes) > 0 {
				fmt.Printf("%s%s\n", indent, utils.Colorize(color, fmt.Sprintf("Branches: %v", push.Branches.Includes)))
			}
			if push.FilePaths != nil && len(push.FilePaths.Includes) > 0 {
				fmt.Printf("%s%s\n", indent, utils.Colorize(color, fmt.Sprintf("File Paths: %v", push.FilePaths.Includes)))
			}
		}
	}
}

// printTriggerDetails prints the details of a trigger with the given indent
func printTriggerDetails(trigger models.Trigger, indent string) {
	fmt.Printf("%sProvider: %s\n", indent, trigger.ProviderType)
	if trigger.GitConfiguration != nil {
		fmt.Printf("%sSource Action: %s\n", indent, trigger.GitConfiguration.SourceActionName)
		if len(trigger.GitConfiguration.Push) > 0 {
			push := trigger.GitConfiguration.Push[0]
			if push.Branches != nil && len(push.Branches.Includes) > 0 {
				fmt.Printf("%sBranches: %v\n", indent, push.Branches.Includes)
			}
			if push.FilePaths != nil && len(push.FilePaths.Includes) > 0 {
				fmt.Printf("%sFile Paths: %v\n", indent, push.FilePaths.Includes)
			}
		}
	}
}

// triggersEqual checks if two trigger slices are equal
func triggersEqual(current, new []models.Trigger) bool {
	if len(current) != len(new) {
		return false
	}

	// Create maps for easier comparison
	currentMap := make(map[string]models.Trigger)
	newMap := make(map[string]models.Trigger)

	for _, trigger := range current {
		if trigger.GitConfiguration != nil {
			currentMap[trigger.GitConfiguration.SourceActionName] = trigger
		}
	}

	for _, trigger := range new {
		if trigger.GitConfiguration != nil {
			newMap[trigger.GitConfiguration.SourceActionName] = trigger
		}
	}

	// Check if all triggers match
	for sourceAction, currentTrigger := range currentMap {
		newTrigger, exists := newMap[sourceAction]
		if !exists {
			return false
		}
		if !triggerEqual(currentTrigger, newTrigger) {
			return false
		}
	}

	return true
}

// triggerEqual checks if two individual triggers are equal
func triggerEqual(a, b models.Trigger) bool {
	if a.ProviderType != b.ProviderType {
		return false
	}

	if (a.GitConfiguration == nil) != (b.GitConfiguration == nil) {
		return false
	}

	if a.GitConfiguration != nil {
		if a.GitConfiguration.SourceActionName != b.GitConfiguration.SourceActionName {
			return false
		}

		if len(a.GitConfiguration.Push) != len(b.GitConfiguration.Push) {
			return false
		}

		if len(a.GitConfiguration.Push) > 0 {
			pushA := a.GitConfiguration.Push[0]
			pushB := b.GitConfiguration.Push[0]

			// Compare branches
			if (pushA.Branches == nil) != (pushB.Branches == nil) {
				return false
			}
			if pushA.Branches != nil && !stringSlicesEqual(pushA.Branches.Includes, pushB.Branches.Includes) {
				return false
			}

			// Compare file paths
			if (pushA.FilePaths == nil) != (pushB.FilePaths == nil) {
				return false
			}
			if pushA.FilePaths != nil && !stringSlicesEqual(pushA.FilePaths.Includes, pushB.FilePaths.Includes) {
				return false
			}
		}
	}

	return true
}

// TriggerModification represents a modification to a trigger
type TriggerModification struct {
	SourceAction string
	Changes      []FieldChange
}

// FieldChange represents a change to a specific field
type FieldChange struct {
	Field string
	Old   string
	New   string
}

// compareTriggers compares two trigger slices and returns added, removed, and modified triggers
func compareTriggers(current, new []models.Trigger) (added, removed []models.Trigger, modified []TriggerModification) {
	// Create maps for easier comparison
	currentMap := make(map[string]models.Trigger)
	newMap := make(map[string]models.Trigger)

	for _, trigger := range current {
		if trigger.GitConfiguration != nil {
			currentMap[trigger.GitConfiguration.SourceActionName] = trigger
		}
	}

	for _, trigger := range new {
		if trigger.GitConfiguration != nil {
			newMap[trigger.GitConfiguration.SourceActionName] = trigger
		}
	}

	// Find added triggers
	for sourceAction, newTrigger := range newMap {
		if _, exists := currentMap[sourceAction]; !exists {
			added = append(added, newTrigger)
		}
	}

	// Find removed triggers
	for sourceAction, currentTrigger := range currentMap {
		if _, exists := newMap[sourceAction]; !exists {
			removed = append(removed, currentTrigger)
		}
	}

	// Find modified triggers
	for sourceAction, currentTrigger := range currentMap {
		if newTrigger, exists := newMap[sourceAction]; exists {
			if !triggerEqual(currentTrigger, newTrigger) {
				changes := getTriggerChanges(currentTrigger, newTrigger)
				if len(changes) > 0 {
					modified = append(modified, TriggerModification{
						SourceAction: sourceAction,
						Changes:      changes,
					})
				}
			}
		}
	}

	return added, removed, modified
}

// getTriggerChanges returns the specific changes between two triggers
func getTriggerChanges(current, new models.Trigger) []FieldChange {
	var changes []FieldChange

	if current.ProviderType != new.ProviderType {
		changes = append(changes, FieldChange{
			Field: "Provider",
			Old:   current.ProviderType,
			New:   new.ProviderType,
		})
	}

	if current.GitConfiguration != nil && new.GitConfiguration != nil {
		if len(current.GitConfiguration.Push) > 0 && len(new.GitConfiguration.Push) > 0 {
			currentPush := current.GitConfiguration.Push[0]
			newPush := new.GitConfiguration.Push[0]

			// Check branches
			if currentPush.Branches != nil && newPush.Branches != nil {
				if !stringSlicesEqual(currentPush.Branches.Includes, newPush.Branches.Includes) {
					changes = append(changes, FieldChange{
						Field: "Branches",
						Old:   fmt.Sprintf("%v", currentPush.Branches.Includes),
						New:   fmt.Sprintf("%v", newPush.Branches.Includes),
					})
				}
			}

			// Check file paths
			if currentPush.FilePaths != nil && newPush.FilePaths != nil {
				if !stringSlicesEqual(currentPush.FilePaths.Includes, newPush.FilePaths.Includes) {
					changes = append(changes, FieldChange{
						Field: "File Paths",
						Old:   fmt.Sprintf("%v", currentPush.FilePaths.Includes),
						New:   fmt.Sprintf("%v", newPush.FilePaths.Includes),
					})
				}
			}
		}
	}

	return changes
}

// validateTriggerModeFlags validates the trigger mode flags
func validateTriggerModeFlags() error {
	validModes := []string{"auto", "single", "none"}
	validSourceActions := []string{"aft-global-customizations", "aft-account-customizations"}

	// Check if trigger mode is valid
	isValidMode := false
	for _, mode := range validModes {
		if triggerMode == mode {
			isValidMode = true
			break
		}
	}
	if !isValidMode {
		return fmt.Errorf("invalid trigger-mode '%s'. Valid options: %v", triggerMode, validModes)
	}

	// If single mode is selected, source-action must be specified
	if triggerMode == "single" {
		if sourceAction == "" {
			return fmt.Errorf("source-action must be specified when using 'single' trigger-mode. Valid options: %v", validSourceActions)
		}

		// Check if source action is valid
		isValidSourceAction := false
		for _, action := range validSourceActions {
			if sourceAction == action {
				isValidSourceAction = true
				break
			}
		}
		if !isValidSourceAction {
			return fmt.Errorf("invalid source-action '%s'. Valid options: %v", sourceAction, validSourceActions)
		}
	}

	// If not single mode, source-action should not be specified
	if triggerMode != "single" && sourceAction != "" {
		return fmt.Errorf("source-action can only be used with 'single' trigger-mode")
	}

	return nil
}

// convertTriggersToAWSForDisplay converts triggers to AWS format for display
func convertTriggersToAWSForDisplay(triggers []models.Trigger) []map[string]interface{} {
	var awsTriggers []map[string]interface{}

	for _, trigger := range triggers {
		awsTrigger := map[string]interface{}{
			"providerType": trigger.ProviderType,
		}

		if trigger.GitConfiguration != nil {
			gitConfig := map[string]interface{}{
				"sourceActionName": trigger.GitConfiguration.SourceActionName,
			}

			if trigger.GitConfiguration.Push != nil {
				var pushFilters []map[string]interface{}
				for _, pushFilter := range trigger.GitConfiguration.Push {
					awsPushFilter := map[string]interface{}{}

					if pushFilter.Branches != nil && len(pushFilter.Branches.Includes) > 0 {
						awsPushFilter["branches"] = map[string]interface{}{
							"includes": pushFilter.Branches.Includes,
						}
					}

					if pushFilter.FilePaths != nil && len(pushFilter.FilePaths.Includes) > 0 {
						awsPushFilter["filePaths"] = map[string]interface{}{
							"includes": pushFilter.FilePaths.Includes,
						}
					}

					pushFilters = append(pushFilters, awsPushFilter)
				}
				gitConfig["push"] = pushFilters
			}

			awsTrigger["gitConfiguration"] = gitConfig
		}

		awsTriggers = append(awsTriggers, awsTrigger)
	}

	return awsTriggers
}

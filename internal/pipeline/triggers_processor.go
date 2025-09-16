package pipeline

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hacker65536/aft-pipeline-tool/internal/aws"
	"github.com/hacker65536/aft-pipeline-tool/internal/cache"
	"github.com/hacker65536/aft-pipeline-tool/internal/models"
	"github.com/hacker65536/aft-pipeline-tool/internal/utils"
)

// TriggersProcessor handles pipeline trigger fixes
type TriggersProcessor struct {
	triggerMode  string
	sourceAction string
}

// NewTriggersProcessor creates a new triggers processor
func NewTriggersProcessor(triggerMode, sourceAction string) *TriggersProcessor {
	return &TriggersProcessor{
		triggerMode:  triggerMode,
		sourceAction: sourceAction,
	}
}

// ProcessPipeline processes a single pipeline for trigger fixes
func (p *TriggersProcessor) ProcessPipeline(ctx context.Context, awsClient *aws.Client, pipeline *models.Pipeline, dryRun bool, fileCache *cache.FileCache) error {
	// Source stageからactionsを分析
	sourceActions := p.analyzeSourceActions(pipeline)
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
	newTriggers := p.generateTriggers(sourceActions, pipeline)
	fmt.Printf("  Generated %d new triggers\n", len(newTriggers))

	return ProcessPipelineWithCommonLogic(ctx, awsClient, pipeline, dryRun, fileCache, p, func(ctx context.Context, awsClient *aws.Client, pipeline *models.Pipeline) error {
		return awsClient.UpdatePipelineTriggersV2(ctx, pipeline.GetName(), newTriggers)
	})
}

// ShowDiff displays the differences for dry-run mode
func (p *TriggersProcessor) ShowDiff(pipeline *models.Pipeline) error {
	sourceActions := p.analyzeSourceActions(pipeline)
	newTriggers := p.generateTriggers(sourceActions, pipeline)
	return p.showTriggersDiff(pipeline, newTriggers)
}

// ShouldUpdate determines if the pipeline needs to be updated
func (p *TriggersProcessor) ShouldUpdate(pipeline *models.Pipeline) bool {
	sourceActions := p.analyzeSourceActions(pipeline)
	existingTriggers := pipeline.Pipeline.Triggers
	expectedTriggers := p.generateTriggers(sourceActions, pipeline)
	return !p.triggersEqual(existingTriggers, expectedTriggers)
}

// GetProcessorName returns the name of the processor for logging
func (p *TriggersProcessor) GetProcessorName() string {
	return "triggers"
}

func (p *TriggersProcessor) analyzeSourceActions(pipeline *models.Pipeline) []models.Action {
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

func (p *TriggersProcessor) generateTriggers(sourceActions []models.Action, pipeline *models.Pipeline) []models.Trigger {
	var triggers []models.Trigger

	// trigger-mode に基づいてtriggerを生成
	switch p.triggerMode {
	case "none":
		// noneモードの場合は空のtrigger配列を返す
		fmt.Printf("  Trigger mode: none - removing all triggers\n")
		return triggers

	case "single":
		// singleモードの場合は指定されたsource actionのみ
		fmt.Printf("  Trigger mode: single - configuring trigger for '%s'\n", p.sourceAction)
		for _, action := range sourceActions {
			if action.Name == p.sourceAction {
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
									Includes: p.getFilePathsForAction(action.Name, pipeline),
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
								Includes: p.getFilePathsForAction(action.Name, pipeline),
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

func (p *TriggersProcessor) getFilePathsForAction(actionName string, pipeline *models.Pipeline) []string {
	// actionNameに基づいてfile pathsを決定
	switch actionName {
	case "aft-global-customizations":
		return []string{"*.tf"}
	case "aft-account-customizations":
		// pipelineからaccount nameを取得
		accountName := p.extractAccountNameFromPipeline(pipeline)
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
func (p *TriggersProcessor) extractAccountNameFromPipeline(pipeline *models.Pipeline) string {
	// AccountNameが設定されている場合はそれを使用（これが正しいaccount name）
	if pipeline.AccountName != "" {
		return pipeline.AccountName
	}

	// AccountNameが設定されていない場合は空文字を返す
	// Account IDは数字なので、ファイルパスには適さない
	return ""
}

// showTriggersDiff displays the difference between current and new triggers
func (p *TriggersProcessor) showTriggersDiff(pipeline *models.Pipeline, newTriggers []models.Trigger) error {
	// Check if there are any differences
	hasDifferences := !p.triggersEqual(pipeline.Pipeline.Triggers, newTriggers)

	if !hasDifferences {
		fmt.Printf("  %s\n", utils.Success("✓ No changes needed - triggers are already properly configured"))
		return nil
	}

	fmt.Printf("\n    %s\n", utils.Highlight("=== TRIGGERS DIFF ==="))

	// Show only the differences
	addedTriggers, removedTriggers, modifiedTriggers := p.compareTriggers(pipeline.Pipeline.Triggers, newTriggers)

	// Show removed triggers
	if len(removedTriggers) > 0 {
		fmt.Printf("    %s\n", utils.Error("REMOVED (-):"))
		for i, trigger := range removedTriggers {
			fmt.Printf("      %s\n", utils.Error(fmt.Sprintf("- Trigger %d:", i+1)))
			p.printTriggerDetailsColored(trigger, "          ", utils.Red)
		}
	}

	// Show added triggers
	if len(addedTriggers) > 0 {
		fmt.Printf("    %s\n", utils.Success("ADDED (+):"))
		for i, trigger := range addedTriggers {
			fmt.Printf("      %s\n", utils.Success(fmt.Sprintf("+ Trigger %d:", i+1)))
			p.printTriggerDetailsColored(trigger, "          ", utils.Green)
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
func (p *TriggersProcessor) printTriggerDetailsColored(trigger models.Trigger, indent, color string) {
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

// triggersEqual checks if two trigger slices are equal
func (p *TriggersProcessor) triggersEqual(current, new []models.Trigger) bool {
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
		if !p.triggerEqual(currentTrigger, newTrigger) {
			return false
		}
	}

	return true
}

// triggerEqual checks if two individual triggers are equal
func (p *TriggersProcessor) triggerEqual(a, b models.Trigger) bool {
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
			if pushA.Branches != nil && !p.stringSlicesEqual(pushA.Branches.Includes, pushB.Branches.Includes) {
				return false
			}

			// Compare file paths
			if (pushA.FilePaths == nil) != (pushB.FilePaths == nil) {
				return false
			}
			if pushA.FilePaths != nil && !p.stringSlicesEqual(pushA.FilePaths.Includes, pushB.FilePaths.Includes) {
				return false
			}
		}
	}

	return true
}

// stringSlicesEqual compares two string slices for equality
func (p *TriggersProcessor) stringSlicesEqual(a, b []string) bool {
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
func (p *TriggersProcessor) compareTriggers(current, new []models.Trigger) (added, removed []models.Trigger, modified []TriggerModification) {
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
			if !p.triggerEqual(currentTrigger, newTrigger) {
				changes := p.getTriggerChanges(currentTrigger, newTrigger)
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
func (p *TriggersProcessor) getTriggerChanges(current, new models.Trigger) []FieldChange {
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
				if !p.stringSlicesEqual(currentPush.Branches.Includes, newPush.Branches.Includes) {
					changes = append(changes, FieldChange{
						Field: "Branches",
						Old:   fmt.Sprintf("%v", currentPush.Branches.Includes),
						New:   fmt.Sprintf("%v", newPush.Branches.Includes),
					})
				}
			}

			// Check file paths
			if currentPush.FilePaths != nil && newPush.FilePaths != nil {
				if !p.stringSlicesEqual(currentPush.FilePaths.Includes, newPush.FilePaths.Includes) {
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

// ValidateTriggerModeFlags validates the trigger mode flags
func ValidateTriggerModeFlags(triggerMode, sourceAction string) error {
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

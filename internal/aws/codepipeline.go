package aws

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/codepipeline"
	"github.com/aws/aws-sdk-go-v2/service/codepipeline/types"

	"github.com/hacker65536/aft-pipeline-tool/internal/models"
)

// ListAFTPipelines retrieves AFT pipelines for the given accounts
func (c *Client) ListAFTPipelines(ctx context.Context, accounts []models.Account) ([]models.Pipeline, error) {
	var pipelines []models.Pipeline

	output, err := c.CodePipeline.ListPipelines(ctx, &codepipeline.ListPipelinesInput{})
	if err != nil {
		return nil, fmt.Errorf("failed to list pipelines: %w", err)
	}

	accountMap := make(map[string]models.Account)
	for _, account := range accounts {
		accountMap[account.ID] = account
	}

	for _, pipeline := range output.Pipelines {
		pipelineName := *pipeline.Name

		// AFTパイプラインのパターンマッチング
		if strings.Contains(pipelineName, "-customizations-pipeline") {
			accountID := extractAccountIDFromPipelineName(pipelineName)
			if _, exists := accountMap[accountID]; exists {
				var version int32
				if pipeline.Version != nil {
					version = *pipeline.Version
				}
				pipelines = append(pipelines, models.Pipeline{
					Pipeline: models.PipelineDeclaration{
						Name:    pipelineName,
						Version: version,
					},
					Metadata: models.PipelineMetadata{
						Created: *pipeline.Created,
						Updated: *pipeline.Updated,
					},
					AccountID: accountID,
				})
			}
		}
	}

	return pipelines, nil
}

// GetPipelineDetails retrieves detailed information about a specific pipeline
func (c *Client) GetPipelineDetails(ctx context.Context, pipelineName string) (*models.Pipeline, error) {
	output, err := c.CodePipeline.GetPipeline(ctx, &codepipeline.GetPipelineInput{
		Name: &pipelineName,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get pipeline %s: %w", pipelineName, err)
	}

	// Convert AWS pipeline to our model
	pipeline := convertAWSPipelineToModel(output.Pipeline, output.Metadata)

	return pipeline, nil
}

// convertAWSPipelineToModel converts AWS CodePipeline to our Pipeline model
func convertAWSPipelineToModel(awsPipeline *types.PipelineDeclaration, metadata *types.PipelineMetadata) *models.Pipeline {
	var version int32
	if awsPipeline.Version != nil {
		version = *awsPipeline.Version
	}

	pipeline := &models.Pipeline{
		Pipeline: models.PipelineDeclaration{
			Name:    *awsPipeline.Name,
			Version: version,
		},
	}

	// Set role ARN if available
	if awsPipeline.RoleArn != nil {
		pipeline.Pipeline.RoleArn = *awsPipeline.RoleArn
	}

	// Set execution mode if available
	if awsPipeline.ExecutionMode != "" {
		pipeline.Pipeline.ExecutionMode = string(awsPipeline.ExecutionMode)
	}

	// Set pipeline type if available
	if awsPipeline.PipelineType != "" {
		pipeline.Pipeline.PipelineType = string(awsPipeline.PipelineType)
	}

	// Convert artifact store
	if awsPipeline.ArtifactStore != nil {
		pipeline.Pipeline.ArtifactStore = models.ArtifactStore{
			Type:     string(awsPipeline.ArtifactStore.Type),
			Location: *awsPipeline.ArtifactStore.Location,
		}
		if awsPipeline.ArtifactStore.EncryptionKey != nil {
			pipeline.Pipeline.ArtifactStore.EncryptionKey = &models.EncryptionKey{
				Id:   *awsPipeline.ArtifactStore.EncryptionKey.Id,
				Type: string(awsPipeline.ArtifactStore.EncryptionKey.Type),
			}
		}
	}

	// Convert stages
	for _, stage := range awsPipeline.Stages {
		modelStage := models.Stage{
			Name:    *stage.Name,
			Actions: []models.Action{},
		}

		for _, action := range stage.Actions {
			modelAction := models.Action{
				Name:     *action.Name,
				RunOrder: int(*action.RunOrder),
				ActionTypeId: models.ActionTypeId{
					Category: string(action.ActionTypeId.Category),
					Owner:    string(action.ActionTypeId.Owner),
					Provider: *action.ActionTypeId.Provider,
					Version:  *action.ActionTypeId.Version,
				},
				Configuration:   make(map[string]interface{}),
				OutputArtifacts: []models.Artifact{},
				InputArtifacts:  []models.Artifact{},
			}

			// Convert configuration
			for key, value := range action.Configuration {
				modelAction.Configuration[key] = value
			}

			// Convert artifacts
			for _, artifact := range action.OutputArtifacts {
				modelAction.OutputArtifacts = append(modelAction.OutputArtifacts, models.Artifact{
					Name: *artifact.Name,
				})
			}
			for _, artifact := range action.InputArtifacts {
				modelAction.InputArtifacts = append(modelAction.InputArtifacts, models.Artifact{
					Name: *artifact.Name,
				})
			}

			modelStage.Actions = append(modelStage.Actions, modelAction)
		}

		pipeline.Pipeline.Stages = append(pipeline.Pipeline.Stages, modelStage)
	}

	// Convert triggers
	if awsPipeline.Triggers != nil {
		for _, trigger := range awsPipeline.Triggers {
			modelTrigger := models.Trigger{
				ProviderType: string(trigger.ProviderType),
			}

			// Convert git configuration if present
			if trigger.GitConfiguration != nil {
				gitConfig := &models.GitConfiguration{
					SourceActionName: *trigger.GitConfiguration.SourceActionName,
				}

				// Convert push filters
				if trigger.GitConfiguration.Push != nil {
					for _, pushFilter := range trigger.GitConfiguration.Push {
						modelPushFilter := models.PushFilter{}

						// Convert branch filters
						if pushFilter.Branches != nil && pushFilter.Branches.Includes != nil {
							modelPushFilter.Branches = &models.BranchFilter{
								Includes: pushFilter.Branches.Includes,
							}
						}

						// Convert file path filters
						if pushFilter.FilePaths != nil && pushFilter.FilePaths.Includes != nil {
							modelPushFilter.FilePaths = &models.FilePathFilter{
								Includes: pushFilter.FilePaths.Includes,
							}
						}

						gitConfig.Push = append(gitConfig.Push, modelPushFilter)
					}
				}

				modelTrigger.GitConfiguration = gitConfig
			}

			pipeline.Pipeline.Triggers = append(pipeline.Pipeline.Triggers, modelTrigger)
		}
	}

	// Set metadata if available
	if metadata != nil {
		if metadata.PipelineArn != nil {
			pipeline.Metadata.PipelineArn = *metadata.PipelineArn
		}
		if metadata.Created != nil {
			pipeline.Metadata.Created = *metadata.Created
		}
		if metadata.Updated != nil {
			pipeline.Metadata.Updated = *metadata.Updated
		}
	}

	return pipeline
}

// UpdatePipelineTrigger updates the trigger settings for a pipeline
func (c *Client) UpdatePipelineTrigger(ctx context.Context, pipelineName string, settings models.TriggerSettings) error {
	// パイプライン取得
	getPipelineOutput, err := c.CodePipeline.GetPipeline(ctx, &codepipeline.GetPipelineInput{
		Name: &pipelineName,
	})
	if err != nil {
		return fmt.Errorf("failed to get pipeline: %w", err)
	}

	// ソースアクションの設定を更新
	pipeline := getPipelineOutput.Pipeline
	if len(pipeline.Stages) > 0 && len(pipeline.Stages[0].Actions) > 0 {
		sourceAction := &pipeline.Stages[0].Actions[0]
		updateSourceActionTrigger(sourceAction, settings)
	}

	// パイプライン更新
	_, err = c.CodePipeline.UpdatePipeline(ctx, &codepipeline.UpdatePipelineInput{
		Pipeline: pipeline,
	})
	if err != nil {
		return fmt.Errorf("failed to update pipeline: %w", err)
	}

	return nil
}

// extractAccountIDFromPipelineName extracts account ID from pipeline name
func extractAccountIDFromPipelineName(pipelineName string) string {
	parts := strings.Split(pipelineName, "-")
	if len(parts) > 0 {
		return parts[0]
	}
	return ""
}

// UpdatePipelineTriggersV2 updates the V2 pipeline triggers
func (c *Client) UpdatePipelineTriggersV2(ctx context.Context, pipelineName string, triggers []models.Trigger) error {
	// パイプライン取得
	getPipelineOutput, err := c.CodePipeline.GetPipeline(ctx, &codepipeline.GetPipelineInput{
		Name: &pipelineName,
	})
	if err != nil {
		return fmt.Errorf("failed to get pipeline: %w", err)
	}

	// AWS SDK用のトリガーに変換
	awsTriggers := convertTriggersToAWS(triggers)

	// パイプライン定義を更新
	pipeline := getPipelineOutput.Pipeline
	pipeline.Triggers = awsTriggers

	// パイプライン更新
	_, err = c.CodePipeline.UpdatePipeline(ctx, &codepipeline.UpdatePipelineInput{
		Pipeline: pipeline,
	})
	if err != nil {
		return fmt.Errorf("failed to update pipeline: %w", err)
	}

	return nil
}

// convertTriggersToAWS converts our trigger models to AWS SDK types
func convertTriggersToAWS(triggers []models.Trigger) []types.PipelineTriggerDeclaration {
	var awsTriggers []types.PipelineTriggerDeclaration

	for _, trigger := range triggers {
		awsTrigger := types.PipelineTriggerDeclaration{
			ProviderType: types.PipelineTriggerProviderType(trigger.ProviderType),
		}

		if trigger.GitConfiguration != nil {
			gitConfig := &types.GitConfiguration{
				SourceActionName: &trigger.GitConfiguration.SourceActionName,
			}

			// Push filtersを変換
			if trigger.GitConfiguration.Push != nil {
				for _, pushFilter := range trigger.GitConfiguration.Push {
					awsPushFilter := types.GitPushFilter{}

					// Branch filtersを変換
					if pushFilter.Branches != nil && len(pushFilter.Branches.Includes) > 0 {
						awsPushFilter.Branches = &types.GitBranchFilterCriteria{
							Includes: pushFilter.Branches.Includes,
						}
					}

					// File path filtersを変換
					if pushFilter.FilePaths != nil && len(pushFilter.FilePaths.Includes) > 0 {
						awsPushFilter.FilePaths = &types.GitFilePathFilterCriteria{
							Includes: pushFilter.FilePaths.Includes,
						}
					}

					gitConfig.Push = append(gitConfig.Push, awsPushFilter)
				}
			}

			awsTrigger.GitConfiguration = gitConfig
		}

		awsTriggers = append(awsTriggers, awsTrigger)
	}

	return awsTriggers
}

// UpdatePipelineConnectionsV2 updates the ConnectionArn values for source actions in a pipeline
func (c *Client) UpdatePipelineConnectionsV2(ctx context.Context, pipelineName, newConnectionArn, sourceActionName string) error {
	// パイプライン取得
	getPipelineOutput, err := c.CodePipeline.GetPipeline(ctx, &codepipeline.GetPipelineInput{
		Name: &pipelineName,
	})
	if err != nil {
		return fmt.Errorf("failed to get pipeline: %w", err)
	}

	// パイプライン定義を更新
	pipeline := getPipelineOutput.Pipeline
	updated := false

	// Source stageのactionsを更新
	for stageIdx, stage := range pipeline.Stages {
		if *stage.Name == "Source" {
			for actionIdx, action := range stage.Actions {
				// CodeStarSourceConnectionのactionのみを対象とする
				if action.ActionTypeId != nil && *action.ActionTypeId.Provider == "CodeStarSourceConnection" {
					// 特定のsource actionが指定されている場合はそれのみを対象とする
					if sourceActionName != "" && *action.Name != sourceActionName {
						continue
					}

					// ConnectionArnを更新
					if action.Configuration == nil {
						action.Configuration = make(map[string]string)
					}
					action.Configuration["ConnectionArn"] = newConnectionArn
					pipeline.Stages[stageIdx].Actions[actionIdx] = action
					updated = true
				}
			}
			break
		}
	}

	if !updated {
		return fmt.Errorf("no matching source actions found to update")
	}

	// パイプライン更新
	_, err = c.CodePipeline.UpdatePipeline(ctx, &codepipeline.UpdatePipelineInput{
		Pipeline: pipeline,
	})
	if err != nil {
		return fmt.Errorf("failed to update pipeline: %w", err)
	}

	return nil
}

// UpdatePipelineTypeV2 updates the PipelineType field for a pipeline
func (c *Client) UpdatePipelineTypeV2(ctx context.Context, pipelineName, newPipelineType string) error {
	// パイプライン取得
	getPipelineOutput, err := c.CodePipeline.GetPipeline(ctx, &codepipeline.GetPipelineInput{
		Name: &pipelineName,
	})
	if err != nil {
		return fmt.Errorf("failed to get pipeline: %w", err)
	}

	// パイプライン定義を更新
	pipeline := getPipelineOutput.Pipeline
	pipeline.PipelineType = types.PipelineType(newPipelineType)

	// パイプライン更新
	_, err = c.CodePipeline.UpdatePipeline(ctx, &codepipeline.UpdatePipelineInput{
		Pipeline: pipeline,
	})
	if err != nil {
		return fmt.Errorf("failed to update pipeline: %w", err)
	}

	return nil
}

// GetPipelineState retrieves the current state of a pipeline
func (c *Client) GetPipelineState(ctx context.Context, pipelineName string) (*codepipeline.GetPipelineStateOutput, error) {
	output, err := c.CodePipeline.GetPipelineState(ctx, &codepipeline.GetPipelineStateInput{
		Name: &pipelineName,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get pipeline state for %s: %w", pipelineName, err)
	}

	return output, nil
}

// StartPipelineExecution starts a pipeline execution
func (c *Client) StartPipelineExecution(ctx context.Context, pipelineName string) (*codepipeline.StartPipelineExecutionOutput, error) {
	output, err := c.CodePipeline.StartPipelineExecution(ctx, &codepipeline.StartPipelineExecutionInput{
		Name: &pipelineName,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to start pipeline execution for %s: %w", pipelineName, err)
	}

	return output, nil
}

// GetPipelineExecution retrieves information about a specific pipeline execution
func (c *Client) GetPipelineExecution(ctx context.Context, pipelineName, executionId string) (*codepipeline.GetPipelineExecutionOutput, error) {
	output, err := c.CodePipeline.GetPipelineExecution(ctx, &codepipeline.GetPipelineExecutionInput{
		PipelineName:        &pipelineName,
		PipelineExecutionId: &executionId,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get pipeline execution %s for %s: %w", executionId, pipelineName, err)
	}

	return output, nil
}

// ListPipelineExecutions lists recent executions for a pipeline
func (c *Client) ListPipelineExecutions(ctx context.Context, pipelineName string, maxResults int32) (*codepipeline.ListPipelineExecutionsOutput, error) {
	input := &codepipeline.ListPipelineExecutionsInput{
		PipelineName: &pipelineName,
	}

	if maxResults > 0 {
		input.MaxResults = &maxResults
	}

	output, err := c.CodePipeline.ListPipelineExecutions(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to list pipeline executions for %s: %w", pipelineName, err)
	}

	return output, nil
}

// StopPipelineExecution stops a pipeline execution
func (c *Client) StopPipelineExecution(ctx context.Context, pipelineName, executionId, reason string) (*codepipeline.StopPipelineExecutionOutput, error) {
	input := &codepipeline.StopPipelineExecutionInput{
		PipelineName:        &pipelineName,
		PipelineExecutionId: &executionId,
	}

	if reason != "" {
		input.Reason = &reason
	}

	output, err := c.CodePipeline.StopPipelineExecution(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to stop pipeline execution %s for %s: %w", executionId, pipelineName, err)
	}

	return output, nil
}

// updateSourceActionTrigger updates source action trigger configuration
func updateSourceActionTrigger(action *types.ActionDeclaration, settings models.TriggerSettings) {
	if action.Configuration == nil {
		action.Configuration = make(map[string]string)
	}

	// ブランチ設定
	if settings.Branch != "" {
		action.Configuration["Branch"] = settings.Branch
		action.Configuration["BranchName"] = settings.Branch
	}

	// トリガー設定
	if settings.TriggerEnabled {
		action.Configuration["PollForSourceChanges"] = "false"
		// Webhook設定は別途実装が必要
	} else {
		action.Configuration["PollForSourceChanges"] = "false"
		delete(action.Configuration, "WebhookFilters")
	}

	if settings.PollingEnabled {
		action.Configuration["PollForSourceChanges"] = "true"
	}
}

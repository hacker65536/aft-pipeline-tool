package models

import "time"

// TriggerType represents the type of pipeline trigger
type TriggerType string

const (
	TriggerTypeWebhook TriggerType = "webhook"
	TriggerTypePolling TriggerType = "polling"
	TriggerTypeNone    TriggerType = "none"
)

// ActionTypeId represents the action type identifier
type ActionTypeId struct {
	Category string `json:"category"`
	Owner    string `json:"owner"`
	Provider string `json:"provider"`
	Version  string `json:"version"`
}

// Artifact represents input/output artifacts
type Artifact struct {
	Name string `json:"name"`
}

// Action represents a pipeline action
type Action struct {
	Name            string                 `json:"name"`
	ActionTypeId    ActionTypeId           `json:"actionTypeId"`
	RunOrder        int                    `json:"runOrder"`
	Configuration   map[string]interface{} `json:"configuration"`
	OutputArtifacts []Artifact             `json:"outputArtifacts"`
	InputArtifacts  []Artifact             `json:"inputArtifacts"`
}

// Stage represents a pipeline stage
type Stage struct {
	Name    string   `json:"name"`
	Actions []Action `json:"actions"`
}

// EncryptionKey represents artifact store encryption
type EncryptionKey struct {
	Id   string `json:"id"`
	Type string `json:"type"`
}

// ArtifactStore represents the artifact store configuration
type ArtifactStore struct {
	Type          string         `json:"type"`
	Location      string         `json:"location"`
	EncryptionKey *EncryptionKey `json:"encryptionKey,omitempty"`
}

// BranchFilter represents branch filtering for triggers
type BranchFilter struct {
	Includes []string `json:"includes"`
}

// FilePathFilter represents file path filtering for triggers
type FilePathFilter struct {
	Includes []string `json:"includes"`
}

// PushFilter represents push event filtering
type PushFilter struct {
	Branches  *BranchFilter   `json:"branches,omitempty"`
	FilePaths *FilePathFilter `json:"filePaths,omitempty"`
}

// GitConfiguration represents git-based trigger configuration
type GitConfiguration struct {
	SourceActionName string       `json:"sourceActionName"`
	Push             []PushFilter `json:"push,omitempty"`
}

// Trigger represents a pipeline trigger
type Trigger struct {
	ProviderType     string            `json:"providerType"`
	GitConfiguration *GitConfiguration `json:"gitConfiguration,omitempty"`
}

// PipelineMetadata represents pipeline metadata
type PipelineMetadata struct {
	PipelineArn string    `json:"pipelineArn"`
	Created     time.Time `json:"created"`
	Updated     time.Time `json:"updated"`
}

// PipelineDeclaration represents the complete pipeline declaration
type PipelineDeclaration struct {
	Name          string        `json:"name"`
	RoleArn       string        `json:"roleArn"`
	ArtifactStore ArtifactStore `json:"artifactStore"`
	Stages        []Stage       `json:"stages"`
	Version       int32         `json:"version"`
	ExecutionMode string        `json:"executionMode"`
	PipelineType  string        `json:"pipelineType"`
	Triggers      []Trigger     `json:"triggers"`
}

// PipelineState represents the state of a pipeline
type PipelineState struct {
	PipelineName    string       `json:"pipelineName"`
	PipelineVersion int32        `json:"pipelineVersion"`
	StageStates     []StageState `json:"stageStates"`
	Created         *time.Time   `json:"created,omitempty"`
	Updated         *time.Time   `json:"updated,omitempty"`
}

// StageState represents the state of a stage
type StageState struct {
	StageName              string           `json:"stageName"`
	InboundTransitionState *TransitionState `json:"inboundTransitionState,omitempty"`
	ActionStates           []ActionState    `json:"actionStates"`
	LatestExecution        *StageExecution  `json:"latestExecution,omitempty"`
}

// TransitionState represents the state of a transition
type TransitionState struct {
	Enabled        bool       `json:"enabled"`
	LastChangedBy  string     `json:"lastChangedBy,omitempty"`
	LastChangedAt  *time.Time `json:"lastChangedAt,omitempty"`
	DisabledReason string     `json:"disabledReason,omitempty"`
}

// ActionState represents the state of an action
type ActionState struct {
	ActionName      string           `json:"actionName"`
	CurrentRevision *ActionRevision  `json:"currentRevision,omitempty"`
	LatestExecution *ActionExecution `json:"latestExecution,omitempty"`
	EntityUrl       string           `json:"entityUrl,omitempty"`
	RevisionUrl     string           `json:"revisionUrl,omitempty"`
}

// ActionRevision represents the revision of an action
type ActionRevision struct {
	RevisionId       string    `json:"revisionId"`
	RevisionChangeId string    `json:"revisionChangeId,omitempty"`
	Created          time.Time `json:"created"`
}

// ActionExecution represents the execution of an action
type ActionExecution struct {
	ActionExecutionId    string        `json:"actionExecutionId,omitempty"`
	Status               string        `json:"status,omitempty"`
	Summary              string        `json:"summary,omitempty"`
	LastStatusChange     *time.Time    `json:"lastStatusChange,omitempty"`
	Token                string        `json:"token,omitempty"`
	LastUpdatedBy        string        `json:"lastUpdatedBy,omitempty"`
	ExternalExecutionId  string        `json:"externalExecutionId,omitempty"`
	ExternalExecutionUrl string        `json:"externalExecutionUrl,omitempty"`
	PercentComplete      int32         `json:"percentComplete,omitempty"`
	ErrorDetails         *ErrorDetails `json:"errorDetails,omitempty"`
}

// StageExecution represents the execution of a stage
type StageExecution struct {
	PipelineExecutionId string `json:"pipelineExecutionId"`
	Status              string `json:"status"`
}

// ErrorDetails represents error details
type ErrorDetails struct {
	Code    string `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
}

// Pipeline represents a CodePipeline with full AWS structure
type Pipeline struct {
	Pipeline PipelineDeclaration `json:"pipeline"`
	Metadata PipelineMetadata    `json:"metadata"`
	State    *PipelineState      `json:"state,omitempty"`
	// Additional fields for our application
	AccountID   string `json:"account_id"`
	AccountName string `json:"account_name"`
}

// Source represents the source configuration of a pipeline (legacy compatibility)
type Source struct {
	Provider      string                 `json:"provider"`
	Repository    string                 `json:"repository"`
	Branch        string                 `json:"branch"`
	TriggerType   TriggerType            `json:"trigger_type"`
	Configuration map[string]interface{} `json:"configuration"`
}

// PipelineCache represents cached pipeline data
type PipelineCache struct {
	Pipelines []Pipeline `json:"pipelines"`
	CachedAt  time.Time  `json:"cached_at"`
	TTL       int        `json:"ttl"`
}

// PipelineDetailsCache represents cached pipeline details data
type PipelineDetailsCache struct {
	Pipelines []Pipeline `json:"pipelines"`
	CachedAt  time.Time  `json:"cached_at"`
	TTL       int        `json:"ttl"`
}

// PipelineDetailCache represents cached individual pipeline detail data
type PipelineDetailCache struct {
	Pipeline Pipeline  `json:"pipeline"`
	CachedAt time.Time `json:"cached_at"`
	TTL      int       `json:"ttl"`
}

// PipelineStateCache represents cached individual pipeline state data
type PipelineStateCache struct {
	State    *PipelineState `json:"state"`
	CachedAt time.Time      `json:"cached_at"`
	TTL      int            `json:"ttl"`
}

// SimplePipelineCache represents the legacy simplified pipeline cache format
type SimplePipelineCache struct {
	Pipeline SimplePipeline `json:"pipeline"`
	CachedAt time.Time      `json:"cached_at"`
	TTL      int            `json:"ttl"`
}

// SimplePipeline represents the legacy simplified pipeline structure
type SimplePipeline struct {
	Name        string    `json:"name"`
	Version     int32     `json:"version"`
	Created     time.Time `json:"created"`
	Updated     time.Time `json:"updated"`
	AccountID   string    `json:"account_id"`
	AccountName string    `json:"account_name"`
	Source      Source    `json:"source"`
}

// TriggerSettings represents trigger configuration settings
type TriggerSettings struct {
	Branch         string `yaml:"branch"`
	TriggerEnabled bool   `yaml:"trigger_enabled"`
	PollingEnabled bool   `yaml:"polling_enabled"`
}

// Helper methods for backward compatibility

// GetName returns the pipeline name
func (p *Pipeline) GetName() string {
	return p.Pipeline.Name
}

// GetVersion returns the pipeline version
func (p *Pipeline) GetVersion() int32 {
	return p.Pipeline.Version
}

// GetCreated returns the pipeline creation time
func (p *Pipeline) GetCreated() time.Time {
	return p.Metadata.Created
}

// GetUpdated returns the pipeline update time
func (p *Pipeline) GetUpdated() time.Time {
	return p.Metadata.Updated
}

// GetLatestStageUpdateTime returns the latest update time from all stages
func (p *Pipeline) GetLatestStageUpdateTime() *time.Time {
	if p.State == nil || len(p.State.StageStates) == 0 {
		return nil
	}

	var latestTime *time.Time

	for _, stage := range p.State.StageStates {
		// Check stage's latest execution time
		if stage.LatestExecution != nil {
			// Note: StageExecution doesn't have a timestamp field in the current model
			// We need to check ActionStates for timestamps
		}

		// Check action states for timestamps
		for _, action := range stage.ActionStates {
			if action.LatestExecution != nil && action.LatestExecution.LastStatusChange != nil {
				if latestTime == nil || action.LatestExecution.LastStatusChange.After(*latestTime) {
					latestTime = action.LatestExecution.LastStatusChange
				}
			}
			if action.CurrentRevision != nil {
				if latestTime == nil || action.CurrentRevision.Created.After(*latestTime) {
					latestTime = &action.CurrentRevision.Created
				}
			}
		}

		// Check transition state timestamps
		if stage.InboundTransitionState != nil && stage.InboundTransitionState.LastChangedAt != nil {
			if latestTime == nil || stage.InboundTransitionState.LastChangedAt.After(*latestTime) {
				latestTime = stage.InboundTransitionState.LastChangedAt
			}
		}
	}

	return latestTime
}

// GetSource returns the legacy source configuration from the first source action
func (p *Pipeline) GetSource() Source {
	source := Source{
		Configuration: make(map[string]interface{}),
	}

	// Find the first source stage and action
	for _, stage := range p.Pipeline.Stages {
		if stage.Name == "Source" && len(stage.Actions) > 0 {
			action := stage.Actions[0]
			source.Provider = action.ActionTypeId.Provider
			source.Configuration = action.Configuration

			// Extract branch and repository information
			if branch, exists := action.Configuration["BranchName"]; exists {
				if branchStr, ok := branch.(string); ok {
					source.Branch = branchStr
				}
			}
			if repo, exists := action.Configuration["FullRepositoryId"]; exists {
				if repoStr, ok := repo.(string); ok {
					source.Repository = repoStr
				}
			}

			// Determine trigger type based on V2 pipeline triggers specification
			source.TriggerType = p.determineTriggerTypeV2(action.Name)

			break
		}
	}

	return source
}

// determineTriggerTypeV2 determines trigger type based on V2 pipeline triggers specification
func (p *Pipeline) determineTriggerTypeV2(sourceActionName string) TriggerType {
	// Default to None
	triggerType := TriggerTypeNone

	// Check V2 pipeline triggers first (preferred method)
	if len(p.Pipeline.Triggers) > 0 {
		for _, trigger := range p.Pipeline.Triggers {
			// V2 pipeline triggers with GitConfiguration
			if trigger.GitConfiguration != nil && trigger.GitConfiguration.SourceActionName == sourceActionName {
				switch trigger.ProviderType {
				case "CodeStarSourceConnection":
					triggerType = TriggerTypeWebhook
				default:
					// Other provider types can be added here as needed
					triggerType = TriggerTypeWebhook
				}
				break
			}
		}
	}

	// Fallback to legacy detection if no V2 triggers found
	if triggerType == TriggerTypeNone {
		// Find the source action to check legacy configuration
		for _, stage := range p.Pipeline.Stages {
			if stage.Name == "Source" {
				for _, action := range stage.Actions {
					if action.Name == sourceActionName {
						// Check for legacy polling configuration
						if detectChanges, exists := action.Configuration["DetectChanges"]; exists {
							if detectStr, ok := detectChanges.(string); ok && detectStr == "true" {
								triggerType = TriggerTypePolling
							}
						}
						// Check for legacy webhook configuration
						if _, hasWebhook := action.Configuration["WebhookFilters"]; hasWebhook {
							triggerType = TriggerTypeWebhook
						}
						break
					}
				}
				break
			}
		}
	}

	return triggerType
}

// SetName sets the pipeline name
func (p *Pipeline) SetName(name string) {
	p.Pipeline.Name = name
}

// SetVersion sets the pipeline version
func (p *Pipeline) SetVersion(version int32) {
	p.Pipeline.Version = version
}

// SetCreated sets the pipeline creation time
func (p *Pipeline) SetCreated(created time.Time) {
	p.Metadata.Created = created
}

// SetUpdated sets the pipeline update time
func (p *Pipeline) SetUpdated(updated time.Time) {
	p.Metadata.Updated = updated
}

// Helper methods for SimplePipeline

// ToSimplePipeline converts a full Pipeline to SimplePipeline format
func (p *Pipeline) ToSimplePipeline() SimplePipeline {
	return SimplePipeline{
		Name:        p.Pipeline.Name,
		Version:     p.Pipeline.Version,
		Created:     p.Metadata.Created,
		Updated:     p.Metadata.Updated,
		AccountID:   p.AccountID,
		AccountName: p.AccountName,
		Source:      p.GetSource(),
	}
}

// FromSimplePipeline creates a Pipeline from SimplePipeline format
func FromSimplePipeline(sp SimplePipeline) Pipeline {
	pipeline := Pipeline{
		Pipeline: PipelineDeclaration{
			Name:    sp.Name,
			Version: sp.Version,
			Stages:  []Stage{},
		},
		Metadata: PipelineMetadata{
			Created: sp.Created,
			Updated: sp.Updated,
		},
		AccountID:   sp.AccountID,
		AccountName: sp.AccountName,
	}

	// Create a basic source stage from the source configuration
	if sp.Source.Provider != "" {
		sourceAction := Action{
			Name: "source",
			ActionTypeId: ActionTypeId{
				Category: "Source",
				Owner:    "AWS",
				Provider: sp.Source.Provider,
				Version:  "1",
			},
			RunOrder:        1,
			Configuration:   sp.Source.Configuration,
			OutputArtifacts: []Artifact{{Name: "source"}},
			InputArtifacts:  []Artifact{},
		}

		sourceStage := Stage{
			Name:    "Source",
			Actions: []Action{sourceAction},
		}

		pipeline.Pipeline.Stages = append(pipeline.Pipeline.Stages, sourceStage)
	}

	return pipeline
}

// GetName returns the simple pipeline name
func (sp *SimplePipeline) GetName() string {
	return sp.Name
}

// GetVersion returns the simple pipeline version
func (sp *SimplePipeline) GetVersion() int32 {
	return sp.Version
}

// GetCreated returns the simple pipeline creation time
func (sp *SimplePipeline) GetCreated() time.Time {
	return sp.Created
}

// GetUpdated returns the simple pipeline update time
func (sp *SimplePipeline) GetUpdated() time.Time {
	return sp.Updated
}

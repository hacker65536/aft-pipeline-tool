package utils

import (
	"testing"
	"time"

	"github.com/hacker65536/aft-pipeline-tool/internal/models"
)

func TestGetPipelineStateInfo(t *testing.T) {
	tests := []struct {
		name     string
		pipeline models.Pipeline
		expected string
	}{
		{
			name: "No state",
			pipeline: models.Pipeline{
				State: nil,
			},
			expected: "No state",
		},
		{
			name: "No stages",
			pipeline: models.Pipeline{
				State: &models.PipelineState{
					StageStates: []models.StageState{},
				},
			},
			expected: "No stages",
		},
		{
			name: "First stage not succeeded",
			pipeline: models.Pipeline{
				State: &models.PipelineState{
					StageStates: []models.StageState{
						{
							StageName: "Source",
							LatestExecution: &models.StageExecution{
								PipelineExecutionId: "exec-123",
								Status:              "Failed",
							},
						},
						{
							StageName: "Build",
							LatestExecution: &models.StageExecution{
								PipelineExecutionId: "exec-123",
								Status:              "Succeeded",
							},
						},
					},
				},
			},
			expected: "0:Failed",
		},
		{
			name: "Second stage not succeeded",
			pipeline: models.Pipeline{
				State: &models.PipelineState{
					StageStates: []models.StageState{
						{
							StageName: "Source",
							LatestExecution: &models.StageExecution{
								PipelineExecutionId: "exec-123",
								Status:              "Succeeded",
							},
						},
						{
							StageName: "Build",
							LatestExecution: &models.StageExecution{
								PipelineExecutionId: "exec-123",
								Status:              "InProgress",
							},
						},
					},
				},
			},
			expected: "1:InProgress",
		},
		{
			name: "No execution info",
			pipeline: models.Pipeline{
				State: &models.PipelineState{
					StageStates: []models.StageState{
						{
							StageName:       "Source",
							LatestExecution: nil,
						},
					},
				},
			},
			expected: "0:NoExecution",
		},
		{
			name: "All succeeded with same execution ID",
			pipeline: models.Pipeline{
				State: &models.PipelineState{
					StageStates: []models.StageState{
						{
							StageName: "Source",
							LatestExecution: &models.StageExecution{
								PipelineExecutionId: "exec-123",
								Status:              "Succeeded",
							},
						},
						{
							StageName: "Build",
							LatestExecution: &models.StageExecution{
								PipelineExecutionId: "exec-123",
								Status:              "Succeeded",
							},
						},
						{
							StageName: "Deploy",
							LatestExecution: &models.StageExecution{
								PipelineExecutionId: "exec-123",
								Status:              "Succeeded",
							},
						},
					},
				},
			},
			expected: "Succeeded",
		},
		{
			name: "All succeeded but different execution IDs",
			pipeline: models.Pipeline{
				State: &models.PipelineState{
					StageStates: []models.StageState{
						{
							StageName: "Source",
							LatestExecution: &models.StageExecution{
								PipelineExecutionId: "exec-123",
								Status:              "Succeeded",
							},
						},
						{
							StageName: "Build",
							LatestExecution: &models.StageExecution{
								PipelineExecutionId: "exec-456",
								Status:              "Succeeded",
							},
						},
					},
				},
			},
			expected: "1:DiffExecId",
		},
		{
			name: "Mixed execution info availability",
			pipeline: models.Pipeline{
				State: &models.PipelineState{
					StageStates: []models.StageState{
						{
							StageName: "Source",
							LatestExecution: &models.StageExecution{
								PipelineExecutionId: "exec-123",
								Status:              "Succeeded",
							},
						},
						{
							StageName:       "Build",
							LatestExecution: nil,
						},
					},
				},
			},
			expected: "1:NoExecution",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getPipelineStateInfo(tt.pipeline)
			if result != tt.expected {
				t.Errorf("getPipelineStateInfo() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

func TestGetLatestStageUpdateTime(t *testing.T) {
	// Test timezone conversion functionality
	utcTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

	tests := []struct {
		name     string
		pipeline models.Pipeline
		expected string
	}{
		{
			name: "No latest stage update time",
			pipeline: models.Pipeline{
				State: nil,
			},
			expected: "N/A",
		},
		{
			name: "With latest stage update time - should convert to local timezone",
			pipeline: models.Pipeline{
				State: &models.PipelineState{
					StageStates: []models.StageState{
						{
							StageName: "Source",
							ActionStates: []models.ActionState{
								{
									ActionName: "SourceAction",
									LatestExecution: &models.ActionExecution{
										LastStatusChange: &utcTime,
									},
								},
							},
						},
					},
				},
			},
			expected: utcTime.Local().Format("2006-01-02 15:04-0700"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getLatestStageUpdateTime(tt.pipeline)
			if result != tt.expected {
				t.Errorf("getLatestStageUpdateTime() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

package cmd

import (
	"testing"
	"time"

	"github.com/hacker65536/aft-pipeline-tool/internal/models"
	"github.com/hacker65536/aft-pipeline-tool/internal/utils"
)

func TestSortPipelinesByLatestStageUpdate(t *testing.T) {
	// Create test time points
	now := time.Now()
	oneHourAgo := now.Add(-1 * time.Hour)
	twoHoursAgo := now.Add(-2 * time.Hour)

	// Create test pipelines with different latest stage update times
	pipelines := []models.Pipeline{
		{
			Pipeline: models.PipelineDeclaration{Name: "pipeline-old"},
			State: &models.PipelineState{
				StageStates: []models.StageState{
					{
						ActionStates: []models.ActionState{
							{
								LatestExecution: &models.ActionExecution{
									LastStatusChange: &twoHoursAgo,
								},
							},
						},
					},
				},
			},
		},
		{
			Pipeline: models.PipelineDeclaration{Name: "pipeline-newest"},
			State: &models.PipelineState{
				StageStates: []models.StageState{
					{
						ActionStates: []models.ActionState{
							{
								LatestExecution: &models.ActionExecution{
									LastStatusChange: &now,
								},
							},
						},
					},
				},
			},
		},
		{
			Pipeline: models.PipelineDeclaration{Name: "pipeline-middle"},
			State: &models.PipelineState{
				StageStates: []models.StageState{
					{
						ActionStates: []models.ActionState{
							{
								LatestExecution: &models.ActionExecution{
									LastStatusChange: &oneHourAgo,
								},
							},
						},
					},
				},
			},
		},
		{
			Pipeline: models.PipelineDeclaration{Name: "pipeline-no-state"},
			State:    nil,
		},
	}

	// Sort the pipelines
	utils.SortPipelinesByLatestStageUpdate(pipelines)

	// Verify the order: newest first, then middle, then old, then no-state
	expectedOrder := []string{
		"pipeline-newest",
		"pipeline-middle",
		"pipeline-old",
		"pipeline-no-state",
	}

	for i, expected := range expectedOrder {
		if pipelines[i].GetName() != expected {
			t.Errorf("Expected pipeline at index %d to be %s, but got %s", i, expected, pipelines[i].GetName())
		}
	}
}

func TestSortPipelinesByLatestStageUpdateAllNil(t *testing.T) {
	// Test case where all pipelines have no state information
	pipelines := []models.Pipeline{
		{Pipeline: models.PipelineDeclaration{Name: "pipeline-a"}, State: nil},
		{Pipeline: models.PipelineDeclaration{Name: "pipeline-b"}, State: nil},
		{Pipeline: models.PipelineDeclaration{Name: "pipeline-c"}, State: nil},
	}

	originalOrder := make([]string, len(pipelines))
	for i, p := range pipelines {
		originalOrder[i] = p.GetName()
	}

	// Sort the pipelines
	utils.SortPipelinesByLatestStageUpdate(pipelines)

	// Verify the order remains the same (stable sort)
	for i, expected := range originalOrder {
		if pipelines[i].GetName() != expected {
			t.Errorf("Expected pipeline at index %d to be %s, but got %s", i, expected, pipelines[i].GetName())
		}
	}
}

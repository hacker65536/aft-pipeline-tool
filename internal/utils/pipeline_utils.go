package utils

import (
	"sort"

	"github.com/hacker65536/aft-pipeline-tool/internal/models"
)

// SortPipelinesByLatestStageUpdate sorts pipelines by latest stage update time in descending order (newest first)
func SortPipelinesByLatestStageUpdate(pipelines []models.Pipeline) {
	sort.Slice(pipelines, func(i, j int) bool {
		timeI := pipelines[i].GetLatestStageUpdateTime()
		timeJ := pipelines[j].GetLatestStageUpdateTime()

		// Handle nil cases: pipelines with no stage update time go to the end
		if timeI == nil && timeJ == nil {
			// If both are nil, maintain original order (stable sort)
			return false
		}
		if timeI == nil {
			// i has no time, j should come first
			return false
		}
		if timeJ == nil {
			// j has no time, i should come first
			return true
		}

		// Both have times, sort by newest first (descending order)
		return timeI.After(*timeJ)
	})
}

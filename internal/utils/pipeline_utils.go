package utils

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/hacker65536/aft-pipeline-tool/internal/models"
	"github.com/hacker65536/aft-pipeline-tool/pkg/aft"
)

// ExtractAccountIDFromPipelineName extracts account ID from pipeline name
// This function was duplicated across fix_connections.go, fix_triggers.go, and fix_pipeline_type.go
func ExtractAccountIDFromPipelineName(pipelineName string) string {
	parts := strings.Split(pipelineName, "-")
	if len(parts) > 0 {
		return parts[0]
	}
	return ""
}

// SetPipelineAccountInfo sets AccountID and AccountName for a pipeline
// This logic was duplicated across multiple fix commands
func SetPipelineAccountInfo(ctx context.Context, pipeline *models.Pipeline, manager *aft.Manager) error {
	// Extract AccountID from pipeline name
	accountID := ExtractAccountIDFromPipelineName(pipeline.GetName())
	pipeline.AccountID = accountID

	// Get accounts to find AccountName
	accounts, err := manager.GetAccounts(ctx)
	if err != nil {
		return fmt.Errorf("failed to get accounts: %w", err)
	}

	// Find AccountName from AccountID
	for _, account := range accounts {
		if account.ID == accountID {
			pipeline.AccountName = account.Name
			break
		}
	}

	return nil
}

// ClearPipelineCache clears cache for a pipeline with proper error handling and logging
// This logic was duplicated across multiple fix commands
func ClearPipelineCache(pipelineName string, fileCache interface{}) {
	if fileCache == nil {
		return
	}

	fmt.Println("  Clearing cache for updated pipeline...")

	// Type assertion to get the actual cache interface
	if cache, ok := fileCache.(interface{ DeletePipelineCache(string) error }); ok {
		if err := cache.DeletePipelineCache(pipelineName); err != nil {
			fmt.Printf("  Warning: Failed to clear cache for pipeline %s: %v\n", pipelineName, err)
		} else {
			fmt.Println("  Pipeline cache cleared successfully")
		}
	}
}

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

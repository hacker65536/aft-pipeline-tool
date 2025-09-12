package aws

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/organizations"
	"github.com/aws/aws-sdk-go-v2/service/organizations/types"

	"github.com/hacker65536/aft-pipeline-tool/internal/models"
)

// ListAccounts retrieves all active AWS accounts from Organizations
func (c *Client) ListAccounts(ctx context.Context) ([]models.Account, error) {
	var accounts []models.Account

	paginator := organizations.NewListAccountsPaginator(c.Organizations, &organizations.ListAccountsInput{})

	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list accounts: %w", err)
		}

		for _, account := range output.Accounts {
			if account.Status == types.AccountStatusActive {
				name := strings.ReplaceAll(*account.Name, " ", "_")
				accounts = append(accounts, models.Account{
					ID:     *account.Id,
					Name:   name,
					Email:  *account.Email,
					Status: string(account.Status),
				})
			}
		}
	}

	return accounts, nil
}

// FilterAFTAccounts filters out excluded accounts from the account list
func (c *Client) FilterAFTAccounts(accounts []models.Account, excludeIDs []string) []models.Account {
	excludeMap := make(map[string]bool)
	for _, id := range excludeIDs {
		excludeMap[id] = true
	}

	var filtered []models.Account
	for _, account := range accounts {
		if !excludeMap[account.ID] {
			filtered = append(filtered, account)
		}
	}

	return filtered
}

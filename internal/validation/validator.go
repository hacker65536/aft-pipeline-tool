package validation

import (
	"fmt"
	"strings"

	"github.com/hacker65536/aft-pipeline-tool/internal/errors"
)

// Validator provides validation functions for various inputs
type Validator struct{}

// NewValidator creates a new validator instance
func NewValidator() *Validator {
	return &Validator{}
}

// ValidateConnectionArn validates AWS CodeConnections ARN format
func (v *Validator) ValidateConnectionArn(connectionArn string) error {
	if connectionArn == "" {
		return errors.NewValidationError("connection-arn is required", nil)
	}

	if !strings.HasPrefix(connectionArn, "arn:aws:codeconnections:") {
		return errors.NewValidationError(
			"invalid connection-arn format. Expected format: arn:aws:codeconnections:region:account:connection/connection-id",
			nil,
		)
	}

	return nil
}

// ValidateSourceAction validates source action name
func (v *Validator) ValidateSourceAction(sourceActionName string) error {
	if sourceActionName == "" {
		return nil // Optional parameter
	}

	validSourceActions := []string{"aft-global-customizations", "aft-account-customizations"}
	for _, action := range validSourceActions {
		if sourceActionName == action {
			return nil
		}
	}

	return errors.NewValidationError(
		fmt.Sprintf("invalid source-action '%s'. Valid options: %v", sourceActionName, validSourceActions),
		nil,
	)
}

// ValidatePipelineType validates pipeline type
func (v *Validator) ValidatePipelineType(pipelineType string) error {
	if pipelineType == "" {
		return errors.NewValidationError("pipeline-type is required", nil)
	}

	validTypes := []string{"V1", "V2"}
	for _, validType := range validTypes {
		if pipelineType == validType {
			return nil
		}
	}

	return errors.NewValidationError(
		fmt.Sprintf("invalid pipeline-type '%s'. Valid options: %v", pipelineType, validTypes),
		nil,
	)
}

// ValidateTriggerMode validates trigger mode
func (v *Validator) ValidateTriggerMode(mode string) error {
	if mode == "" {
		return nil // Optional parameter
	}

	validModes := []string{"auto", "single", "none"}
	for _, validMode := range validModes {
		if mode == validMode {
			return nil
		}
	}

	return errors.NewValidationError(
		fmt.Sprintf("invalid trigger mode '%s'. Valid options: %v", mode, validModes),
		nil,
	)
}

// ValidatePipelineName validates pipeline name format
func (v *Validator) ValidatePipelineName(pipelineName string) error {
	if pipelineName == "" {
		return errors.NewValidationError("pipeline name cannot be empty", nil)
	}

	// AFT pipeline naming convention: aft-account-{account-id}-{type}
	if !strings.HasPrefix(pipelineName, "aft-account-") {
		return errors.NewValidationError(
			"invalid pipeline name format. Expected format: aft-account-{account-id}-{type}",
			nil,
		)
	}

	parts := strings.Split(pipelineName, "-")
	if len(parts) < 4 {
		return errors.NewValidationError(
			"invalid pipeline name format. Expected format: aft-account-{account-id}-{type}",
			nil,
		)
	}

	return nil
}

// ValidateAccountID validates AWS account ID format
func (v *Validator) ValidateAccountID(accountID string) error {
	if accountID == "" {
		return errors.NewValidationError("account ID cannot be empty", nil)
	}

	if len(accountID) != 12 {
		return errors.NewValidationError("account ID must be 12 digits", nil)
	}

	for _, char := range accountID {
		if char < '0' || char > '9' {
			return errors.NewValidationError("account ID must contain only digits", nil)
		}
	}

	return nil
}

// ValidateRequiredString validates that a string is not empty
func (v *Validator) ValidateRequiredString(value, fieldName string) error {
	if strings.TrimSpace(value) == "" {
		return errors.NewValidationError(fmt.Sprintf("%s is required", fieldName), nil)
	}
	return nil
}

// ValidateStringInSlice validates that a string is in a given slice
func (v *Validator) ValidateStringInSlice(value string, validValues []string, fieldName string) error {
	if value == "" {
		return nil // Allow empty values for optional fields
	}

	for _, validValue := range validValues {
		if value == validValue {
			return nil
		}
	}

	return errors.NewValidationError(
		fmt.Sprintf("invalid %s '%s'. Valid options: %v", fieldName, value, validValues),
		nil,
	)
}

package validation

import (
	"testing"

	"github.com/hacker65536/aft-pipeline-tool/internal/errors"
)

func TestValidator_ValidateConnectionArn(t *testing.T) {
	validator := NewValidator()

	tests := []struct {
		name          string
		connectionArn string
		wantErr       bool
		errType       errors.ErrorType
	}{
		{
			name:          "valid connection ARN",
			connectionArn: "arn:aws:codeconnections:us-east-1:123456789012:connection/12345678-1234-1234-1234-123456789012",
			wantErr:       false,
		},
		{
			name:          "empty connection ARN",
			connectionArn: "",
			wantErr:       true,
			errType:       errors.ErrorTypeValidation,
		},
		{
			name:          "invalid connection ARN format",
			connectionArn: "arn:aws:codepipeline:us-east-1:123456789012:pipeline/test",
			wantErr:       true,
			errType:       errors.ErrorTypeValidation,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.ValidateConnectionArn(tt.connectionArn)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateConnectionArn() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil && tt.wantErr {
				if !errors.IsType(err, tt.errType) {
					t.Errorf("ValidateConnectionArn() error type = %T, want %v", err, tt.errType)
				}
			}
		})
	}
}

func TestValidator_ValidateSourceAction(t *testing.T) {
	validator := NewValidator()

	tests := []struct {
		name             string
		sourceActionName string
		wantErr          bool
	}{
		{
			name:             "valid source action - global customizations",
			sourceActionName: "aft-global-customizations",
			wantErr:          false,
		},
		{
			name:             "valid source action - account customizations",
			sourceActionName: "aft-account-customizations",
			wantErr:          false,
		},
		{
			name:             "empty source action (optional)",
			sourceActionName: "",
			wantErr:          false,
		},
		{
			name:             "invalid source action",
			sourceActionName: "invalid-action",
			wantErr:          true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.ValidateSourceAction(tt.sourceActionName)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateSourceAction() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidator_ValidatePipelineType(t *testing.T) {
	validator := NewValidator()

	tests := []struct {
		name         string
		pipelineType string
		wantErr      bool
	}{
		{
			name:         "valid pipeline type V1",
			pipelineType: "V1",
			wantErr:      false,
		},
		{
			name:         "valid pipeline type V2",
			pipelineType: "V2",
			wantErr:      false,
		},
		{
			name:         "empty pipeline type",
			pipelineType: "",
			wantErr:      true,
		},
		{
			name:         "invalid pipeline type",
			pipelineType: "V3",
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.ValidatePipelineType(tt.pipelineType)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidatePipelineType() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidator_ValidateTriggerMode(t *testing.T) {
	validator := NewValidator()

	tests := []struct {
		name    string
		mode    string
		wantErr bool
	}{
		{
			name:    "valid trigger mode auto",
			mode:    "auto",
			wantErr: false,
		},
		{
			name:    "valid trigger mode single",
			mode:    "single",
			wantErr: false,
		},
		{
			name:    "valid trigger mode none",
			mode:    "none",
			wantErr: false,
		},
		{
			name:    "empty trigger mode (optional)",
			mode:    "",
			wantErr: false,
		},
		{
			name:    "invalid trigger mode",
			mode:    "invalid",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.ValidateTriggerMode(tt.mode)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateTriggerMode() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidator_ValidatePipelineName(t *testing.T) {
	validator := NewValidator()

	tests := []struct {
		name         string
		pipelineName string
		wantErr      bool
	}{
		{
			name:         "valid pipeline name",
			pipelineName: "aft-account-123456789012-customizations",
			wantErr:      false,
		},
		{
			name:         "valid pipeline name with global",
			pipelineName: "aft-account-123456789012-global-customizations",
			wantErr:      false,
		},
		{
			name:         "empty pipeline name",
			pipelineName: "",
			wantErr:      true,
		},
		{
			name:         "invalid pipeline name format",
			pipelineName: "invalid-pipeline-name",
			wantErr:      true,
		},
		{
			name:         "incomplete pipeline name",
			pipelineName: "aft-account",
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.ValidatePipelineName(tt.pipelineName)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidatePipelineName() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidator_ValidateAccountID(t *testing.T) {
	validator := NewValidator()

	tests := []struct {
		name      string
		accountID string
		wantErr   bool
	}{
		{
			name:      "valid account ID",
			accountID: "123456789012",
			wantErr:   false,
		},
		{
			name:      "empty account ID",
			accountID: "",
			wantErr:   true,
		},
		{
			name:      "account ID too short",
			accountID: "12345678901",
			wantErr:   true,
		},
		{
			name:      "account ID too long",
			accountID: "1234567890123",
			wantErr:   true,
		},
		{
			name:      "account ID with non-digits",
			accountID: "12345678901a",
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.ValidateAccountID(tt.accountID)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateAccountID() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidator_ValidateRequiredString(t *testing.T) {
	validator := NewValidator()

	tests := []struct {
		name      string
		value     string
		fieldName string
		wantErr   bool
	}{
		{
			name:      "valid required string",
			value:     "test-value",
			fieldName: "test-field",
			wantErr:   false,
		},
		{
			name:      "empty required string",
			value:     "",
			fieldName: "test-field",
			wantErr:   true,
		},
		{
			name:      "whitespace only string",
			value:     "   ",
			fieldName: "test-field",
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.ValidateRequiredString(tt.value, tt.fieldName)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateRequiredString() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidator_ValidateStringInSlice(t *testing.T) {
	validator := NewValidator()

	tests := []struct {
		name        string
		value       string
		validValues []string
		fieldName   string
		wantErr     bool
	}{
		{
			name:        "valid value in slice",
			value:       "option1",
			validValues: []string{"option1", "option2", "option3"},
			fieldName:   "test-field",
			wantErr:     false,
		},
		{
			name:        "empty value (optional)",
			value:       "",
			validValues: []string{"option1", "option2", "option3"},
			fieldName:   "test-field",
			wantErr:     false,
		},
		{
			name:        "invalid value not in slice",
			value:       "invalid-option",
			validValues: []string{"option1", "option2", "option3"},
			fieldName:   "test-field",
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.ValidateStringInSlice(tt.value, tt.validValues, tt.fieldName)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateStringInSlice() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

package messaging

import (
	"errors"
	"testing"
)

func TestValidateHandlerResult(t *testing.T) {
	testErr := errors.New("processing failed")

	tests := []struct {
		name    string
		result  ProcessingResult
		err     error
		wantErr bool
	}{
		{
			name:    "success without error",
			result:  ProcessingSuccess,
			err:     nil,
			wantErr: false,
		},
		{
			name:    "success with error",
			result:  ProcessingSuccess,
			err:     testErr,
			wantErr: true,
		},
		{
			name:    "retry with error",
			result:  ProcessingRetry,
			err:     testErr,
			wantErr: false,
		},
		{
			name:    "reject with error",
			result:  ProcessingReject,
			err:     testErr,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateHandlerResult(
				tt.result,
				tt.err,
			)

			if (err != nil) != tt.wantErr {
				t.Fatalf(
					"expected error=%v, got error=%v",
					tt.wantErr,
					err,
				)
			}
		})
	}
}

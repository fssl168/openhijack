package errors

import (
	"errors"
	"testing"
)

func TestAppError_Is(t *testing.T) {
	tests := []struct {
		name     string
		err      *AppError
		target   error
		expected bool
	}{
		{
			name:     "same error code",
			err:      New(ErrNotFound, "not found"),
			target:   New(ErrNotFound, "another message"),
			expected: true,
		},
		{
			name:     "different error code",
			err:      New(ErrNotFound, "not found"),
			target:   New(ErrPermissionDenied, "permission denied"),
			expected: false,
		},
		{
			name:     "nil target",
			err:      New(ErrNotFound, "not found"),
			target:   nil,
			expected: false,
		},
		{
			name:     "non-AppError target",
			err:      New(ErrNotFound, "not found"),
			target:   errors.New("standard error"),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.err.Is(tt.target)
			if result != tt.expected {
				t.Errorf("Is() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

func TestAppError_IsCode(t *testing.T) {
	err := New(ErrNotFound, "resource not found")

	if !err.IsCode(ErrNotFound) {
		t.Error("IsCode should return true for matching error code")
	}

	if err.IsCode(ErrPermissionDenied) {
		t.Error("IsCode should return false for non-matching error code")
	}
}

func TestErrorsIs_Compatibility(t *testing.T) {
	err := New(ErrAuthenticationFailed, "auth failed")
	target := New(ErrAuthenticationFailed, "also auth failed")

	if !errors.Is(err, target) {
		t.Error("errors.Is should work with AppError")
	}

	differentErr := New(ErrInternalError, "internal error")
	if errors.Is(err, differentErr) {
		t.Error("errors.Is should return false for different error codes")
	}
}

func TestAsAppError(t *testing.T) {
	appErr := New(ErrNotFound, "test error")

	result, ok := AsAppError(appErr)
	if !ok || result != appErr {
		t.Error("AsAppError should return the same error for *AppError")
	}

	stdErr := errors.New("standard error")
	result, ok = AsAppError(stdErr)
	if ok {
		t.Error("AsAppError should return false for non-*AppError")
	}
}

func TestIsErrorCode(t *testing.T) {
	appErr := New(ErrConfigFileNotFound, "config missing")

	if !IsErrorCode(appErr, ErrConfigFileNotFound) {
		t.Error("IsErrorCode should return true for matching code")
	}

	if IsErrorCode(appErr, ErrNotFound) {
		t.Error("IsErrorCode should return false for non-matching code")
	}

	stdErr := errors.New("standard error")
	if IsErrorCode(stdErr, ErrNotFound) {
		t.Error("IsErrorCode should return false for non-*AppError")
	}
}

package errors

import (
	"errors"
	"fmt"
)

// ErrorCode represents a specific error type
type ErrorCode string

const (
	// Storage errors
	ErrCodeStorageFull      ErrorCode = "STORAGE_FULL"
	ErrCodeStorageCorrupted ErrorCode = "STORAGE_CORRUPTED"
	ErrCodeChunkNotFound    ErrorCode = "CHUNK_NOT_FOUND"
	ErrCodeChunkInvalid     ErrorCode = "CHUNK_INVALID"

	// Network errors
	ErrCodeNetworkTimeout     ErrorCode = "NETWORK_TIMEOUT"
	ErrCodeNetworkUnreachable ErrorCode = "NETWORK_UNREACHABLE"
	ErrCodePeerNotFound       ErrorCode = "PEER_NOT_FOUND"
	ErrCodeConnectionFailed   ErrorCode = "CONNECTION_FAILED"

	// Crypto errors
	ErrCodeEncryptionFailed ErrorCode = "ENCRYPTION_FAILED"
	ErrCodeDecryptionFailed ErrorCode = "DECRYPTION_FAILED"
	ErrCodeInvalidSignature ErrorCode = "INVALID_SIGNATURE"
	ErrCodeInvalidKey       ErrorCode = "INVALID_KEY"

	// Snapshot errors
	ErrCodeSnapshotNotFound  ErrorCode = "SNAPSHOT_NOT_FOUND"
	ErrCodeSnapshotCorrupted ErrorCode = "SNAPSHOT_CORRUPTED"
	ErrCodeSnapshotInvalid   ErrorCode = "SNAPSHOT_INVALID"

	// Configuration errors
	ErrCodeConfigInvalid ErrorCode = "CONFIG_INVALID"
	ErrCodeConfigMissing ErrorCode = "CONFIG_MISSING"

	// Permission errors
	ErrCodePermissionDenied ErrorCode = "PERMISSION_DENIED"
	ErrCodeUnauthorized     ErrorCode = "UNAUTHORIZED"

	// Resource errors
	ErrCodeResourceExhausted ErrorCode = "RESOURCE_EXHAUSTED"
	ErrCodeRateLimitExceeded ErrorCode = "RATE_LIMIT_EXCEEDED"
)

// ShadowVaultError is the base error type for all ShadowVault errors
type ShadowVaultError struct {
	Code       ErrorCode
	Message    string
	Err        error
	Retryable  bool
	StatusCode int
}

// Error implements the error interface
func (e *ShadowVaultError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

// Unwrap returns the wrapped error
func (e *ShadowVaultError) Unwrap() error {
	return e.Err
}

// Is checks if this error matches the target
func (e *ShadowVaultError) Is(target error) bool {
	t, ok := target.(*ShadowVaultError)
	if !ok {
		return false
	}
	return e.Code == t.Code
}

// NewError creates a new ShadowVault error
func NewError(code ErrorCode, message string) *ShadowVaultError {
	return &ShadowVaultError{
		Code:       code,
		Message:    message,
		Retryable:  isRetryable(code),
		StatusCode: getStatusCode(code),
	}
}

// WrapError wraps an existing error
func WrapError(code ErrorCode, message string, err error) *ShadowVaultError {
	return &ShadowVaultError{
		Code:       code,
		Message:    message,
		Err:        err,
		Retryable:  isRetryable(code),
		StatusCode: getStatusCode(code),
	}
}

// isRetryable determines if an error should be retried
func isRetryable(code ErrorCode) bool {
	switch code {
	case ErrCodeNetworkTimeout, ErrCodeNetworkUnreachable,
		ErrCodeConnectionFailed, ErrCodeResourceExhausted:
		return true
	default:
		return false
	}
}

// getStatusCode returns the HTTP status code for an error
func getStatusCode(code ErrorCode) int {
	switch code {
	case ErrCodePermissionDenied, ErrCodeUnauthorized:
		return 403
	case ErrCodeSnapshotNotFound, ErrCodeChunkNotFound, ErrCodePeerNotFound:
		return 404
	case ErrCodeRateLimitExceeded:
		return 429
	case ErrCodeStorageFull, ErrCodeResourceExhausted:
		return 507
	case ErrCodeConfigInvalid, ErrCodeSnapshotInvalid, ErrCodeChunkInvalid:
		return 400
	default:
		return 500
	}
}

// Common error constructors

func NewStorageFullError(message string) *ShadowVaultError {
	return NewError(ErrCodeStorageFull, message)
}

func NewChunkNotFoundError(hash string) *ShadowVaultError {
	return NewError(ErrCodeChunkNotFound, fmt.Sprintf("chunk not found: %s", hash))
}

func NewNetworkTimeoutError(message string) *ShadowVaultError {
	return NewError(ErrCodeNetworkTimeout, message)
}

func NewEncryptionFailedError(err error) *ShadowVaultError {
	return WrapError(ErrCodeEncryptionFailed, "encryption failed", err)
}

func NewDecryptionFailedError(err error) *ShadowVaultError {
	return WrapError(ErrCodeDecryptionFailed, "decryption failed", err)
}

func NewInvalidSignatureError(message string) *ShadowVaultError {
	return NewError(ErrCodeInvalidSignature, message)
}

func NewSnapshotNotFoundError(id string) *ShadowVaultError {
	return NewError(ErrCodeSnapshotNotFound, fmt.Sprintf("snapshot not found: %s", id))
}

func NewSnapshotCorruptedError(id string) *ShadowVaultError {
	return NewError(ErrCodeSnapshotCorrupted, fmt.Sprintf("snapshot corrupted: %s", id))
}

func NewPermissionDeniedError(message string) *ShadowVaultError {
	return NewError(ErrCodePermissionDenied, message)
}

func NewRateLimitExceededError() *ShadowVaultError {
	return NewError(ErrCodeRateLimitExceeded, "rate limit exceeded")
}

// IsRetryable checks if an error should be retried
func IsRetryable(err error) bool {
	var svErr *ShadowVaultError
	if errors.As(err, &svErr) {
		return svErr.Retryable
	}
	return false
}

// GetErrorCode extracts the error code from an error
func GetErrorCode(err error) ErrorCode {
	var svErr *ShadowVaultError
	if errors.As(err, &svErr) {
		return svErr.Code
	}
	return ""
}

// GetStatusCode extracts the HTTP status code from an error
func GetStatusCode(err error) int {
	var svErr *ShadowVaultError
	if errors.As(err, &svErr) {
		return svErr.StatusCode
	}
	return 500
}

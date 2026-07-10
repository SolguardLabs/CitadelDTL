package domain

import "fmt"

type ErrorCode string

const (
	CodeValidation        ErrorCode = "validation"
	CodeNotFound          ErrorCode = "not_found"
	CodeConflict          ErrorCode = "conflict"
	CodeInsufficientFunds ErrorCode = "insufficient_funds"
	CodeUnauthorized      ErrorCode = "unauthorized"
	CodeLimitExceeded     ErrorCode = "limit_exceeded"
	CodeInvariant         ErrorCode = "invariant"
)

type AppError struct {
	Code    ErrorCode `json:"code"`
	Reason  string    `json:"reason"`
	Message string    `json:"message"`
}

func (e *AppError) Error() string {
	if e == nil {
		return ""
	}
	if e.Reason == "" {
		return fmt.Sprintf("%s: %s", e.Code, e.Message)
	}
	return fmt.Sprintf("%s:%s: %s", e.Code, e.Reason, e.Message)
}

func NewError(code ErrorCode, reason string, message string) *AppError {
	return &AppError{Code: code, Reason: reason, Message: message}
}

func NewValidationError(reason string, message string) *AppError {
	return NewError(CodeValidation, reason, message)
}

func NewNotFoundError(reason string, message string) *AppError {
	return NewError(CodeNotFound, reason, message)
}

func NewConflictError(reason string, message string) *AppError {
	return NewError(CodeConflict, reason, message)
}

func NewInsufficientFundsError(reason string, message string) *AppError {
	return NewError(CodeInsufficientFunds, reason, message)
}

func NewUnauthorizedError(reason string, message string) *AppError {
	return NewError(CodeUnauthorized, reason, message)
}

func NewLimitExceededError(reason string, message string) *AppError {
	return NewError(CodeLimitExceeded, reason, message)
}

func NewInvariantError(reason string, message string) *AppError {
	return NewError(CodeInvariant, reason, message)
}

func ErrorReason(err error) string {
	if err == nil {
		return ""
	}
	if app, ok := err.(*AppError); ok {
		if app.Reason != "" {
			return app.Reason
		}
		return string(app.Code)
	}
	return err.Error()
}

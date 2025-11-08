package errors

import "errors"

// AppError encodes domain specific error details.
type AppError struct {
	Code    string
	Message string
	Err     error
}

func (e *AppError) Error() string {
	if e.Err != nil {
		return e.Message + ": " + e.Err.Error()
	}
	return e.Message
}

func (e *AppError) Unwrap() error {
	return e.Err
}

// Wrap produces a new AppError instance.
func Wrap(code, message string, err error) error {
	if err == nil {
		return &AppError{Code: code, Message: message}
	}
	return &AppError{Code: code, Message: message, Err: err}
}

// IsCode helps handler differentiate failures.
func IsCode(err error, code string) bool {
	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr.Code == code
	}
	return false
}

package trip

import "errors"

type ValidationError struct {
	Field   string
	Message string
}

var (
	ErrTripNotFound  = errors.New("trip not found")
	ErrTripCompleted = errors.New("trip already completed")
	ErrDriverBusy    = errors.New("driver already has an active trip")
)

func (e *ValidationError) Error() string {
	return e.Field + ":" + e.Message
}

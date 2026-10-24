package trip

import domaintrip "github.com/deadOk-core/GoTemplate/internal/domain/trip"

var (
	ErrTripNotFound  = domaintrip.ErrTripNotFound
	ErrTripCompleted = domaintrip.ErrTripCompleted
	ErrDriverBusy    = domaintrip.ErrDriverBusy
)

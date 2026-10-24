package trip

import (
	domaintrip "github.com/deadOk-core/GoTemplate/internal/domain/trip"
	"github.com/deadOk-core/GoTemplate/internal/model"
)

type (
	Status        = domaintrip.Status
	Trip          = model.Trip
	StatusHistory = model.StatusHistory
	CreateParams  = domaintrip.CreateParams
)

const (
	StatusActive    = domaintrip.StatusActive
	StatusCompleted = domaintrip.StatusCompleted
)

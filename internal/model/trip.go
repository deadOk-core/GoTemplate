package model

import (
	"time"

	domaintrip "github.com/deadOk-core/GoTemplate/internal/domain/trip"
	"github.com/google/uuid"
)

type Trip struct {
	ID       uuid.UUID
	UserID   uuid.UUID
	DriverID uuid.UUID

	StartLatitude  float64
	StartLongitude float64
	EndLatitude    float64
	EndLongitude   float64

	Price  int64
	Status domaintrip.Status

	StartedAt  time.Time
	FinishedAt *time.Time
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

type StatusHistory struct {
	TripID     uuid.UUID
	FromStatus *domaintrip.Status
	ToStatus   domaintrip.Status
	Reason     string
}

// domaintrip.Trip хранит состояние поездки и её поведение — например, правило Finish
// model.Trip передаёт данные между слоями. У него нет бизнес-метода Finish

func FromDomain(t domaintrip.Trip) Trip {
	return Trip(t)
}

func (t Trip) ToDomain() domaintrip.Trip {
	return domaintrip.Trip(t)
}

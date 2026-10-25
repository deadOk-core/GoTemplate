package create_test

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	domaintrip "github.com/deadOk-core/GoTemplate/internal/domain/trip"
	"github.com/deadOk-core/GoTemplate/internal/model"
	createusecase "github.com/deadOk-core/GoTemplate/internal/usecase/trip/create"
)

func TestUseCaseCreate(t *testing.T) {
	ctrl := gomock.NewController(t)
	repository := NewMocktripRepository(ctrl)
	manager := NewMocktxManager(ctrl)
	useCase := createusecase.NewUseCase(repository, manager, slog.New(slog.DiscardHandler))

	ctx := t.Context()
	txCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	params := validCreateParams()

	dbTime := time.Date(2026, time.October, 24, 12, 0, 0, 0, time.UTC)
	var savedTrip model.Trip

	manager.EXPECT().
		Do(ctx, gomock.Any()).
		DoAndReturn(func(
			_ context.Context,
			fn func(context.Context) error,
		) error {
			return fn(txCtx)
		})

	repository.EXPECT().
		Create(txCtx, gomock.Any()).
		DoAndReturn(func(
			_ context.Context,
			input model.Trip,
		) (model.Trip, error) {
			assert.NotEqual(t, uuid.Nil, input.ID)
			assert.Equal(t, params.UserID, input.UserID)
			assert.Equal(t, params.DriverID, input.DriverID)
			assert.Equal(t, params.Price, input.Price)
			assert.Equal(t, domaintrip.StatusActive, input.Status)
			assert.Nil(t, input.FinishedAt)

			savedTrip = input
			savedTrip.CreatedAt = dbTime
			savedTrip.UpdatedAt = dbTime

			return savedTrip, nil
		})

	repository.EXPECT().
		AddStatusHistory(txCtx, gomock.Any()).
		DoAndReturn(func(
			_ context.Context,
			history model.StatusHistory,
		) error {
			assert.Equal(t, savedTrip.ID, history.TripID)
			assert.Nil(t, history.FromStatus)
			assert.Equal(t, domaintrip.StatusActive, history.ToStatus)
			assert.Equal(t, "trip created", history.Reason)

			return nil
		})

	createdTrip, err := useCase.Create(ctx, params)

	require.NoError(t, err)
	assert.Equal(t, savedTrip, createdTrip)
}

func TestUseCaseCreateErrors(t *testing.T) {
	repositoryErr := errors.New("repository unavailable")
	historyErr := errors.New("cannot save status history")
	beginErr := errors.New("cannot begin transaction")
	commitErr := errors.New("cannot commit transaction")

	// Каждая строка таблицы задаёт один сценарий и место возникновения ошибки.
	// nil в поле ошибки означает, что соответствующая операция успешна.
	tests := []struct {
		name       string
		invalid    bool  // Передать невалидную цену и остановиться на валидации.
		createErr  error // Результат вызова repository.Create.
		historyErr error // Результат вызова repository.AddStatusHistory.
		beginErr   error // Сбой менеджера до запуска callback транзакции.
		commitErr  error // Сбой менеджера после успешного выполнения callback.
		wantErr    error // Исходная ошибка, которую должен вернуть usecase.
	}{
		// Валидация должна завершиться до обращения к внешним зависимостям.
		{
			name:    "invalid parameters",
			invalid: true,
		},
		// Занятый водитель — доменная ошибка, которую позже HTTP переведёт в 409.
		{
			name:      "driver busy",
			createErr: domaintrip.ErrDriverBusy,
			wantErr:   domaintrip.ErrDriverBusy,
		},
		// При техническом сбое создания поездки запись истории не выполняется.
		{
			name:      "repository error",
			createErr: repositoryErr,
			wantErr:   repositoryErr,
		},
		// Создание прошло, но ошибка истории должна выйти из callback наружу.
		{
			name:       "history error",
			historyErr: historyErr,
			wantErr:    historyErr,
		},
		// При сбое начала транзакции ни одна операция репозитория не запускается.
		{
			name:     "begin error",
			beginErr: beginErr,
			wantErr:  beginErr,
		},
		// Даже после двух успешных операций ошибка commit означает неуспех usecase.
		{
			name:      "commit error",
			commitErr: commitErr,
			wantErr:   commitErr,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// У каждого подтеста собственные моки и ожидания вызовов.
			ctrl := gomock.NewController(t)
			repository := NewMocktripRepository(ctrl)
			manager := NewMocktxManager(ctrl)
			useCase := createusecase.NewUseCase(repository, manager, slog.New(slog.DiscardHandler))

			// Usecase получает ctx, а репозиторий должен получить именно txCtx,
			// который менеджер передаёт в callback транзакции.
			ctx := t.Context()
			txCtx, cancel := context.WithCancel(ctx)
			defer cancel()

			params := validCreateParams()
			if tt.invalid {
				params.Price = -1
			}

			// Для невалидных параметров ожиданий не задаём: любой вызов зависимости
			// будет неожиданным для gomock и приведёт к провалу подтеста.
			if !tt.invalid {
				manager.EXPECT().
					Do(ctx, gomock.Any()).
					DoAndReturn(func(
						_ context.Context,
						fn func(context.Context) error,
					) error {
						// Имитируем ошибку начала транзакции: callback не вызывается.
						if tt.beginErr != nil {
							return tt.beginErr
						}

						// Выполняем операции usecase и возвращаем их ошибку менеджеру.
						// Настоящий откат в PostgreSQL проверен интеграционным тестом.
						if err := fn(txCtx); err != nil {
							return err
						}

						// Callback успешен: теперь моделируем результат commit.
						return tt.commitErr
					})

				if tt.beginErr == nil {
					// Непустой результат репозитория помогает поймать ошибочный
					// возврат поездки вызывающему коду при неуспешном создании.
					repository.EXPECT().
						Create(txCtx, gomock.Any()).
						Return(model.Trip{ID: uuid.New()}, tt.createErr)

					if tt.createErr == nil {
						// История допустима только после успешного Create.
						// При createErr ожидания нет: лишний вызов провалит тест.
						repository.EXPECT().
							AddStatusHistory(txCtx, gomock.Any()).
							Return(tt.historyErr)
					}
				}
			}

			// Запускаем настоящий usecase с подготовленными моками зависимостей.
			result, err := useCase.Create(ctx, params)

			// Ни один ошибочный сценарий не должен возвращать созданную поездку.
			assert.Equal(t, model.Trip{}, result)

			if tt.invalid {
				// У валидации отдельный тип ошибки: проверяем его и имя поля.
				var validationErr *domaintrip.ValidationError
				require.ErrorAs(t, err, &validationErr)
				assert.Equal(t, "price", validationErr.Field)
				return
			}

			// Проверяем сохранение исходной ошибки в цепочке обёрток.
			require.ErrorIs(t, err, tt.wantErr)
		})
	}
}

func validCreateParams() domaintrip.CreateParams {
	return domaintrip.CreateParams{
		UserID:         uuid.New(),
		DriverID:       uuid.New(),
		StartLatitude:  55.75,
		StartLongitude: 37.61,
		EndLatitude:    55.80,
		EndLongitude:   37.70,
		Price:          1500,
	}
}

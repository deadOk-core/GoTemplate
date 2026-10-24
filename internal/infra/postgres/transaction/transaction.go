package transaction

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type transactionKey struct{}

type DBTX interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

type TransactionManager struct {
	pool *pgxpool.Pool
}

func NewTransactionManager(pool *pgxpool.Pool) *TransactionManager {
	return &TransactionManager{
		pool: pool,
	}
}

func (m *TransactionManager) Do(ctx context.Context, fn func(ctx context.Context) error) error {
	// Если Do вызван внутри другого Do, транзакция уже находится в контексте.
	// Новую транзакцию не открываем: вложенная операция использует существующую.
	if _, ok := transactionFromContext(ctx); ok {
		return fn(ctx)
	}

	// Открываем транзакцию. Read Committed достаточно для ЛР1: конкурентное
	// создание защищает уникальный индекс, а завершение защитит условный UPDATE.
	tx, err := m.pool.BeginTx(ctx, pgx.TxOptions{
		IsoLevel: pgx.ReadCommitted,
	})
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}

	// Пока Commit не завершился успешно, при любом выходе из Do нужно сделать
	// Rollback. defer выполнится и при обычной ошибке, и при панике внутри fn.
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback(ctx)
		}
	}()

	// Создаём дочерний контекст и сохраняем в нём транзакцию. Репозиторий
	// достанет её через Executor и выполнит все запросы через один pgx.Tx.
	txCtx := context.WithValue(ctx, transactionKey{}, tx)

	// Запускаем бизнес-операции. Любая возвращённая ошибка выйдет из Do,
	// после чего отложенная функция выше выполнит Rollback.
	if err := fn(txCtx); err != nil {
		return err
	}

	// Все операции завершились успешно — подтверждаем изменения в PostgreSQL.
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	// Commit прошёл успешно, поэтому отложенный Rollback больше не нужен.
	committed = true
	return nil

}

func Executor(ctx context.Context, pool *pgxpool.Pool) DBTX {
	// Внутри функции Do контекст содержит pgx.Tx. Возвращаем её,
	// чтобы запрос стал частью общей транзакции и ждал Commit или Rollback.
	if tx, ok := transactionFromContext(ctx); ok {
		return tx
	}

	// Если транзакции в контексте нет, выполняем запрос обычным способом
	// через пул. Это подходит, например, для одиночного чтения GetByID.
	return pool
}

func transactionFromContext(ctx context.Context) (pgx.Tx, bool) {
	// Ищем значение по закрытому ключу transactionKey{} и безопасно проверяем,
	// что найденное значение реализует pgx.Tx. ok=false означает, что текущая
	// операция выполняется вне транзакции.
	tx, ok := ctx.Value(transactionKey{}).(pgx.Tx)
	return tx, ok
}

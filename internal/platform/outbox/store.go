package outbox

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"review-api/internal/platform/transaction"
)

type Store struct {
	pool *pgxpool.Pool
}

func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

func (s *Store) Append(ctx context.Context, msg Message) error {
	err := s.exec(ctx, `
		INSERT INTO outbox_messages (id, event_name, aggregate_type, aggregate_id, payload, status, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, msg.ID, msg.EventName, msg.AggregateType, msg.AggregateID, []byte(msg.Payload), string(msg.Status), msg.CreatedAt.UTC())
	if err != nil {
		return fmt.Errorf("append outbox: %w", err)
	}
	return nil
}

func (s *Store) ListPendingForUpdate(ctx context.Context, limit int) ([]Message, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.query(ctx, `
		SELECT id, event_name, aggregate_type, aggregate_id, payload, status, created_at, published_at
		FROM outbox_messages
		WHERE status = $1
		ORDER BY created_at ASC
		LIMIT $2
		FOR UPDATE SKIP LOCKED
	`, string(StatusPending), limit)
	if err != nil {
		return nil, fmt.Errorf("list pending outbox: %w", err)
	}
	defer rows.Close()

	var out []Message
	for rows.Next() {
		var m Message
		var status string
		var payload []byte
		if err := rows.Scan(&m.ID, &m.EventName, &m.AggregateType, &m.AggregateID, &payload, &status, &m.CreatedAt, &m.PublishedAt); err != nil {
			return nil, fmt.Errorf("scan outbox: %w", err)
		}
		m.Payload = payload
		m.Status = Status(status)
		out = append(out, m)
	}
	return out, rows.Err()
}

func (s *Store) MarkSent(ctx context.Context, id string, publishedAt time.Time) error {
	err := s.exec(ctx, `
		UPDATE outbox_messages
		SET status = $1, published_at = $2
		WHERE id = $3 AND status = $4
	`, string(StatusSent), publishedAt.UTC(), id, string(StatusPending))
	if err != nil {
		return fmt.Errorf("mark outbox sent: %w", err)
	}
	return nil
}

func (s *Store) MarkFailed(ctx context.Context, id string) error {
	err := s.exec(ctx, `
		UPDATE outbox_messages
		SET status = $1
		WHERE id = $2 AND status = $3
	`, string(StatusFailed), id, string(StatusPending))
	if err != nil {
		return fmt.Errorf("mark outbox failed: %w", err)
	}
	return nil
}

func (s *Store) exec(ctx context.Context, sql string, args ...any) error {
	if tx, ok := transaction.TxFromContext(ctx); ok {
		_, err := tx.Exec(ctx, sql, args...)
		return err
	}
	_, err := s.pool.Exec(ctx, sql, args...)
	return err
}

func (s *Store) query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	if tx, ok := transaction.TxFromContext(ctx); ok {
		return tx.Query(ctx, sql, args...)
	}
	return s.pool.Query(ctx, sql, args...)
}

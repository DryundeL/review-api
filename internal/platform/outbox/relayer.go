package outbox

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"review-api/internal/platform/eventbus"
	"review-api/internal/platform/transaction"
)

type PgxRelayer struct {
	pool   *pgxpool.Pool
	store  *Store
	bus    eventbus.Publisher
	log    *slog.Logger
	failOn bool
}

type RelayerOptions struct {
	Logger          *slog.Logger
	MarkFailedOnErr bool
}

func NewRelayer(pool *pgxpool.Pool, bus eventbus.Publisher, opts RelayerOptions) *PgxRelayer {
	log := opts.Logger
	if log == nil {
		log = slog.Default()
	}
	return &PgxRelayer{
		pool:   pool,
		store:  NewStore(pool),
		bus:    bus,
		log:    log,
		failOn: opts.MarkFailedOnErr,
	}
}

func (r *PgxRelayer) RelayPending(ctx context.Context, limit int) (int, error) {
	if r.bus == nil {
		return 0, fmt.Errorf("event bus publisher is nil")
	}

	var published int
	txm := transaction.NewPgxManager(r.pool)
	err := txm.WithinTransaction(ctx, func(txCtx context.Context) error {
		msgs, err := r.store.ListPendingForUpdate(txCtx, limit)
		if err != nil {
			return err
		}
		for _, msg := range msgs {
			if err := r.bus.Publish(txCtx, msg.EventName, []byte(msg.Payload)); err != nil {
				r.log.Error("outbox relay: publish failed",
					"id", msg.ID,
					"event", msg.EventName,
					"error", err,
				)
				if r.failOn {
					if markErr := r.store.MarkFailed(txCtx, msg.ID); markErr != nil {
						return markErr
					}
				}
				continue
			}
			if err := r.store.MarkSent(txCtx, msg.ID, time.Now().UTC()); err != nil {
				return err
			}
			published++
		}
		return nil
	})
	return published, err
}

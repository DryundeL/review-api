package outbox

import "context"

type Writer interface {
	Append(ctx context.Context, msg Message) error
}

type Relayer interface {
	RelayPending(ctx context.Context, limit int) (published int, err error)
}

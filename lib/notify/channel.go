package notify

import "context"

// Channel 出站通知通道。
type Channel interface {
	Name() string
	Send(ctx context.Context, e Event) error
}

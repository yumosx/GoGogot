package channel

import "context"

type ctxKey int

const (
	ctxKeyChannel ctxKey = iota
	ctxKeyChannelID
)

func WithChannel(ctx context.Context, ch Channel, channelID string) context.Context {
	ctx = context.WithValue(ctx, ctxKeyChannel, ch)
	return context.WithValue(ctx, ctxKeyChannelID, channelID)
}

func FromContext(ctx context.Context) (Channel, string, bool) {
	ch, ok1 := ctx.Value(ctxKeyChannel).(Channel)
	id, ok2 := ctx.Value(ctxKeyChannelID).(string)
	return ch, id, ok1 && ok2
}

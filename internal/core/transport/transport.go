package transport

import (
	"context"
	"gogogot/internal/channel"
)

// Transport wraps a channel.Channel and provides a high-level interface
// for the engine, encapsulating UX logic like typing indicators,
// status updates, and optional interface type assertions.
type Transport struct {
	ch channel.Channel
}

func New(ch channel.Channel) *Transport {
	return &Transport{ch: ch}
}

func (t *Transport) Run(ctx context.Context, handler channel.Handler) error {
	return t.ch.Run(ctx, handler)
}

func (t *Transport) SendText(ctx context.Context, channelID, text string) error {
	return t.ch.SendText(ctx, channelID, text)
}

func (t *Transport) Channel() channel.Channel {
	return t.ch
}

func (t *Transport) ChannelName() string {
	return t.ch.Name()
}

func (t *Transport) NotifyTyping(ctx context.Context, channelID string) {
	if tn, ok := t.ch.(channel.TypingNotifier); ok {
		_ = tn.SendTyping(ctx, channelID)
	}
}

func (t *Transport) SendInitialStatus(ctx context.Context, channelID string) string {
	if su, ok := t.ch.(channel.StatusUpdater); ok {
		statusID, _ := su.SendStatus(ctx, channelID, channel.AgentStatus{Phase: channel.PhaseThinking})
		return statusID
	}
	return ""
}

func (t *Transport) UpdateStatus(ctx context.Context, channelID, statusID string, status channel.AgentStatus) {
	if su, ok := t.ch.(channel.StatusUpdater); ok && statusID != "" {
		_ = su.UpdateStatus(ctx, channelID, statusID, status)
	}
}

func (t *Transport) DeleteStatus(ctx context.Context, channelID, statusID string) {
	if su, ok := t.ch.(channel.StatusUpdater); ok && statusID != "" {
		_ = su.DeleteStatus(ctx, channelID, statusID)
	}
}

func (t *Transport) OwnerChannelID() string {
	if or, ok := t.ch.(channel.OwnerResolver); ok {
		return or.OwnerChannelID()
	}
	return ""
}

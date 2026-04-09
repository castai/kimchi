package telemetry

import "context"

type clientKey struct{}

// WithCtx returns a new context with the telemetry client attached.
func WithCtx(ctx context.Context, client Client) context.Context {
	return context.WithValue(ctx, clientKey{}, client)
}

// FromCtx retrieves the telemetry client from the context.
// Returns a no-op client if no client is found.
func FromCtx(ctx context.Context) Client {
	if client, ok := ctx.Value(clientKey{}).(Client); ok {
		return client
	}
	return &noopClient{}
}

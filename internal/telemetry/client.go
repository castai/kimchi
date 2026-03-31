package telemetry

import (
	"fmt"
	"runtime"
	"time"

	"github.com/castai/kimchi/internal/config"
	"github.com/castai/kimchi/internal/version"
	"github.com/denisbrodbeck/machineid"
	"github.com/posthog/posthog-go"
	"k8s.io/klog/v2"
)

const posthogAPIKey = "phc_PLACEHOLDER"
const posthogEndpoint = "https://us.posthog.com"

type Event struct {
	Name    string
	Time    time.Time
	Payload map[string]any
}

// NewEvent creates a new Event with the current time.
func NewEvent(name string, payload map[string]any) Event {
	return Event{
		Name:    name,
		Time:    time.Now(),
		Payload: payload,
	}
}

type Client interface {
	Track(event Event)
	Close()
}

// New creates a telemetry client based on environment and config
func New() Client {
	enabled, err := config.IsTelemetryEnabled()
	if err != nil {
		klog.ErrorS(err, "failed to check telemetry status, assuming disabled")
		enabled = false
	}

	if !enabled {
		klog.InfoS("telemetry disabled")
		return &noopClient{}
	}

	client, err := newPosthogClient(version.Version)
	if err != nil {
		klog.ErrorS(err, "failed to create telemetry client, disabling telemetry")
		return &noopClient{}
	}

	klog.InfoS("telemetry enabled", "endpoint", posthogEndpoint)
	return client
}

// posthogClient is the real PostHog implementation
type posthogClient struct {
	client           posthog.Client
	deviceID         string
	sendFeatureFlags posthog.SendFeatureFlagsOptions
}

func newPosthogClient(cliVersion string) (*posthogClient, error) {
	deviceID, err := machineid.ID()
	if err != nil {
		klog.ErrorS(err, "failed to get machine ID, using empty device ID")
		deviceID = ""
	}

	config := posthog.Config{
		Endpoint: posthogEndpoint,
		DefaultEventProperties: posthog.NewProperties().
			Set("cli_version", cliVersion).
			Set("os", runtime.GOOS).
			Set("arch", runtime.GOARCH),
	}

	client, err := posthog.NewWithConfig(posthogAPIKey, config)
	if err != nil {
		return nil, fmt.Errorf("create posthog client: %w", err)
	}

	opts := posthog.SendFeatureFlagsOptions{
		OnlyEvaluateLocally: true,
	}
	if deviceID != "" {
		opts.DeviceId = &deviceID
	}

	return &posthogClient{
		client:           client,
		deviceID:         deviceID,
		sendFeatureFlags: opts,
	}, nil
}

func (c *posthogClient) Track(evt Event) {
	props := posthog.Properties{}
	for k, v := range evt.Payload {
		props.Set(k, v)
	}

	cap := posthog.Capture{
		Event:            evt.Name,
		Timestamp:        evt.Time,
		Properties:       props,
		DistinctId:       c.deviceID,
		SendFeatureFlags: &c.sendFeatureFlags,
	}
	err := c.client.Enqueue(cap)
	if err != nil {
		klog.ErrorS(err, "failed to capture telemetry event")
	}
}

func (c *posthogClient) Close() {
	if err := c.client.Close(); err != nil {
		klog.ErrorS(err, "failed to close telemetry client")
	}
}

type noopClient struct{}

func (c *noopClient) Track(evt Event) {}
func (c *noopClient) Close()          {}

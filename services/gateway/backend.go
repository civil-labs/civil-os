package main

import (
	"context"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/servicediscovery"
	"github.com/aws/aws-sdk-go-v2/service/servicediscovery/types"
)

// BackendManager handles the list of IPs and round-robin selection
type BackendManager struct {
	client      *servicediscovery.Client
	namespace   string
	serviceName string
	endpoints   []string
	mu          sync.RWMutex
	rrCounter   uint64
}

// NewBackendManager initializes the AWS client
func NewBackendManager(ctx context.Context, namespace, serviceName string) (*BackendManager, error) {
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("unable to load SDK config: %v", err)
	}

	return &BackendManager{
		client:      servicediscovery.NewFromConfig(cfg),
		namespace:   namespace,
		serviceName: serviceName,
		// Init an empty list for pointer safety before initial poll
		endpoints: []string{},
	}, nil
}

// StartPolling updates the endpoint list every 'interval'
func (bm *BackendManager) StartPolling(ctx context.Context, interval time.Duration) {
	// Poll immediately on start
	bm.refreshEndpoints(ctx)

	ticker := time.NewTicker(interval)

	// Creates an anonymous function as a goroutine
	go func() {
		for {
			select {
			// When the process defined by main attempts to shutdown, the read-only channel
			// returned by ctx.Done() will unblock and let this goroutine shutodown
			// Normally, the ticker (also a read-only channel) will return a value first
			// and unblock that path
			case <-ctx.Done():
				return
			case <-ticker.C:
				bm.refreshEndpoints(ctx)
			}
		}
	}()
}

func (bm *BackendManager) refreshEndpoints(ctx context.Context) {
	// Call AWS Cloud Map to get healthy instances
	output, err := bm.client.DiscoverInstances(ctx, &servicediscovery.DiscoverInstancesInput{
		NamespaceName: aws.String(bm.namespace),
		ServiceName:   aws.String(bm.serviceName),
		HealthStatus:  types.HealthStatusFilterHealthy, // Only get healthy instances
		MaxResults:    aws.Int32(100),
	})
	if err != nil {
		log.Printf("Error discovering instances: %v", err)
		return
	}

	var newEndpoints []string
	for _, inst := range output.Instances {
		// Cloud Map stores connection info in Attributes
		ip := inst.Attributes["AWS_INSTANCE_IPV4"]
		port := inst.Attributes["AWS_INSTANCE_PORT"]

		if ip != "" {
			addr := ip
			if port != "" {
				addr = fmt.Sprintf("%s:%s", ip, port)
			}
			newEndpoints = append(newEndpoints, "http://"+addr)
		}
	}

	if len(newEndpoints) > 0 {
		bm.mu.Lock()
		bm.endpoints = newEndpoints
		bm.mu.Unlock()
	}
}

// NextEndpoint returns the next URL in the rotation
func (bm *BackendManager) NextEndpoint() (string, error) {
	bm.mu.RLock()
	defer bm.mu.RUnlock()

	if len(bm.endpoints) == 0 {
		return "", fmt.Errorf("no healthy endpoints available")
	}

	// Atomic increment for thread-safe round robin
	val := atomic.AddUint64(&bm.rrCounter, 1)
	index := val % uint64(len(bm.endpoints))
	return bm.endpoints[index], nil
}

// IsReady returns true if we have at least one healthy backend
func (bm *BackendManager) IsReady() bool {
	bm.mu.RLock()
	defer bm.mu.RUnlock()
	return len(bm.endpoints) > 0
}

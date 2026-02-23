package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
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
		log.Printf("Updated backends: %v", newEndpoints)
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

func main() {
	cfg, err := LoadConfig()
	if err != nil {
		// Log fatal ensures the app exits with a non-zero status code
		log.Fatalf("Configuration Error: %v", err)
	}

	log.Printf("Starting proxy on port %s for Service: %s in Namespace: %s",
		cfg.Port, cfg.TileServerServiceName, cfg.Namespace)

	//
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Replace with your actual Cloud Map details
	tileServers, err := NewBackendManager(ctx, cfg.Namespace, cfg.TileServerServiceName)
	if err != nil {
		log.Fatalf("Failed to init tile service load balancer: %v", err)
	}

	// Poll AWS every 30 seconds
	tileServers.StartPolling(ctx, 30*time.Second)

	// 2. Create the Reverse Proxy with a custom Director
	proxy := &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			// Get next target from our load balancer
			targetStr, err := tileServers.NextEndpoint()
			if err != nil {
				// If no backends, we can't really fail gracefully inside Director
				// best effort is to log. The handler will eventually error out.
				log.Printf("Proxy error: %v", err)
				return
			}

			originalHost := req.Host

			if originalHost == "" {
				originalHost = req.URL.Host // Fallback
			}

			// Parse the target URL (e.g. "http://10.0.1.5:8080")
			// In a real app, you might parse these once and cache them,
			// but parsing here is negligible for most tile loads.
			targetURL, _ := url.Parse(targetStr)

			// Rewrite the request to target the backend
			req.URL.Scheme = targetURL.Scheme
			req.URL.Host = targetURL.Host

			// Important: Update the Host header so the backend accepts it
			req.Host = targetURL.Host

			// 3. TELL THE BACKEND THE TRUTH
			// "The user actually typed 'civillabs.app'"
			req.Header.Set("X-Forwarded-Host", originalHost)

			// "The user is using HTTPS (even if we are talking HTTP right now)"
			req.Header.Set("X-Forwarded-Proto", "https")

			// "This is the user's real IP" (Optional but good for logs)
			req.Header.Set("X-Real-IP", req.RemoteAddr)

			// Note: We do NOT touch req.URL.Path here.
			// It has already been stripped by the middleware below.
		},

		// This is needed to strip off any conflicting header details that the Tile Server attaches
		ModifyResponse: func(r *http.Response) error {

			// The Middleware already set these headers.
			// We MUST delete any versions sent by the backend to avoid the "Multiple Values" error.
			r.Header.Del("Access-Control-Allow-Origin")
			r.Header.Del("Access-Control-Allow-Methods")
			r.Header.Del("Access-Control-Allow-Headers")

			return nil
		},
	}

	allowedClientIDs := []string{"civil-prototype-frontend"}

	auth, err := RequireAuth(cfg.IDPLocalHostName, cfg.IDPLocalPort, allowedClientIDs)

	http.HandleFunc("/health", HealthCheckHandler(tileServers))

	// 3. Setup Middleware and Handler
	// We handle /tiles/, strip the prefix, and pass to proxy
	http.Handle("/tiles/", CORSMiddleware(auth(proxy)))

	log.Printf("Server listening on :%s", cfg.Port)
	if err := http.ListenAndServe(":"+cfg.Port, nil); err != nil {
		log.Fatal(err)
	}
}

func CORSMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		log.Println("CORS middleware activated")

		// 1. ALWAYS set headers (Success, Failure, or Preflight)
		// This guarantees the browser never sees a "Missing Header" error.
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		// 2. Handle Preflight
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		// 3. Pass to next handler
		next.ServeHTTP(w, r)
	})
}

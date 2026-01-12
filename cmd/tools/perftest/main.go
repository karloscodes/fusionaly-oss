// main.go - Performance testing tool for Fusionaly
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"text/tabwriter"
	"time"

	"log/slog"

	v1 "fusionaly/api/v1"
	"fusionaly/internal/events"
)

// Global counter for debug messages
var debugCounter int64

// PerfConfig holds the configuration for the performance test
type PerfConfig struct {
	BaseURL       string
	Origin        string // Origin header for validation (matches registered website domain)
	Concurrency   int
	Duration      time.Duration
	EventsPerSec  int
	VerboseOutput bool
	Timeout       time.Duration
}

// PerfStats holds statistics about the performance test
type PerfStats struct {
	TotalRequests      int64
	SuccessfulRequests int64
	FailedRequests     int64
	TotalDuration      time.Duration
	MinLatency         time.Duration
	MaxLatency         time.Duration
	TotalLatency       time.Duration
	StatusCodes        map[int]int64
	StatusCodesMutex   sync.Mutex
	StartTime          time.Time
	EndTime            time.Time
	DatabaseBusyErrors int64
	// For tracking response time distribution
	ResponseTimes      []time.Duration
	ResponseTimesMutex sync.Mutex
	// Track requests over time for time-series analysis
	RequestsOverTime map[string]int64 // Map of time bucket (minute) to count
	RequestTimeMutex sync.Mutex
}

// Result captures the result of a single request
type Result struct {
	Duration   time.Duration
	StatusCode int
	Error      error
	Timestamp  time.Time // When the request was made
}

func main() {
	// Configure command line flags
	baseURL := flag.String("url", "http://localhost:3000", "Base URL of the API")
	concurrency := flag.Int("c", 10, "Number of concurrent clients")
	duration := flag.Duration("d", 30*time.Second, "Duration of the test")
	eventsPerSec := flag.Int("rate", 0, "Target events per second (0 = unlimited)")
	verbose := flag.Bool("verbose", false, "Enable verbose output")
	timeout := flag.Duration("timeout", 10*time.Second, "Request timeout")
	flag.Parse()

	// Initialize logger
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))

	// Get origin from environment or use default (must match a registered website)
	origin := os.Getenv("FUSIONALY_ORIGIN")
	if origin == "" {
		origin = "https://example.com" // Default - must be registered in the database
		logger.Info("Using default origin (https://example.com)")
	} else {
		logger.Info("Using origin from FUSIONALY_ORIGIN environment variable", slog.String("origin", origin))
	}

	// Initialize test configuration
	config := &PerfConfig{
		BaseURL:       *baseURL,
		Origin:        origin,
		Concurrency:   *concurrency,
		Duration:      *duration,
		EventsPerSec:  *eventsPerSec,
		VerboseOutput: *verbose,
		Timeout:       *timeout,
	}

	// Set up signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Create context for the test duration
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start a goroutine to handle termination signals
	go func() {
		sig := <-sigChan
		fmt.Printf("Received signal %v, shutting down...\n", sig)
		cancel()
	}()

	// Display help information
	fmt.Println("\n=== Fusionaly Performance Testing Tool ===")
	fmt.Println("Parameters explained:")
	fmt.Printf("  URL (-url):           %s - The base URL of the API to test\n", config.BaseURL)
	fmt.Printf("  Concurrency (-c):     %d - Number of concurrent clients sending requests\n", config.Concurrency)
	fmt.Printf("  Duration (-d):        %v - How long the test will run\n", config.Duration)
	fmt.Printf("  Events/sec (-rate):   %d - Target events per second (0 = unlimited requests)\n", config.EventsPerSec)
	fmt.Printf("  Timeout (-timeout):   %v - Maximum time to wait for each request\n", config.Timeout)
	fmt.Printf("  Verbose (-verbose):   %v - Whether to show detailed output\n", config.VerboseOutput)
	fmt.Println("================================================")

	// Initialize stats
	stats := &PerfStats{
		StatusCodes:      make(map[int]int64),
		StatusCodesMutex: sync.Mutex{},
		StartTime:        time.Now(),
	}

	// Run the test
	fmt.Printf("Starting performance test with %d concurrent clients for %v\n", config.Concurrency, config.Duration)
	fmt.Printf("Target URL: %s/x/api/v1/events\n", config.BaseURL)
	if config.EventsPerSec > 0 {
		fmt.Printf("Target rate: %d events/second\n", config.EventsPerSec)
	} else {
		fmt.Println("Target rate: unlimited")
	}

	// Create a context with timeout for the test duration
	testCtx, testCancel := context.WithTimeout(ctx, config.Duration)
	defer testCancel()

	// Run the test
	resultChan := runTest(testCtx, config, logger)

	// Process results
	for result := range resultChan {
		processResult(result, stats)
	}

	// Calculate final statistics
	stats.EndTime = time.Now()
	stats.TotalDuration = stats.EndTime.Sub(stats.StartTime)

	// Print results
	printResults(stats)

	// Cleanup
	fmt.Println("Test completed successfully!")
}

// runTest starts the performance test and returns a channel for results
func runTest(ctx context.Context, config *PerfConfig, logger *slog.Logger) <-chan Result {
	resultChan := make(chan Result, config.Concurrency*10)
	var wg sync.WaitGroup

	// Calculate requests per worker based on rate limit
	requestsPerSecPerWorker := 0.0
	if config.EventsPerSec > 0 {
		requestsPerSecPerWorker = float64(config.EventsPerSec) / float64(config.Concurrency)
		logger.Info("Rate limiting enabled",
			slog.Int("totalRequestsPerSec", config.EventsPerSec),
			slog.Float64("requestsPerSecPerWorker", requestsPerSecPerWorker))
	} else {
		logger.Info("No rate limiting, running at maximum speed")
	}

	// Start worker goroutines
	for i := 0; i < config.Concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			client := &http.Client{
				Timeout: config.Timeout,
			}

			// Create a ticker for this worker if rate limiting is enabled
			var ticker *time.Ticker
			if requestsPerSecPerWorker > 0 {
				interval := time.Duration(float64(time.Second) / requestsPerSecPerWorker)
				ticker = time.NewTicker(interval)
				defer ticker.Stop()
			}

			for {
				select {
				case <-ctx.Done():
					// Context canceled or timed out
					return
				default:
					// Rate limiting if enabled
					if ticker != nil {
						select {
						case <-ticker.C:
							// Time to send next request
						case <-ctx.Done():
							return
						}
					}

					// Send a request
					result := sendRequest(client, config, workerID)
					resultChan <- result

					// Add a small cooldown period to reduce contention
					// This is dynamic based on concurrency: higher concurrency = longer cooldown
					cooldownMs := 2 + (config.Concurrency / 20)
					time.Sleep(time.Duration(cooldownMs) * time.Millisecond)
				}
			}
		}(i)
	}

	// Close the result channel when all workers are done
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	return resultChan
}

// sendRequest sends a single request to the API
func sendRequest(client *http.Client, config *PerfConfig, workerID int) Result {
	// Prepare the request payload
	eventData := generateEventData(config, workerID)
	jsonData, err := json.Marshal(eventData)
	if err != nil {
		return Result{Error: fmt.Errorf("failed to marshal JSON: %w", err)}
	}

	// Log event data for debugging (only for the first few events)
	if workerID < 3 {
		fmt.Printf("Sending event: URL=%s, EventType=%v, Timestamp=%v (UTC, type: %T)\n",
			eventData.URL,
			eventData.EventType,
			eventData.Timestamp,
			eventData.Timestamp)

		// Print the raw JSON to debug timestamp format
		fmt.Printf("Raw JSON payload: %s\n", string(jsonData))
	}

	// Create the request
	req, err := http.NewRequest("POST", config.BaseURL+"/x/api/v1/events", bytes.NewBuffer(jsonData))
	if err != nil {
		return Result{Error: fmt.Errorf("failed to create request: %w", err)}
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", generateUserAgent())
	req.Header.Set("Origin", config.Origin)
	if eventData.Referrer != "" {
		req.Header.Set("Referer", eventData.Referrer)
	}

	// Measure time and send the request
	startTime := time.Now()
	resp, err := client.Do(req)
	duration := time.Since(startTime)

	if err != nil {
		return Result{Duration: duration, Error: fmt.Errorf("request failed: %w", err), Timestamp: startTime}
	}
	defer resp.Body.Close()

	// Read the response body for debugging if there's an error status
	if resp.StatusCode >= 400 {
		var bodyBytes []byte
		bodyBytes, _ = io.ReadAll(resp.Body)
		fmt.Printf("Error response [%d]: %s\n", resp.StatusCode, string(bodyBytes))
	} else {
		// Ensure we read the body even for successful responses to properly close the connection
		_, _ = io.Copy(io.Discard, resp.Body)
	}

	// Debug log for every 1000th request to confirm response times
	if workerID == 0 && atomic.LoadInt64(&debugCounter)%1000 == 0 {
		fmt.Printf("DEBUG: Request %d completed with status %d in %v\n",
			atomic.LoadInt64(&debugCounter), resp.StatusCode, duration)
	}
	atomic.AddInt64(&debugCounter, 1)

	return Result{
		Duration:   duration,
		StatusCode: resp.StatusCode,
		Error:      nil,
		Timestamp:  startTime,
	}
}

// generateEventData creates random event data for testing
func generateEventData(config *PerfConfig, workerID int) v1.CreateEventParams {
	randSource := rand.NewSource(time.Now().UnixNano() + int64(workerID))
	randGen := rand.New(randSource)

	// Generate a random URL path
	paths := []string{
		"/",
		"/products",
		"/services",
		"/about",
		"/contact",
		"/blog",
		"/pricing",
		"/faq",
		"/login",
		"/register",
	}
	path := paths[randGen.Intn(len(paths))]
	url := fmt.Sprintf("https://example.com%s", path)

	// Generate a random referrer (sometimes empty for direct traffic)
	var referrer string
	if randGen.Float64() < 0.7 { // 70% chance of having a referrer
		referrers := []string{
			"https://google.com",
			"https://facebook.com",
			"https://twitter.com",
			"https://linkedin.com",
			"https://bing.com",
			"https://instagram.com",
			"https://youtube.com",
		}
		referrer = referrers[randGen.Intn(len(referrers))]
	}

	// Generate a timestamp in the past (within the last 12 hours)
	// This ensures events fall within the dashboard's query range
	now := time.Now().UTC()
	hoursAgo := randGen.Intn(12) + 1 // Random time between 1-12 hours ago
	minutesAgo := randGen.Intn(60)
	secondsAgo := randGen.Intn(60)
	pastTime := now.Add(-time.Duration(hoursAgo)*time.Hour -
		time.Duration(minutesAgo)*time.Minute -
		time.Duration(secondsAgo)*time.Second)

	return v1.CreateEventParams{
		URL:       url,
		Referrer:  referrer,
		Timestamp: pastTime,
		EventType: events.EventTypePageView,
		UserID:    fmt.Sprintf("test-user-%d-%d", workerID, randGen.Intn(1000)),
		UserAgent: generateUserAgent(),
		EventKey:  "", // Empty for page views
		EventMetadata: map[string]interface{}{
			"testMode": true,
			"workerId": workerID,
		},
	}
}

// generateUserAgent returns a random user agent string
func generateUserAgent() string {
	userAgents := []string{
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/92.0.4515.159 Safari/537.36",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/14.1.1 Safari/605.1.15",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/92.0.4515.131 Safari/537.36",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:90.0) Gecko/20100101 Firefox/90.0",
		"Mozilla/5.0 (iPhone; CPU iPhone OS 14_6 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/14.0 Mobile/15E148 Safari/604.1",
		"Mozilla/5.0 (iPad; CPU OS 14_6 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/14.0 Mobile/15E148 Safari/604.1",
		"Mozilla/5.0 (Linux; Android 11; SM-G998B) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.120 Mobile Safari/537.36",
	}
	return userAgents[rand.Intn(len(userAgents))]
}

// processResult processes the results of a single request
func processResult(result Result, stats *PerfStats) {
	atomic.AddInt64(&stats.TotalRequests, 1)

	if result.Error != nil {
		atomic.AddInt64(&stats.FailedRequests, 1)
		return
	}

	// Store response time for percentiles
	stats.ResponseTimesMutex.Lock()
	stats.ResponseTimes = append(stats.ResponseTimes, result.Duration)
	stats.ResponseTimesMutex.Unlock()

	// Track requests over time (bucket by minute for time series)
	timeBucket := result.Timestamp.Format("15:04") // Hour:Minute
	stats.RequestTimeMutex.Lock()
	if stats.RequestsOverTime == nil {
		stats.RequestsOverTime = make(map[string]int64)
	}
	stats.RequestsOverTime[timeBucket]++
	stats.RequestTimeMutex.Unlock()

	// Track status code distribution
	stats.StatusCodesMutex.Lock()
	stats.StatusCodes[result.StatusCode]++
	stats.StatusCodesMutex.Unlock()

	// Consider only 200 OK and 202 Accepted as successful responses
	if result.StatusCode == http.StatusOK || result.StatusCode == http.StatusAccepted {
		atomic.AddInt64(&stats.SuccessfulRequests, 1)
	} else {
		atomic.AddInt64(&stats.FailedRequests, 1)
		// Track database locking errors specifically (503 Service Unavailable with DATABASE_BUSY code)
		if result.StatusCode == http.StatusServiceUnavailable {
			atomic.AddInt64(&stats.DatabaseBusyErrors, 1)
		}
	}

	atomic.AddInt64((*int64)(&stats.TotalLatency), int64(result.Duration))

	// Update min latency (if it's the first request or smaller than current min)
	if stats.MinLatency == 0 || result.Duration < stats.MinLatency {
		stats.MinLatency = result.Duration
	}

	// Update max latency
	if result.Duration > stats.MaxLatency {
		stats.MaxLatency = result.Duration
	}
}

// printResults displays the test results in a nicely formatted table
func printResults(stats *PerfStats) {
	fmt.Println("\nPerformance Test Results:")
	fmt.Printf("Test Duration: %v\n", stats.TotalDuration.Round(time.Millisecond))
	fmt.Printf("Start Time: %v\n", stats.StartTime.Format(time.RFC3339))
	fmt.Printf("End Time: %v\n", stats.EndTime.Format(time.RFC3339))

	// Calculate requests per second
	requestsPerSecond := float64(stats.TotalRequests) / stats.TotalDuration.Seconds()
	fmt.Printf("Requests Per Second: %.2f\n", requestsPerSecond)

	// Calculate average latency
	var avgLatency time.Duration
	if stats.TotalRequests > 0 {
		avgLatency = time.Duration(int64(stats.TotalLatency) / stats.TotalRequests)
	}

	// Create a tabwriter for aligned output
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "\n%s\t%s\n", "METRIC", "VALUE")
	fmt.Fprintf(w, "%s\t%s\n", "------", "-----")
	fmt.Fprintf(w, "Total Requests\t%d\n", stats.TotalRequests)
	fmt.Fprintf(w, "Successful Requests\t%d (%.2f%%)\n", stats.SuccessfulRequests, 100*float64(stats.SuccessfulRequests)/float64(stats.TotalRequests))
	fmt.Fprintf(w, "Failed Requests\t%d (%.2f%%)\n", stats.FailedRequests, 100*float64(stats.FailedRequests)/float64(stats.TotalRequests))

	// Add database busy errors information if any occurred
	if stats.DatabaseBusyErrors > 0 {
		fmt.Fprintf(w, "Database Busy Errors\t%d (%.2f%%)\n", stats.DatabaseBusyErrors, 100*float64(stats.DatabaseBusyErrors)/float64(stats.TotalRequests))
	}

	fmt.Fprintf(w, "Min Latency\t%v\n", stats.MinLatency)
	fmt.Fprintf(w, "Max Latency\t%v\n", stats.MaxLatency)
	fmt.Fprintf(w, "Avg Latency\t%v\n", avgLatency)
	w.Flush()

	// Create a table for status codes
	if len(stats.StatusCodes) > 0 {
		fmt.Println("\nStatus Code Distribution:")
		w = tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", "STATUS CODE", "COUNT", "PERCENTAGE", "GRAPH")
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", "-----------", "-----", "----------", "-----")

		// Sort the status codes for consistent output
		var codes []int
		for code := range stats.StatusCodes {
			codes = append(codes, code)
		}
		sort.Ints(codes)

		// Find the maximum count for scaling the graph
		var maxCount int64 = 1 // Avoid division by zero
		for _, count := range stats.StatusCodes {
			if count > maxCount {
				maxCount = count
			}
		}

		// Generate graph scale - maximum of 50 characters wide
		const maxBarLength = 50

		for _, code := range codes {
			count := stats.StatusCodes[code]
			percentage := 100 * float64(count) / float64(stats.TotalRequests)

			// Generate bar graph
			barLength := int(float64(count) / float64(maxCount) * maxBarLength)
			bar := strings.Repeat("█", barLength)

			fmt.Fprintf(w, "%d\t%d\t%.2f%%\t%s\n", code, count, percentage, bar)
		}
		w.Flush()
	}

	// Create response time histogram
	if len(stats.ResponseTimes) > 0 {
		printResponseTimeHistogram(stats)
	}

	// Print requests over time (time series)
	if len(stats.RequestsOverTime) > 0 {
		printRequestsOverTime(stats)
	}

	// Print summary dashboard at the end
	printSummaryDashboard(stats, requestsPerSecond, avgLatency)

	// Export results to JSON
	exportResults(stats, requestsPerSecond, avgLatency)
}

// printResponseTimeHistogram generates and prints a histogram of response times
func printResponseTimeHistogram(stats *PerfStats) {
	fmt.Println("\nResponse Time Distribution (ms):")

	// Sort response times
	sort.Slice(stats.ResponseTimes, func(i, j int) bool {
		return stats.ResponseTimes[i] < stats.ResponseTimes[j]
	})

	// Determine histogram bins
	// We'll use 10 bins from min to max response time
	min := stats.MinLatency.Milliseconds()
	max := stats.MaxLatency.Milliseconds()

	// If min and max are the same, create a small range around it
	if min == max {
		min = max - 1
		max = max + 1
	}

	binSize := (max - min) / 10
	if binSize < 1 {
		binSize = 1 // Ensure minimum bin size of 1ms
	}

	// Create bins
	bins := make(map[int64]int)
	for _, duration := range stats.ResponseTimes {
		ms := duration.Milliseconds()
		binIndex := (ms - min) / binSize
		if binIndex >= 10 {
			binIndex = 9 // Ensure we stay within bounds
		}
		binStart := min + (binIndex * binSize)
		bins[binStart]++
	}

	// Sort bin keys for ordered output
	var binKeys []int64
	for key := range bins {
		binKeys = append(binKeys, key)
	}
	sort.Slice(binKeys, func(i, j int) bool {
		return binKeys[i] < binKeys[j]
	})

	// Find the maximum bin count for scaling
	maxCount := 1
	for _, count := range bins {
		if count > maxCount {
			maxCount = count
		}
	}

	// Create a tabwriter for aligned output
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", "RANGE (ms)", "COUNT", "PERCENTAGE", "GRAPH")
	fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", "---------", "-----", "----------", "-----")

	const maxBarLength = 50
	totalResponses := len(stats.ResponseTimes)

	// Print each bin with improved bar graphs
	for _, binStart := range binKeys {
		count := bins[binStart]
		binEnd := binStart + binSize
		percentage := 100.0 * float64(count) / float64(totalResponses)

		// Generate bar with gradient color representation using block characters
		barLength := int(float64(count) / float64(maxCount) * maxBarLength)
		bar := strings.Repeat("█", barLength)

		fmt.Fprintf(w, "%d-%d\t%d\t%.2f%%\t%s\n", binStart, binEnd, count, percentage, bar)
	}

	// Add percentile information
	p50Index := int(float64(totalResponses) * 0.5)
	p90Index := int(float64(totalResponses) * 0.9)
	p95Index := int(float64(totalResponses) * 0.95)
	p99Index := int(float64(totalResponses) * 0.99)

	fmt.Fprintf(w, "\n%s\t%s\n", "PERCENTILE", "VALUE (ms)")
	fmt.Fprintf(w, "%s\t%s\n", "----------", "----------")
	fmt.Fprintf(w, "50th (Median)\t%d\n", stats.ResponseTimes[p50Index].Milliseconds())
	fmt.Fprintf(w, "90th\t%d\n", stats.ResponseTimes[p90Index].Milliseconds())
	fmt.Fprintf(w, "95th\t%d\n", stats.ResponseTimes[p95Index].Milliseconds())
	fmt.Fprintf(w, "99th\t%d\n", stats.ResponseTimes[p99Index].Milliseconds())

	w.Flush()
}

// printRequestsOverTime displays a time series of requests per minute
func printRequestsOverTime(stats *PerfStats) {
	fmt.Println("\nRequests Over Time (by minute):")

	// Get all time buckets (minutes)
	var timeBuckets []string
	for bucket := range stats.RequestsOverTime {
		timeBuckets = append(timeBuckets, bucket)
	}

	// Sort chronologically
	sort.Strings(timeBuckets)

	// Find max for scaling
	var maxRequests int64 = 1
	for _, count := range stats.RequestsOverTime {
		if count > maxRequests {
			maxRequests = count
		}
	}

	// Print header
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "%s\t%s\t%s\n", "TIME", "REQUESTS", "GRAPH")
	fmt.Fprintf(w, "%s\t%s\t%s\n", "----", "--------", "-----")

	// Print time series with bars
	const maxBarLength = 50
	for _, bucket := range timeBuckets {
		count := stats.RequestsOverTime[bucket]
		barLength := int(float64(count) / float64(maxRequests) * maxBarLength)
		bar := strings.Repeat("█", barLength)
		fmt.Fprintf(w, "%s\t%d\t%s\n", bucket, count, bar)
	}
	w.Flush()
}

// printSummaryDashboard displays a final summary with ASCII art dashboard
func printSummaryDashboard(stats *PerfStats, rps float64, avgLatency time.Duration) {
	fmt.Println("\n╔════════════════════════════════════════════════════════════╗")
	fmt.Println("║                TEST SUMMARY DASHBOARD                       ║")
	fmt.Println("╠════════════════════════════════════════════════════════════╣")

	// Success rate meter
	successRate := 100 * float64(stats.SuccessfulRequests) / float64(stats.TotalRequests)
	successMeter := createProgressBar(int(successRate), 50)
	fmt.Printf("║ Success Rate: [%s] %.1f%%\n", successMeter, successRate)

	// Throughput indicator
	rpsMeter := createScaledBar(rps, 500, 50) // Scale up to 500 RPS
	fmt.Printf("║ Throughput:   [%s] %.1f RPS\n", rpsMeter, rps)

	// Latency indicator (smaller is better, so invert the scale)
	maxGoodLatency := 100.0 // ms
	scaledLatency := 100 - math.Min(100, float64(avgLatency.Milliseconds())*100/maxGoodLatency)
	latencyMeter := createProgressBar(int(scaledLatency), 50)
	fmt.Printf("║ Avg Latency:  [%s] %v\n", latencyMeter, avgLatency)

	// Error rate indicator
	errorRate := 100 * float64(stats.FailedRequests) / float64(stats.TotalRequests)
	// Invert for error (lower is better)
	errorMeter := createProgressBar(100-int(errorRate), 50)
	fmt.Printf("║ Error Rate:   [%s] %.1f%%\n", errorMeter, errorRate)

	// Response time variability (using p95/p50 ratio as a proxy)
	totalResponses := len(stats.ResponseTimes)
	if totalResponses > 0 {
		sort.Slice(stats.ResponseTimes, func(i, j int) bool {
			return stats.ResponseTimes[i] < stats.ResponseTimes[j]
		})
		p50 := stats.ResponseTimes[int(float64(totalResponses)*0.5)].Milliseconds()
		p95 := stats.ResponseTimes[int(float64(totalResponses)*0.95)].Milliseconds()
		if p50 > 0 {
			variability := float64(p95) / float64(p50)
			// Lower variability is better (close to 1.0 is ideal)
			variabilityScore := math.Max(0, 100-((variability-1)*50))
			variabilityMeter := createProgressBar(int(variabilityScore), 50)
			fmt.Printf("║ Consistency:  [%s] p95/p50=%.1fx\n", variabilityMeter, variability)
		}
	}

	// Overall performance score (simple average of success rate and latency score)
	perfScore := (successRate + scaledLatency) / 2
	scoreMeter := createProgressBar(int(perfScore), 50)

	// Rating based on score
	var rating string
	if perfScore >= 90 {
		rating = "Excellent"
	} else if perfScore >= 75 {
		rating = "Good"
	} else if perfScore >= 60 {
		rating = "Fair"
	} else if perfScore >= 40 {
		rating = "Poor"
	} else {
		rating = "Critical"
	}

	fmt.Printf("║ Perf Score:   [%s] %.1f/100 (%s)\n", scoreMeter, perfScore, rating)
	fmt.Println("╚════════════════════════════════════════════════════════════╝")

	// Results saving info
	fmt.Println("\nDetailed results saved to 'perf_results.json'")
}

// createProgressBar generates an ASCII progress bar
func createProgressBar(value int, maxLength int) string {
	if value < 0 {
		value = 0
	} else if value > 100 {
		value = 100
	}

	filledLength := value * maxLength / 100
	emptyLength := maxLength - filledLength

	return strings.Repeat("█", filledLength) + strings.Repeat("░", emptyLength)
}

// createScaledBar generates a bar scaled to a maximum value
func createScaledBar(value float64, maxValue float64, maxLength int) string {
	scaledValue := int(math.Min(100, value*100/maxValue))
	return createProgressBar(scaledValue, maxLength)
}

// exportResults saves test results to a JSON file for external visualization
func exportResults(stats *PerfStats, rps float64, avgLatency time.Duration) {
	// Calculate percentiles
	var p50, p90, p95, p99 int64
	totalResponses := len(stats.ResponseTimes)
	if totalResponses > 0 {
		sort.Slice(stats.ResponseTimes, func(i, j int) bool {
			return stats.ResponseTimes[i] < stats.ResponseTimes[j]
		})

		p50 = stats.ResponseTimes[int(float64(totalResponses)*0.5)].Milliseconds()
		p90 = stats.ResponseTimes[int(float64(totalResponses)*0.9)].Milliseconds()
		p95 = stats.ResponseTimes[int(float64(totalResponses)*0.95)].Milliseconds()
		p99 = stats.ResponseTimes[int(float64(totalResponses)*0.99)].Milliseconds()
	}

	// Create result object
	result := map[string]interface{}{
		"summary": map[string]interface{}{
			"totalRequests":      stats.TotalRequests,
			"successfulRequests": stats.SuccessfulRequests,
			"failedRequests":     stats.FailedRequests,
			"successRate":        100 * float64(stats.SuccessfulRequests) / float64(stats.TotalRequests),
			"requestsPerSecond":  rps,
			"totalDurationMs":    stats.TotalDuration.Milliseconds(),
			"avgLatencyMs":       avgLatency.Milliseconds(),
			"minLatencyMs":       stats.MinLatency.Milliseconds(),
			"maxLatencyMs":       stats.MaxLatency.Milliseconds(),
			"p50LatencyMs":       p50,
			"p90LatencyMs":       p90,
			"p95LatencyMs":       p95,
			"p99LatencyMs":       p99,
			"startTime":          stats.StartTime.Format(time.RFC3339),
			"endTime":            stats.EndTime.Format(time.RFC3339),
		},
		"statusCodes": stats.StatusCodes,
		"timeSeries":  stats.RequestsOverTime,
	}

	// Convert to JSON
	jsonData, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		fmt.Printf("Error creating JSON output: %v\n", err)
		return
	}

	// Write to file
	err = os.WriteFile("perf_results.json", jsonData, 0o644)
	if err != nil {
		fmt.Printf("Error writing results to file: %v\n", err)
	}
}

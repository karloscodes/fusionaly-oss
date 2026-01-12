package jobs

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"fusionaly/internal/config"
	"fusionaly/internal/database"
)

// Scheduler is responsible for running background jobs
type Scheduler struct {
	dbManager *database.DBManager
	logger    *slog.Logger
	ctx       context.Context
	cancel    context.CancelFunc
	enabled   bool
	isRunning bool
	cfg       *config.Config

	// Mutex to prevent concurrent job executions
	processingMutex sync.Mutex
	isProcessing    bool

	// Job instances
	eventProcessor *EventProcessorJob
	cleanupJob     *CleanupJob

	// Tickers for each job type
	eventTicker   *time.Ticker
	cleanupTicker *time.Ticker
}

func NewScheduler(dbManager *database.DBManager, logger *slog.Logger) (*Scheduler, error) {
	ctx, cancel := context.WithCancel(context.Background())
	cfg := config.GetConfig()

	s := &Scheduler{
		dbManager: dbManager,
		logger:    logger,
		ctx:       ctx,
		cancel:    cancel,
		enabled:   true,
		isRunning: false,
		cfg:       cfg,
	}

	// Initialize job instances
	s.eventProcessor = NewEventProcessorJob(dbManager, logger)
	s.cleanupJob = NewCleanupJob(dbManager, logger, cfg)

	return s, nil
}

// executeJobSafely runs a job only if no other job is currently executing
func (s *Scheduler) executeJobSafely(jobName string, jobFunc func() error) {
	s.processingMutex.Lock()
	if s.isProcessing {
		s.logger.Debug("Skipping job execution - previous job still running", slog.String("job", jobName))
		s.processingMutex.Unlock()
		return
	}
	s.isProcessing = true
	s.processingMutex.Unlock()

	defer func() {
		if r := recover(); r != nil {
			s.logger.Error("Panic recovered in background job",
				slog.String("job", jobName),
				slog.Any("panic", r))
		}

		s.processingMutex.Lock()
		s.isProcessing = false
		s.processingMutex.Unlock()
	}()

	if err := jobFunc(); err != nil {
		s.logger.Error("Error executing job", slog.String("job", jobName), slog.Any("error", err))
	}
}

// Start begins all background jobs
func (s *Scheduler) Start() error {
	if !s.enabled {
		s.logger.Info("Background jobs are disabled.")
		return nil
	}

	if s.isRunning {
		s.logger.Info("Background jobs already running.")
		return nil
	}

	s.logger.Info("Starting background jobs...")

	s.isRunning = true

	// Start event processing job
	s.startEventProcessingJob()

	// Start cleanup job
	s.startCleanupJob()

	s.logger.Info("Background jobs started",
		slog.Bool("enabled", s.enabled),
		slog.Bool("isRunning", s.isRunning))

	return nil
}

func (s *Scheduler) startEventProcessingJob() {
	interval := time.Duration(s.cfg.JobIntervalSeconds) * time.Second
	s.logger.Info("Starting event processing job", slog.Duration("interval", interval))
	s.eventTicker = time.NewTicker(interval)

	go func() {
		// Run initial execution
		s.logger.Info("Running initial event processing...")
		s.executeJobSafely("event_processor", s.eventProcessor.Run)

		for {
			select {
			case <-s.eventTicker.C:
				s.executeJobSafely("event_processor", s.eventProcessor.Run)
			case <-s.ctx.Done():
				s.logger.Info("Event processing job stopped")
				return
			}
		}
	}()
}

func (s *Scheduler) startCleanupJob() {
	interval := 24 * time.Hour
	s.logger.Info("Starting cleanup job", slog.Duration("interval", interval))
	s.cleanupTicker = time.NewTicker(interval)

	go func() {
		// Run initial cleanup
		s.logger.Info("Running initial cleanup...")
		if err := s.cleanupJob.Run(); err != nil {
			s.logger.Error("Error in initial cleanup job", slog.Any("error", err))
		}

		for {
			select {
			case <-s.cleanupTicker.C:
				if err := s.cleanupJob.Run(); err != nil {
					s.logger.Error("Error in cleanup job", slog.Any("error", err))
				}
			case <-s.ctx.Done():
				s.logger.Info("Cleanup job stopped")
				return
			}
		}
	}()
}

// Stop halts all background jobs.
// Implements cartridge.BackgroundWorker interface.
func (s *Scheduler) Stop() {
	s.logger.Info("Stopping background jobs...")
	s.enabled = false

	if s.eventTicker != nil {
		s.eventTicker.Stop()
	}
	if s.cleanupTicker != nil {
		s.cleanupTicker.Stop()
	}

	s.cancel()
	s.isRunning = false
	s.logger.Info("Background jobs stopped")
}

// IsRunning returns whether jobs are currently running
func (s *Scheduler) IsRunning() bool {
	return s.isRunning
}

// ProcessEvents allows manual triggering of event processing (for backwards compatibility)
func (s *Scheduler) ProcessEvents() error {
	if !s.enabled {
		return nil
	}
	return s.eventProcessor.Run()
}

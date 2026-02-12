// main.go - HTTP server application
package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"fusionaly/internal"
	"fusionaly/web"
)

const (
	defaultShutdownTimeout = 30 * time.Second
)

func main() {
	// Initialize application with embedded assets
	app, err := internal.NewApp(internal.WithStaticFS(web.Assets()))
	if err != nil {
		log.Fatalf("Failed to create app: %v", err)
	}

	// Run database migrations
	log.Println("Running database migrations...")
	if err := app.DBManager.MigrateDatabase(); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}
	log.Println("Migrations completed")

	// Start the application
	log.Println("Starting application...")
	if err := app.StartAsync(); err != nil {
		log.Fatalf("Failed to start application: %v", err)
	}
	log.Println("Application started successfully")

	// Wait for termination signal
	waitForShutdownSignal(app)
}

// waitForShutdownSignal sets up signal handling and performs graceful shutdown
func waitForShutdownSignal(app *internal.Application) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)

	sig := <-sigChan
	log.Printf("Received signal: %v", sig)

	ctx, cancel := context.WithTimeout(context.Background(), defaultShutdownTimeout)
	defer cancel()

	log.Println("Initiating graceful shutdown...")
	if err := app.Shutdown(ctx); err != nil {
		log.Printf("Error during shutdown: %v", err)
		os.Exit(1)
	}
	log.Println("Server shutdown complete")
}

// main.go - Admin control tool for Fusionaly
package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"gorm.io/gorm"

	"fusionaly/internal"
	"fusionaly/internal/seeder"
	"fusionaly/internal/users"

	"log/slog"
)

const (
	defaultShutdownTimeout = 30 * time.Second
)

// Command defines the interface for all command implementations
type Command interface {
	// Name returns the command name
	Name() string
	// Description returns the command description
	Description() string
	// Execute runs the command with the given app and args
	Execute(ctx context.Context, app *internal.Application, args []string) error
}

// The set of available commands
var commands = []Command{
	&CreateAdminUserCommand{},
	&ChangeAdminPasswordCommand{},
	&MigrateCommand{},
	&SeedCommand{},
	&StatusCommand{},
	&HelpCommand{},
}

func main() {
	// Parse global flags
	flag.Parse()

	// Set up signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)

	// Set up context with cancellation for cleanup
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle shutdown signals in a separate goroutine
	go func() {
		sig := <-sigChan
		log.Printf("Received signal: %v, initiating cleanup...", sig)
		cancel() // Signal the context to cancel
	}()

	// Parse command and arguments
	cmdName, args := parseArgs()

	// Find the requested command
	cmd := findCommand(cmdName)
	if cmd == nil {
		showUsageAndExit()
	}

	// Try to initialize the app
	app, err := internal.NewApp()
	if err != nil {
		log.Printf("Warning: Failed to initialize app: %v", err)
		log.Println("Proceeding with limited functionality...")
		// Let the command handle this situation
	}

	// Ensure app is cleaned up
	defer func() {
		if app != nil {
			shutdownCtx, cancel := context.WithTimeout(context.Background(), defaultShutdownTimeout)
			defer cancel()
			if err := app.Shutdown(shutdownCtx); err != nil {
				log.Printf("Warning: Cleanup error: %v", err)
			}
		}
	}()

	// Execute the command
	if err := cmd.Execute(ctx, app, args); err != nil {
		log.Fatalf("Command failed: %v", err)
	}

	log.Printf("Command %s completed successfully", cmd.Name())
}

// CreateAdminUserCommand implements the command to create an initial admin user
type CreateAdminUserCommand struct{}

// Name returns the command name
func (c *CreateAdminUserCommand) Name() string {
	return "create-admin-user"
}

// Description returns the command description
func (c *CreateAdminUserCommand) Description() string {
	return "Creates an initial admin user"
}

// Execute implements the create-admin-user command
func (c *CreateAdminUserCommand) Execute(ctx context.Context, app *internal.Application, args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: %s <email> <password>", c.Name())
	}

	email := args[0]
	password := args[1]

	log.Printf("Setting up initial user with email: %s", email)

	// Get database connection
	var db *gorm.DB
	if app != nil {
		db = app.DBManager.GetConnection()
	} else {
		// Fallback to direct connection if app initialization failed
		return fmt.Errorf("app initialization failed, direct connection not implemented in this example")
	}

	// Ensure app is cleaned up
	defer func() {
		if app != nil {
			shutdownCtx, cancel := context.WithTimeout(context.Background(), defaultShutdownTimeout)
			defer cancel()
			if err := app.Shutdown(shutdownCtx); err != nil {
				log.Printf("Warning: Cleanup error: %v", err)
			}
		}
	}()

	// Create the admin user
	if err := users.CreateAdminUser(db, email, password); err != nil {
		if errors.Is(err, users.ErrUserExists) {
			log.Printf("User %s already exists", email)
			return nil
		}
		return fmt.Errorf("failed to create user: %w", err)
	}

	return nil
}

// ChangeAdminPasswordCommand implements password update for existing admin user
type ChangeAdminPasswordCommand struct{}

// Name returns the command name
func (c *ChangeAdminPasswordCommand) Name() string {
	return "change-admin-password"
}

// Description returns the command description
func (c *ChangeAdminPasswordCommand) Description() string {
	return "Changes the password of an existing admin user"
}

// Execute implements the change-admin-password command
func (c *ChangeAdminPasswordCommand) Execute(ctx context.Context, app *internal.Application, args []string) error {
	reader := bufio.NewReader(os.Stdin)

	// Email input
	var email string
	if len(args) >= 1 {
		email = args[0]
	} else {
		fmt.Print("Enter admin email: ")
		input, _ := reader.ReadString('\n')
		email = strings.TrimSpace(input)
	}

	// Ensure email provided
	if email == "" {
		return fmt.Errorf("email is required")
	}

	// Get database connection
	var db *gorm.DB
	if app != nil {
		db = app.DBManager.GetConnection()
	} else {
		return fmt.Errorf("app initialization failed, cannot connect to database")
	}

	// Ensure user exists
	if _, err := users.FindByEmail(db, email); err != nil {
		return fmt.Errorf("user lookup failed: %w", err)
	}

	// Password input
	var newPassword string
	if len(args) >= 2 {
		newPassword = args[1]
	} else {
		fmt.Print("Enter new password: ")
		pwd1, _ := reader.ReadString('\n')
		pwd1 = strings.TrimSpace(pwd1)

		fmt.Print("Confirm new password: ")
		pwd2, _ := reader.ReadString('\n')
		pwd2 = strings.TrimSpace(pwd2)

		if pwd1 != pwd2 {
			return fmt.Errorf("passwords do not match")
		}
		newPassword = pwd1
	}

	if newPassword == "" {
		return fmt.Errorf("password cannot be empty")
	}

	if err := users.ChangePassword(db, email, newPassword); err != nil {
		return fmt.Errorf("failed to update password: %w", err)
	}

	fmt.Println("Password updated successfully")
	return nil
}

// StatusCommand implements a command to check the system status
type StatusCommand struct{}

// Name returns the command name
func (c *StatusCommand) Name() string {
	return "status"
}

// Description returns the command description
func (c *StatusCommand) Description() string {
	return "Shows the current system status"
}

// Execute implements the status command
func (c *StatusCommand) Execute(ctx context.Context, app *internal.Application, args []string) error {
	if app == nil {
		return fmt.Errorf("cannot check status: app initialization failed")
	}

	// Get database connection
	db := app.DBManager.GetConnection()

	// Check database status
	var count int64
	if err := db.Model(&users.User{}).Count(&count).Error; err != nil {
		return fmt.Errorf("database error: %w", err)
	}

	log.Println("System Status:")
	log.Println("- Database: Connected")
	log.Printf("- Users: %d", count)

	// Check database statistics
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("failed to get SQL DB: %w", err)
	}

	log.Printf("- Max Open Connections: %d", sqlDB.Stats().MaxOpenConnections)
	log.Printf("- Open Connections: %d", sqlDB.Stats().OpenConnections)
	log.Printf("- In Use: %d", sqlDB.Stats().InUse)
	log.Printf("- Idle: %d", sqlDB.Stats().Idle)

	return nil
}

// HelpCommand implements a command to show usage information
type HelpCommand struct{}

// Name returns the command name
func (c *HelpCommand) Name() string {
	return "help"
}

// Description returns the command description
func (c *HelpCommand) Description() string {
	return "Shows usage information"
}

// Execute implements the help command
func (c *HelpCommand) Execute(ctx context.Context, app *internal.Application, args []string) error {
	fmt.Println("Usage: fnctl [command] [args...]")
	fmt.Println("Available commands:")

	for _, cmd := range commands {
		fmt.Printf("  %s: %s\n", cmd.Name(), cmd.Description())
	}

	return nil
}

// MigrateCommand runs database migrations
type MigrateCommand struct{}

func (c *MigrateCommand) Name() string        { return "migrate" }
func (c *MigrateCommand) Description() string { return "Runs database migrations" }

func (c *MigrateCommand) Execute(ctx context.Context, app *internal.Application, args []string) error {
	if app == nil {
		return fmt.Errorf("app initialization failed, cannot run migrations")
	}

	log.Println("Running database migrations...")

	// Use the app's DBManager directly (no singleton)
	if err := app.DBManager.MigrateDatabase(); err != nil {
		return fmt.Errorf("migration failed: %w", err)
	}

	log.Println("Migrations completed successfully")
	return nil
}

// SeedCommand populates the DB with test data
type SeedCommand struct{}

func (c *SeedCommand) Name() string        { return "seed" }
func (c *SeedCommand) Description() string { return "Seeds the database with sample data" }

func (c *SeedCommand) Execute(ctx context.Context, app *internal.Application, args []string) error {
	fs := flag.NewFlagSet("seed", flag.ContinueOnError)
	events := fs.Int("events", 10000, "number of events to generate")
	domain := fs.String("domain", "", "specific domain to seed (seeds all defaults if empty)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if app == nil {
		return fmt.Errorf("unable to initialise app")
	}

	se := seeder.NewSeeder(app.DBManager, slog.Default(), *events)

	// If a specific domain is provided, seed only that domain
	if *domain != "" {
		if err := se.SeedDomain(ctx, *domain); err != nil {
			return err
		}
		return nil
	}

	if err := se.Run(ctx); err != nil {
		return err
	}
	return nil
}

// Helper functions

// parseArgs parses the command name and arguments
func parseArgs() (string, []string) {
	args := os.Args[1:]
	if len(args) == 0 {
		return "help", []string{}
	}
	return args[0], args[1:]
}

// findCommand finds a command by name
func findCommand(name string) Command {
	for _, cmd := range commands {
		if cmd.Name() == name {
			return cmd
		}
	}
	return nil
}

// showUsageAndExit shows usage information and exits
func showUsageAndExit() {
	fmt.Println("Usage: fnctl [command] [args...]")
	fmt.Println("Available commands:")

	for _, cmd := range commands {
		fmt.Printf("  %s: %s\n", cmd.Name(), cmd.Description())
	}

	os.Exit(1)
}

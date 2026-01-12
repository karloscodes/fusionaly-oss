package admin

import (
	"fmt"

	"fusionaly/internal/manager/database"
	"fusionaly/internal/manager/docker"
	"fusionaly/internal/manager/logging"
)

// Manager handles administrative user operations inside the running container.
// It is decoupled from the Installer/Updater flows so it can be invoked
// separately (e.g. via `fusionaly change-admin-password`).

type dockerExecutor interface {
	ExecuteCommand(args ...string) error
}

type Manager struct {
	docker dockerExecutor
	logger *logging.Logger
}

// NewManager creates a Manager with default docker executor.
func NewManager(logger *logging.Logger) *Manager {
	db := database.NewDatabase(logger)
	d := docker.NewDocker(logger, db)
	return &Manager{docker: d, logger: logger}
}

// withExecutor is used in tests to inject a fake executor.
func newManagerWithExecutor(logger *logging.Logger, exec dockerExecutor) *Manager {
	return &Manager{docker: exec, logger: logger}
}

// CreateAdminUser creates the initial admin user inside the container.
func (m *Manager) CreateAdminUser(email, password string) error {
	err := m.docker.ExecuteCommand("/app/fnctl", "create-admin-user", email, password)
	if err != nil {
		return fmt.Errorf("failed to create admin user: %w", err)
	}
	return nil
}

// ChangeAdminPassword changes the password of an existing admin user.
func (m *Manager) ChangeAdminPassword(email, newPassword string) error {
	m.logger.InfoWithTime("Changing admin password for %s", email)
	err := m.docker.ExecuteCommand("/app/fnctl", "change-admin-password", email, newPassword)
	if err != nil {
		return fmt.Errorf("failed to change admin password: %w", err)
	}
	m.logger.Success("Password changed for %s", email)
	return nil
}

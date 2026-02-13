package requirements

import (
	"fmt"
	"net"
	"os"

	"fusionaly/internal/manager/logging"
)

type Checker struct {
	logger *logging.Logger
}

func NewChecker(logger *logging.Logger) *Checker {
	return &Checker{
		logger: logger,
	}
}

// CheckSystemRequirements performs all system requirement checks
func (c *Checker) CheckSystemRequirements() error {
	// Root privilege check
	if err := c.checkRootPrivileges(); err != nil {
		return err
	}

	// Port availability check
	if err := c.checkPortAvailability(); err != nil {
		return err
	}

	return nil
}

// checkRootPrivileges verifies that the installer is running with root privileges
func (c *Checker) checkRootPrivileges() error {
	if os.Geteuid() != 0 && os.Getenv("ENV") != "test" {
		return fmt.Errorf("root privileges required")
	}
	return nil
}

// checkPortAvailability verifies that required ports are available
func (c *Checker) checkPortAvailability() error {
	// Skip port checking in integration tests
	if os.Getenv("SKIP_PORT_CHECKING") == "1" {
		return nil
	}

	if !c.checkPort(80) {
		return fmt.Errorf("port 80 is not available")
	}

	if !c.checkPort(443) {
		return fmt.Errorf("port 443 is not available")
	}

	return nil
}

// checkPort checks if a specific port is available
func (c *Checker) checkPort(port int) bool {
	address := fmt.Sprintf("localhost:%d", port)
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return false
	}
	listener.Close()
	return true
}

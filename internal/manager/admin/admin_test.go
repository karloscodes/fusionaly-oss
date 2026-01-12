package admin

import (
	"fmt"
	"reflect"
	"testing"

	"fusionaly/internal/manager/logging"
)

type fakeExecutor struct {
	cmds      [][]string
	failAfter int // fail after N commands; 0 means no fail unless failAfter==1 etc.
}

func (f *fakeExecutor) ExecuteCommand(args ...string) error {
	copyArgs := make([]string, len(args))
	copy(copyArgs, args)
	f.cmds = append(f.cmds, copyArgs)
	if f.failAfter != 0 && len(f.cmds) >= f.failAfter {
		return fmt.Errorf("executor failure")
	}
	return nil
}

// makeFakeManager returns a Manager wired with a fake executor for testing.
func makeFakeManager() (*Manager, *fakeExecutor) {
	logger := logging.NewLogger(logging.Config{Level: "debug"})
	fe := &fakeExecutor{}
	mgr := newManagerWithExecutor(logger, fe)
	return mgr, fe
}

func TestCreateAdminUser(t *testing.T) {
	mgr, fe := makeFakeManager()
	email := "test@example.com"
	pass := "password123"
	if err := mgr.CreateAdminUser(email, pass); err != nil {
		t.Fatalf("CreateAdminUser returned error: %v", err)
	}
	want := [][]string{{"/app/fnctl", "create-admin-user", email, pass}}
	if !reflect.DeepEqual(fe.cmds, want) {
		t.Errorf("commands mismatch\nwant %#v\ngot  %#v", want, fe.cmds)
	}
}

func TestChangeAdminPassword(t *testing.T) {
	mgr, fe := makeFakeManager()
	email := "test@example.com"
	pass := "newpass123"
	if err := mgr.ChangeAdminPassword(email, pass); err != nil {
		t.Fatalf("ChangeAdminPassword returned error: %v", err)
	}
	want := [][]string{{"/app/fnctl", "change-admin-password", email, pass}}
	if !reflect.DeepEqual(fe.cmds, want) {
		t.Errorf("commands mismatch\nwant %#v\ngot  %#v", want, fe.cmds)
	}
}

func TestCreateAdminUser_Error(t *testing.T) {
	mgr, fe := makeFakeManager()
	fe.failAfter = 1
	if err := mgr.CreateAdminUser("x@y.com", "passw0rd"); err == nil {
		t.Fatal("expected error but got nil")
	}
}

func TestChangeAdminPassword_Error(t *testing.T) {
	mgr, fe := makeFakeManager()
	fe.failAfter = 1
	if err := mgr.ChangeAdminPassword("x@y.com", "pass123"); err == nil {
		t.Fatal("expected error but got nil")
	}
}

func TestSequenceCommands(t *testing.T) {
	mgr, fe := makeFakeManager()
	if err := mgr.CreateAdminUser("a@b.com", "pass1234"); err != nil {
		t.Fatal(err)
	}
	if err := mgr.ChangeAdminPassword("a@b.com", "pass4321"); err != nil {
		t.Fatal(err)
	}
	want := [][]string{
		{"/app/fnctl", "create-admin-user", "a@b.com", "pass1234"},
		{"/app/fnctl", "change-admin-password", "a@b.com", "pass4321"},
	}
	if !reflect.DeepEqual(fe.cmds, want) {
		t.Errorf("sequence commands mismatch\nwant %#v\ngot  %#v", want, fe.cmds)
	}
}

func TestChangeAdminPassword_FailsExecutor(t *testing.T) {
	logger := logging.NewLogger(logging.Config{Level: "error"})
	fe := &fakeExecutor{failAfter: 1}
	mgr := newManagerWithExecutor(logger, fe)
	// Expect failure on first call
	err := mgr.ChangeAdminPassword("x@y.com", "pass")
	if err == nil {
		t.Fatalf("expected error but got nil")
	}
	if len(fe.cmds) != 1 {
		t.Fatalf("expected 1 command recorded, got %d", len(fe.cmds))
	}
}

func TestAdminUserCreation(t *testing.T) {
	t.Run("CreateUserWithValidCredentials", func(t *testing.T) {
		mgr, fe := makeFakeManager()
		email := "admin@company.com"
		password := "SecurePassword123"
		
		err := mgr.CreateAdminUser(email, password)
		
		if err != nil {
			t.Errorf("Expected admin user creation to succeed, got error: %v", err)
		}
		
		expectedCmd := [][]string{{"/app/fnctl", "create-admin-user", email, password}}
		if !reflect.DeepEqual(fe.cmds, expectedCmd) {
			t.Errorf("Expected create-admin-user command, got: %v", fe.cmds)
		}
	})

	t.Run("CreateUserFailsOnSystemError", func(t *testing.T) {
		mgr, fe := makeFakeManager()
		fe.failAfter = 1
		
		err := mgr.CreateAdminUser("admin@test.com", "password123")
		
		if err == nil {
			t.Error("Expected admin user creation to fail when system fails")
		}
	})
}

func TestAdminPasswordManagement(t *testing.T) {
	t.Run("ChangePasswordExecutesCorrectCommand", func(t *testing.T) {
		mgr, fe := makeFakeManager()
		email := "admin@company.com"
		newPassword := "NewSecurePassword456"
		
		err := mgr.ChangeAdminPassword(email, newPassword)
		
		if err != nil {
			t.Errorf("Expected password change to succeed, got error: %v", err)
		}
		
		expectedCmd := [][]string{{"/app/fnctl", "change-admin-password", email, newPassword}}
		if !reflect.DeepEqual(fe.cmds, expectedCmd) {
			t.Errorf("Expected change-admin-password command, got: %v", fe.cmds)
		}
	})

	t.Run("ChangePasswordFailsOnSystemError", func(t *testing.T) {
		mgr, fe := makeFakeManager()
		fe.failAfter = 1
		
		err := mgr.ChangeAdminPassword("admin@test.com", "newpassword")
		
		if err == nil {
			t.Error("Expected password change to fail when system fails")
		}
	})
}

func TestAdminWorkflow(t *testing.T) {
	t.Run("InstallationFlowCreateUserThenChangePassword", func(t *testing.T) {
		mgr, fe := makeFakeManager()
		email := "admin@company.com"
		initialPassword := "InitialPass123"
		newPassword := "UpdatedPass456"
		
		// Create admin during installation
		err1 := mgr.CreateAdminUser(email, initialPassword)
		if err1 != nil {
			t.Fatalf("Admin creation failed: %v", err1)
		}
		
		// Later change password
		err2 := mgr.ChangeAdminPassword(email, newPassword)
		if err2 != nil {
			t.Fatalf("Password change failed: %v", err2)
		}
		
		expectedCmds := [][]string{
			{"/app/fnctl", "create-admin-user", email, initialPassword},
			{"/app/fnctl", "change-admin-password", email, newPassword},
		}
		
		if !reflect.DeepEqual(fe.cmds, expectedCmds) {
			t.Errorf("Expected admin workflow commands, got: %v", fe.cmds)
		}
	})
}

package onboarding_test

import (
	"fmt"
	"io"
	"log/slog"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"

	"fusionaly/internal/onboarding"
	"fusionaly/internal/testsupport"
	"fusionaly/internal/users"
)

func TestConcurrentOnboardingCompletion(t *testing.T) {
	t.Run("with many requests at once, only one admin is created", func(t *testing.T) {
		dbManager, _ := testsupport.SetupTestDBManager(t)
		db := dbManager.GetConnection()
		testsupport.CleanAllTables(db)
		logger := slog.New(slog.NewTextHandler(io.Discard, nil))
		const requests = 10
		start := make(chan struct{})
		errs := make([]error, requests)
		var wg sync.WaitGroup

		for i := range requests {
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-start
				_, errs[i] = onboarding.CompleteOnboarding(dbManager, logger, onboarding.CompletionData{
					Email:        fmt.Sprintf("admin%d@example.com", i),
					PasswordHash: "$2a$10$hash",
				})
			}()
		}
		close(start)
		wg.Wait()

		var count int64
		db.Model(&users.User{}).Count(&count)
		assert.Equal(t, int64(1), count)
		succeeded := 0
		for _, err := range errs {
			if err == nil {
				succeeded++
			} else {
				assert.ErrorIs(t, err, onboarding.ErrSetupAlreadyComplete)
			}
		}
		assert.Equal(t, 1, succeeded)
	})
}

// internal/orchestrator/flag.go
package orchestrator

import (
	"fmt"
	"os"

	"github.com/gofrs/flock"
)

// FlagManager manages the acquisition and release of operation flags using file locks.
type FlagManager struct {
	flock    *flock.Flock
	flagPath string
}

// NewFlagManager creates a new FlagManager.
func NewFlagManager(flagPath string) *FlagManager {
	return &FlagManager{
		flock:    flock.New(flagPath + ".lock"),
		flagPath: flagPath,
	}
}

// Acquire acquires the lock and creates the flag file atomically.
func (fm *FlagManager) Acquire() error {
	locked, err := fm.flock.TryLock()
	if err != nil {
		return fmt.Errorf("failed to acquire lock: %w", err)
	}
	if !locked {
		return fmt.Errorf("another operation is in progress")
	}

	file, err := os.OpenFile(fm.flagPath, os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		fm.flock.Unlock()
		if os.IsExist(err) {
			return fmt.Errorf("another operation is in progress")
		}
		return fmt.Errorf("creating flag file: %w", err)
	}
	file.Close()
	return nil
}

// Release releases the lock and removes the flag file.
func (fm *FlagManager) Release() error {
	if err := os.Remove(fm.flagPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("removing flag file: %w", err)
	}
	if err := fm.flock.Unlock(); err != nil {
		return fmt.Errorf("unlocking file: %w", err)
	}
	return nil
}

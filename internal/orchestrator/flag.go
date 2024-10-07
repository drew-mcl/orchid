// internal/orchestrator/flag.go
package orchestrator

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/gofrs/flock"
)

// FlagMetadata holds the information to be stored in the flag file.
type FlagMetadata struct {
	PipelineID  string    `json:"pipeline_id,omitempty"`
	CommitRef   string    `json:"commit_ref,omitempty"`
	ProjectName string    `json:"project_name,omitempty"`
	Environment string    `json:"environment,omitempty"`
	AcquiredAt  time.Time `json:"acquired_at"`
}

// FlagManager manages the acquisition and release of operation flags using file locks.
type FlagManager struct {
	flock         *flock.Flock
	flagPath      string
	metadata      FlagMetadata
	metadataBytes []byte
}

// NewFlagManager creates a new FlagManager.
func NewFlagManager(flagPath string, environment string) *FlagManager {
	return &FlagManager{
		flock:    flock.New(flagPath + ".lock"),
		flagPath: flagPath,
		metadata: FlagMetadata{
			PipelineID:  os.Getenv("CI_PIPELINE_ID"),
			CommitRef:   os.Getenv("CI_COMMIT_REF_NAME"),
			ProjectName: os.Getenv("CI_PROJECT_NAME"),
			Environment: environment,
			AcquiredAt:  time.Now(),
		},
	}
}

// Acquire acquires the lock and creates the flag file atomically with metadata.
func (fm *FlagManager) Acquire() error {
	locked, err := fm.flock.TryLock()
	if err != nil {
		return fmt.Errorf("failed to acquire lock: %w", err)
	}
	if !locked {
		return fmt.Errorf("another operation is in progress")
	}

	fm.metadataBytes, err = json.MarshalIndent(fm.metadata, "", "  ")
	if err != nil {
		fm.flock.Unlock()
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	file, err := os.OpenFile(fm.flagPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if err != nil {
		fm.flock.Unlock()
		if os.IsExist(err) {
			return fmt.Errorf("another operation is in progress")
		}
		return fmt.Errorf("creating flag file: %w", err)
	}
	defer file.Close()

	if _, err := file.Write(fm.metadataBytes); err != nil {
		fm.flock.Unlock()
		return fmt.Errorf("writing to flag file: %w", err)
	}

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

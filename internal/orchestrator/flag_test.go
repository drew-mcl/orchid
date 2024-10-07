// internal/orchestrator/flag_test.go
package orchestrator

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// TestFlagManager_AcquireRelease verifies that the flag can be acquired and released correctly,
// and that the flag file contains the appropriate metadata.
func TestFlagManager_AcquireRelease(t *testing.T) {
	tmpDir := t.TempDir()
	flagPath := filepath.Join(tmpDir, "test_flag")

	os.Setenv("CI_PIPELINE_ID", "12345")
	os.Setenv("CI_COMMIT_REF_NAME", "main")
	os.Setenv("CI_PROJECT_NAME", "orchid_project")
	os.Setenv("CI_ENVIRONMENT_NAME", "test_env")

	defer func() {
		os.Unsetenv("CI_PIPELINE_ID")
		os.Unsetenv("CI_COMMIT_REF_NAME")
		os.Unsetenv("CI_PROJECT_NAME")
		os.Unsetenv("CI_ENVIRONMENT_NAME")
	}()

	fm := NewFlagManager(flagPath, "test_env")

	err := fm.Acquire()
	if err != nil {
		t.Fatalf("unexpected error during acquire: %v", err)
	}

	if _, err := os.Stat(flagPath); os.IsNotExist(err) {
		t.Fatalf("expected flag file to be created")
	}

	data, err := os.ReadFile(flagPath)
	if err != nil {
		t.Fatalf("failed to read flag file: %v", err)
	}

	var metadata FlagMetadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		t.Fatalf("failed to unmarshal flag metadata: %v", err)
	}

	if metadata.PipelineID != "12345" {
		t.Errorf("expected PipelineID '12345', got '%s'", metadata.PipelineID)
	}
	if metadata.CommitRef != "main" {
		t.Errorf("expected CommitRef 'main', got '%s'", metadata.CommitRef)
	}
	if metadata.ProjectName != "orchid_project" {
		t.Errorf("expected ProjectName 'orchid_project', got '%s'", metadata.ProjectName)
	}
	if metadata.Environment != "test_env" {
		t.Errorf("expected Environment 'test_env', got '%s'", metadata.Environment)
	}
	if metadata.AcquiredAt.IsZero() {
		t.Errorf("expected AcquiredAt to be set, got zero value")
	}

	err = fm.Release()
	if err != nil {
		t.Fatalf("unexpected error during release: %v", err)
	}

	if _, err := os.Stat(flagPath); !os.IsNotExist(err) {
		t.Fatalf("expected flag file to be removed")
	}
}

// TestFlagManager_LockContention verifies that only one FlagManager can acquire the lock at a time.
func TestFlagManager_LockContention(t *testing.T) {
	tmpDir := t.TempDir()
	flagPath := filepath.Join(tmpDir, "test_flag")

	os.Setenv("CI_PIPELINE_ID", "67890")
	os.Setenv("CI_COMMIT_REF_NAME", "develop")
	os.Setenv("CI_PROJECT_NAME", "orchid_project2")
	os.Setenv("CI_ENVIRONMENT_NAME", "test_env")

	defer func() {
		os.Unsetenv("CI_PIPELINE_ID")
		os.Unsetenv("CI_COMMIT_REF_NAME")
		os.Unsetenv("CI_PROJECT_NAME")
		os.Unsetenv("CI_ENVIRONMENT_NAME")
	}()

	fm1 := NewFlagManager(flagPath, "test_env")
	fm2 := NewFlagManager(flagPath, "test_env")

	err := fm1.Acquire()
	if err != nil {
		t.Fatalf("unexpected error during acquire by fm1: %v", err)
	}

	err = fm2.Acquire()
	if err == nil {
		t.Fatalf("expected error during acquire by fm2, but got none")
	}

	err = fm1.Release()
	if err != nil {
		t.Fatalf("unexpected error during release by fm1: %v", err)
	}
}

// TestFlagManager_ReleaseWithoutAcquire ensures that releasing without acquiring does not cause errors.
func TestFlagManager_ReleaseWithoutAcquire(t *testing.T) {
	tmpDir := t.TempDir()
	flagPath := filepath.Join(tmpDir, "test_flag")

	fm := NewFlagManager(flagPath, "test_env")

	err := fm.Release()
	if err != nil {
		t.Fatalf("unexpected error during release without acquire: %v", err)
	}
}

// TestFlagManager_ErrorHandling verifies that acquiring a flag in a non-existent directory returns an error.
func TestFlagManager_ErrorHandling(t *testing.T) {
	flagPath := "/non_existent_dir/test_flag"

	fm := NewFlagManager(flagPath, "test_env")

	err := fm.Acquire()
	if err == nil {
		t.Fatalf("expected error during acquire, but got none")
	}
}

package orchestrator

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFlagManager_AcquireRelease(t *testing.T) {
	tmpDir := t.TempDir()
	flagPath := filepath.Join(tmpDir, "test_flag")

	fm := NewFlagManager(flagPath)

	err := fm.Acquire()
	if err != nil {
		t.Fatalf("unexpected error during acquire: %v", err)
	}

	if _, err := os.Stat(flagPath); os.IsNotExist(err) {
		t.Fatalf("expected flag file to be created")
	}

	err = fm.Release()
	if err != nil {
		t.Fatalf("unexpected error during release: %v", err)
	}

	if _, err := os.Stat(flagPath); !os.IsNotExist(err) {
		t.Fatalf("expected flag file to be removed")
	}
}

func TestFlagManager_LockContention(t *testing.T) {
	tmpDir := t.TempDir()
	flagPath := filepath.Join(tmpDir, "test_flag")

	fm1 := NewFlagManager(flagPath)
	fm2 := NewFlagManager(flagPath)

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

func TestFlagManager_ReleaseWithoutAcquire(t *testing.T) {
	tmpDir := t.TempDir()
	flagPath := filepath.Join(tmpDir, "test_flag")

	fm := NewFlagManager(flagPath)

	err := fm.Release()
	if err != nil {
		t.Fatalf("unexpected error during release without acquire: %v", err)
	}
}

func TestFlagManager_ErrorHandling(t *testing.T) {
	flagPath := "/non_existent_dir/test_flag"

	fm := NewFlagManager(flagPath)

	err := fm.Acquire()
	if err == nil {
		t.Fatalf("expected error during acquire, but got none")
	}
}

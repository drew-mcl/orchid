package config

import (
	"os"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	t.Run("valid config", func(t *testing.T) {
		validConfig := `environments:
  dev:
    remote_user: devuser
    applications:
      - name: app1
        host: 127.0.0.1
        start_command: start app1
        stop_command: stop app1
        check_command: check app1
        check_interval: 5`

		filePath := createTempFile(t, validConfig)
		defer func() {
			if err := os.Remove(filePath); err != nil {
				t.Errorf("unable to remove temp file: %v", err)
			}
		}()

		_, err := LoadConfig(filePath)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
	})

	t.Run("missing remote_user", func(t *testing.T) {
		invalidConfig := `environments:
  dev:
    remote_user: devuser
    applications:
      - name: app1
        host: 127.0.0.1
        start_command: start app1
        stop_command: stop app1
        check_command: check app1
        check_interval: 0`

		filePath := createTempFile(t, invalidConfig)
		defer func() {
			if err := os.Remove(filePath); err != nil {
				t.Errorf("unable to remove temp file: %v", err)
			}
		}()

		_, err := LoadConfig(filePath)
		if err == nil {
			t.Errorf("expected an error for missing remote_user, got none")
		}
	})

	t.Run("invalid application config", func(t *testing.T) {
		invalidConfig := `environments:
  dev:
    remote_user: devuser
    applications:
      - name: app1
        host: 127.0.0.1
        start_command: start app1
        stop_command: stop app1
        check_command: check app1
        check_interval: 0`

		filePath := createTempFile(t, invalidConfig)
		defer func() {
			if err := os.Remove(filePath); err != nil {
				t.Errorf("unable to remove temp file: %v", err)
			}
		}()

		_, err := LoadConfig(filePath)
		if err == nil {
			t.Errorf("expected an error for invalid application config, got none")
		}
	})

	t.Run("invalid check_interval", func(t *testing.T) {
		invalidConfig := `environments:
  dev:
    remote_user: devuser
    applications:
      - name: app1
        host: 127.0.0.1
        start_command: start app1
        stop_command: stop app1
        check_command: check app1
        check_interval: 0`

		filePath := createTempFile(t, invalidConfig)
		defer func() {
			if err := os.Remove(filePath); err != nil {
				t.Errorf("unable to remove temp file: %v", err)
			}
		}()

		_, err := LoadConfig(filePath)
		if err == nil {
			t.Errorf("expected an error for invalid check_interval, got none")
		}
	})
}

func createTempFile(t *testing.T, content string) string {
	t.Helper()
	tempFile, err := os.CreateTemp("", "config-*.yaml")
	if err != nil {
		t.Fatalf("unable to create temp file: %v", err)
	}

	if err := os.WriteFile(tempFile.Name(), []byte(content), 0644); err != nil {
		t.Fatalf("unable to write to temp file: %v", err)
	}

	return tempFile.Name()
}

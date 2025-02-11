package notdiamond

import (
	"fmt"
	"os"
	"testing"

	"github.com/Not-Diamond/go-notdiamond/database"
)

func TestMain(m *testing.M) {

	// Create a temporary directory for all database files during tests.
	tmpDir, err := os.MkdirTemp("", "test-db-")
	if err != nil {
		_, err := fmt.Fprintf(os.Stderr, "Failed to create temp dir: %v\n", err)
		if err != nil {
			return
		}
		os.Exit(1)
	}

	// setJSON the global dataFolder for the innerDB package.
	database.DataFolder = tmpDir

	// Run tests
	code := m.Run()

	// Cleanup: remove the temporary directory.
	if err := os.RemoveAll(tmpDir); err != nil {
		_, err := fmt.Fprintf(os.Stderr, "Failed to remove temp dir: %v\n", err)
		if err != nil {
			return
		}
	}
	os.Exit(code)
}

package utils

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// GetDefaultDBPathOnly returns a system-appropriate default path for the database
func GetDefaultDBPathOnly() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "recall.db"
	}

	switch runtime.GOOS {
	case "windows":
		return filepath.Join(homeDir, "AppData", "Roaming", "recall", "recall.db")
	case "darwin":
		return filepath.Join(homeDir, "Library", "Application Support", "recall", "recall.db")
	default: // Primarily Linux, but also other UNIX-like systems.
		return filepath.Join(homeDir, ".local", "share", "recall", "recall.db")
	}
}

func ResolveAndEnsureDBPath(providedPath string) (string, error) {
	targetPath := providedPath
	if targetPath == "" {
		targetPath = GetDefaultDBPathOnly()
	}

	if strings.HasPrefix(targetPath, "~/") {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to get user home directory to expand path '%s': %w", targetPath, err)
		}
		targetPath = filepath.Join(homeDir, targetPath[2:])
	}

	absPath, err := filepath.Abs(targetPath)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute path for '%s': %w", targetPath, err)
	}
	targetPath = absPath

	dbDir := filepath.Dir(targetPath)
	if _, err := os.Stat(dbDir); os.IsNotExist(err) {
		if err := os.MkdirAll(dbDir, 0755); err != nil { // 0755 gives rwx for user, rx for group/other
			return "", fmt.Errorf("failed to create directory '%s' for database: %w", dbDir, err)
		}
	} else if err != nil {
		// Some other error occurred when checking the directory.
		return "", fmt.Errorf("failed to stat directory '%s' for database: %w", dbDir, err)
	}

	return targetPath, nil
}

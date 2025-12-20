package cmd

import (
	"os"
	"path/filepath"
)

// getKubeconfigPath returns the kubeconfig path from flag, env var, or default location
func getKubeconfigPath(flagPath string) string {
	// Priority: flag > env var > default location
	if flagPath != "" {
		if _, err := os.Stat(flagPath); err == nil {
			return flagPath
		}
	}

	if envPath := os.Getenv("KUBECONFIG"); envPath != "" {
		if _, err := os.Stat(envPath); err == nil {
			return envPath
		}
	}

	// Default location
	defaultPath := filepath.Join(os.Getenv("HOME"), ".kube", "config")
	if _, err := os.Stat(defaultPath); err == nil {
		return defaultPath
	}

	return ""
}


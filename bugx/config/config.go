package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

const (
	configDirName  = ".bugx"
	configFileName = "config.json"
	tokenFileName  = "token"
)

// Config manages CLI configuration
type Config struct {
	configDir string
}

// NewConfig creates a new config instance
func NewConfig() *Config {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		// Fallback to current directory
		homeDir = "."
	}

	configDir := filepath.Join(homeDir, configDirName)
	return &Config{configDir: configDir}
}

// ensureConfigDir ensures the config directory exists
func (c *Config) ensureConfigDir() error {
	return os.MkdirAll(c.configDir, 0700)
}

// SaveToken saves the authentication token securely
func (c *Config) SaveToken(token string) error {
	if err := c.ensureConfigDir(); err != nil {
		return err
	}

	tokenPath := filepath.Join(c.configDir, tokenFileName)
	
	// Set restrictive permissions (owner read/write only)
	file, err := os.OpenFile(tokenPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = file.WriteString(token)
	return err
}

// LoadToken loads the authentication token
func (c *Config) LoadToken() (string, error) {
	tokenPath := filepath.Join(c.configDir, tokenFileName)
	
	data, err := os.ReadFile(tokenPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("token file not found")
		}
		return "", err
	}

	return string(data), nil
}

// RemoveToken removes the saved token
func (c *Config) RemoveToken() error {
	tokenPath := filepath.Join(c.configDir, tokenFileName)
	
	if _, err := os.Stat(tokenPath); os.IsNotExist(err) {
		return nil // Token doesn't exist, nothing to remove
	}

	return os.Remove(tokenPath)
}

// SaveAPIURL saves the API URL
func (c *Config) SaveAPIURL(url string) error {
	cfg, err := c.loadConfig()
	if err != nil {
		cfg = make(map[string]interface{})
	}

	cfg["api_url"] = url
	return c.saveConfig(cfg)
}

// LoadAPIURL loads the API URL
func (c *Config) LoadAPIURL() (string, error) {
	cfg, err := c.loadConfig()
	if err != nil {
		return "", err
	}

	url, ok := cfg["api_url"].(string)
	if !ok {
		return "", fmt.Errorf("api_url not found in config")
	}

	return url, nil
}

// SaveClusterName saves the default cluster name
func (c *Config) SaveClusterName(clusterName string) error {
	cfg, err := c.loadConfig()
	if err != nil {
		cfg = make(map[string]interface{})
	}

	cfg["cluster_name"] = clusterName
	return c.saveConfig(cfg)
}

// LoadClusterName loads the default cluster name
func (c *Config) LoadClusterName() (string, error) {
	cfg, err := c.loadConfig()
	if err != nil {
		return "", err
	}

	clusterName, ok := cfg["cluster_name"].(string)
	if !ok {
		return "", fmt.Errorf("cluster_name not found in config")
	}

	return clusterName, nil
}

// loadConfig loads the config file
func (c *Config) loadConfig() (map[string]interface{}, error) {
	if err := c.ensureConfigDir(); err != nil {
		return nil, err
	}

	configPath := filepath.Join(c.configDir, configFileName)
	
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return make(map[string]interface{}), nil
		}
		return nil, err
	}

	var cfg map[string]interface{}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

// saveConfig saves the config file
func (c *Config) saveConfig(cfg map[string]interface{}) error {
	if err := c.ensureConfigDir(); err != nil {
		return err
	}

	configPath := filepath.Join(c.configDir, configFileName)
	
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	// Set restrictive permissions
	return os.WriteFile(configPath, data, 0600)
}

// GetConfigDir returns the config directory path
func (c *Config) GetConfigDir() string {
	return c.configDir
}

// GetOS returns the operating system
func GetOS() string {
	return runtime.GOOS
}


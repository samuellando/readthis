// Package config provides runtime configuration for ReadThis.
package config

import "os"

const (
	// defaultDataPath is the default JSON storage location.
	defaultDataPath = "./data/links.json"
	defaultAddr = ":8080"
	// dataPathEnv overrides the default data path.
	dataPathEnv = "READTHIS_DATA_PATH"
)

// Config holds server configuration values.
type Config struct {
	Addr     string
	DataPath string
}

// Load reads configuration defaults and environment overrides.
func Load() Config {
	dataPath := defaultDataPath
	if value := os.Getenv(dataPathEnv); value != "" {
		dataPath = value
	}

	return Config{
		Addr:     defaultAddr,
		DataPath: dataPath,
	}
}

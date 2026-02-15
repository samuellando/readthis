package config

import "os"

const (
	defaultDataPath = "./data/links.json"
	defaultAddr     = ":8080"

	dataPathEnv = "READTHIS_DATA_PATH"
)

type Config struct {
	Addr     string
	DataPath string
}

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

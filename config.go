package main

import (
	"bufio"
	"os"
	"strings"
)

var configMap = make(map[string]string)

// LoadConfig reads a simple key=value configuration file and populates the global configMap.
func LoadConfig(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.Trim(strings.TrimSpace(parts[1]), "\"")
			configMap[key] = value
		}
	}
	return scanner.Err()
}

// getConf retrieves a configuration value with the following priority:
// 1. Environment Variable
// 2. Value in configMap (from config.toml)
// 3. Default value provided as argument
func getConf(key, defaultValue string) string {
	// Check environment variable first
	if val := os.Getenv(key); val != "" {
		return val
	}
	// Check configMap (from config.toml)
	if val, ok := configMap[key]; ok {
		return val
	}
	// Fallback to default
	return defaultValue
}

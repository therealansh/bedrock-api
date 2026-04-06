package configs

import "time"

// Default returns the default configuration for the application.
func Default() *Config {
	return &Config{
		API: &APIConfig{
			LogLevel:                   "info",
			HTTPHost:                   "127.0.0.1",
			HTTPPort:                   8080,
			SocketHost:                 "127.0.0.1",
			SocketPort:                 8081,
			SocketHandlers:             1,
			FullStackMode:              false,
			DockerDHealthCheckInterval: 1 * time.Minute,
			SessionStatusCheckInterval: 30 * time.Second,
		},
		Dockerd: &DockerdConfig{
			Name:          "hostname",
			LogLevel:      "info",
			APISocketHost: "127.0.0.1",
			APISocketPort: 8081,
			APITimeout:    10 * time.Second,
			PullInterval:  30 * time.Second,
		},
		FileMD: &FileMDConfig{
			LogLevel:    "info",
			APIHTTPHost: "127.0.0.1",
			APIHTTPPort: 8081,
		},
	}
}

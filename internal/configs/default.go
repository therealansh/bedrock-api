package configs

// Default returns the default configuration for the application.
func Default() *Config {
	return &Config{
		API:     DefaultAPIConfig(),
		Dockerd: DefaultDockerdConfig(),
		FileMD:  DefaultFileMDConfig(),
	}
}

func DefaultAPIConfig() *APIConfig {
	return &APIConfig{
		LogLevel:                   "info",
		HTTPHost:                   "127.0.0.1",
		HTTPPort:                   8080,
		SocketHost:                 "127.0.0.1",
		SocketPort:                 8081,
		SocketHandlers:             1,
		FullStackMode:              false,
		DockerDHealthCheckInterval: "1m",
		SessionStatusCheckInterval: "30s",
		BedrockTracerImage:         "ghcr.io/amirhnajafiz/bedrock-tracer:v0.0.6-beta",
	}
}

func DefaultDockerdConfig() *DockerdConfig {
	return &DockerdConfig{
		Name:                      "hostname",
		LogLevel:                  "info",
		APISocketHost:             "127.0.0.1",
		APISocketPort:             8081,
		APITimeout:                "5s",
		PullInterval:              "10s",
		BedrockTracerImage:        "ghcr.io/amirhnajafiz/bedrock-tracer:v0.0.6-beta",
		DataDir:                   "/tmp/bedrock-logs",
		ContainerRuntimeInterface: "simulator",
	}
}

func DefaultFileMDConfig() *FileMDConfig {
	return &FileMDConfig{
		LogLevel:    "info",
		APIHTTPHost: "127.0.0.1",
		APIHTTPPort: 8081,
		DataDir:     "/tmp/bedrock-logs",
	}
}

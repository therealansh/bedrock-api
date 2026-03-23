package configs

// Default returns the default configuration for the application.
func Default() *Config {
	return &Config{
		API: &APIConfig{
			LogLevel:      "info",
			HTTPHost:      "127.0.0.1",
			HTTPPort:      8080,
			SocketHost:    "127.0.0.1",
			SocketPort:    8081,
			RunInFullMode: false,
		},
		Dockerd: &DockerdConfig{
			LogLevel:      "info",
			APISocketHost: "127.0.0.1",
			APISocketPort: 8081,
		},
		FileMD: &FileMDConfig{
			LogLevel:      "info",
			APISocketHost: "127.0.0.1",
			APISocketPort: 8081,
		},
	}
}

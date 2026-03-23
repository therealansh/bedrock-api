package configs

import (
	"encoding/json"

	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/providers/structs"
	"github.com/knadh/koanf/v2"
)

// APIConfig represents the configuration for the API server.
type APIConfig struct {
	LogLevel      string `koanf:"log_level"`
	HTTPHost      string `koanf:"http_host"`
	HTTPPort      int    `koanf:"http_port"`
	SocketHost    string `koanf:"socket_host"`
	SocketPort    int    `koanf:"socket_port"`
	RunInFullMode bool   `koanf:"run_in_full_mode"`
}

// DockerdConfig represents the configuration for the Docker Daemon.
type DockerdConfig struct {
	LogLevel      string `koanf:"log_level"`
	APISocketHost string `koanf:"api_socket_host"`
	APISocketPort int    `koanf:"api_socket_port"`
}

// FileMDConfig represents the configuration for the File Management Daemon.
type FileMDConfig struct {
	LogLevel      string `koanf:"log_level"`
	APISocketHost string `koanf:"api_socket_host"`
	APISocketPort int    `koanf:"api_socket_port"`
}

// Config represents the configuration for the application.
type Config struct {
	API     *APIConfig     `koanf:"api"`
	Dockerd *DockerdConfig `koanf:"dockerd"`
	FileMD  *FileMDConfig  `koanf:"filemd"`
}

func (c *Config) String() string {
	bytes, _ := json.MarshalIndent(c, "", "  ")
	return string(bytes)
}

// LoadConfig loads the configuration for the application.
func LoadConfig(cpath string) (*Config, error) {
	var instance Config

	k := koanf.New(".")

	// load default values
	if err := k.Load(structs.Provider(Default(), "koanf"), nil); err != nil {
		return nil, err
	}

	// load configurations from file
	if err := k.Load(file.Provider(cpath), yaml.Parser()); err != nil {
		return nil, err
	}

	// unmarshal the configuration into the Config struct
	if err := k.Unmarshal("", &instance); err != nil {
		return nil, err
	}

	return &instance, nil
}

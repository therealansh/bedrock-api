package configs

import (
	"encoding/json"
	"fmt"

	"github.com/go-playground/validator/v10"
	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/providers/structs"
	"github.com/knadh/koanf/v2"
)

// APIConfig represents the configuration for the API server.
type APIConfig struct {
	LogLevel       string `koanf:"log_level" validate:"oneof=debug info warn error"`
	HTTPHost       string `koanf:"http_host" validate:"ip"`
	HTTPPort       int    `koanf:"http_port" validate:"min=1,max=65535"`
	SocketHost     string `koanf:"socket_host" validate:"ip"`
	SocketPort     int    `koanf:"socket_port" validate:"min=1,max=65535"`
	SocketHandlers int    `koanf:"socket_handlers" validate:"min=1"`
	RunInFullMode  bool   `koanf:"run_in_full_mode"`
}

// DockerdConfig represents the configuration for the Docker Daemon.
type DockerdConfig struct {
	Name                string `koanf:"name"`
	LogLevel            string `koanf:"log_level" validate:"oneof=debug info warn error"`
	APISocketHost       string `koanf:"api_socket_host" validate:"ip"`
	APISocketPort       int    `koanf:"api_socket_port" validate:"min=1,max=65535"`
	APIConnectionRetrys int    `koanf:"api_connection_retrys" validate:"min=1"`
}

// FileMDConfig represents the configuration for the File Management Daemon.
type FileMDConfig struct {
	LogLevel    string `koanf:"log_level" validate:"oneof=debug info warn error"`
	APIHTTPHost string `koanf:"api_http_host" validate:"ip"`
	APIHTTPPort int    `koanf:"api_http_port" validate:"min=1,max=65535"`
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
		return nil, fmt.Errorf("koanf default values issue: %v", err)
	}

	// load configurations from file
	if err := k.Load(file.Provider(cpath), yaml.Parser()); err != nil {
		return nil, fmt.Errorf("koanf file loading issue: %v", err)
	}

	// unmarshal the configuration into the Config struct
	if err := k.Unmarshal("", &instance); err != nil {
		return nil, fmt.Errorf("koanf unmarshal issue: %v", err)
	}

	// validate the configuration
	validate := validator.New()
	if err := validate.Struct(&instance); err != nil {
		return nil, fmt.Errorf("configuration issue: %v", err)
	}

	return &instance, nil
}

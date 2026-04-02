package configs

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/providers/structs"
	"github.com/knadh/koanf/v2"
)

// APIConfig represents the configuration for the API server.
type APIConfig struct {
	LogLevel                   string `koanf:"log_level" validate:"oneof=debug info warn error"`
	HTTPHost                   string `koanf:"http_host" validate:"ip"`
	HTTPPort                   int    `koanf:"http_port" validate:"min=1,max=65535"`
	SocketHost                 string `koanf:"socket_host" validate:"ip"`
	SocketPort                 int    `koanf:"socket_port" validate:"min=1,max=65535"`
	SocketHandlers             int    `koanf:"socket_handlers" validate:"min=1"`
	FullStackMode              bool   `koanf:"full_stack_mode"`
	DockerDHealthCheckInterval string `koanf:"dockerd_health_check_interval" validate:"duration"`
	SessionStatusCheckInterval string `koanf:"session_status_check_interval" validate:"duration"`
	BedrockTracerImage         string `koanf:"bedrock_tracer_image"`
}

// DockerdConfig represents the configuration for the Docker Daemon.
type DockerdConfig struct {
	Name                      string `koanf:"name"`
	LogLevel                  string `koanf:"log_level" validate:"oneof=debug info warn error"`
	APISocketHost             string `koanf:"api_socket_host" validate:"ip"`
	APISocketPort             int    `koanf:"api_socket_port" validate:"min=1,max=65535"`
	APITimeout                string `koanf:"api_timeout" validate:"duration"`
	PullInterval              string `koanf:"pull_interval" validate:"duration"`
	BedrockTracerImage        string `koanf:"bedrock_tracer_image"`
	DataDir                   string `koanf:"data_dir"`
	ContainerRuntimeInterface string `koanf:"container_runtime_interface" validate:"oneof=simulator docker"`
}

// FileMDConfig represents the configuration for the File Management Daemon.
type FileMDConfig struct {
	LogLevel     string        `koanf:"log_level" validate:"oneof=debug info warn error"`
	APIHTTPHost  string        `koanf:"api_http_host" validate:"ip"`
	APIHTTPPort  int           `koanf:"api_http_port" validate:"min=1,max=65535"`
	DataDir      string        `koanf:"data_dir"`
	VolumePath   string        `koanf:"volume_path"`
	PollInterval time.Duration `koanf:"poll_interval" validate:"duration"`
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

func (c *APIConfig) String() string {
	bytes, _ := json.MarshalIndent(c, "", "  ")
	return string(bytes)
}

func (c *DockerdConfig) String() string {
	bytes, _ := json.MarshalIndent(c, "", "  ")
	return string(bytes)
}

func (c *FileMDConfig) String() string {
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
	validate.RegisterValidation("duration", ValidateDuration)
	if err := validate.Struct(&instance); err != nil {
		return nil, fmt.Errorf("configuration issue: %v", err)
	}

	return &instance, nil
}

// ValidateDuration is a custom validation function for time.Duration fields.
func ValidateDuration(fl validator.FieldLevel) bool {
	_, err := time.ParseDuration(fl.Field().String())
	return err == nil
}

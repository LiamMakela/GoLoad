package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server       ServerConfig       `yaml:"server"`
	LoadBalancer LoadBalancerConfig `yaml:"load_balancer"`
	Health       HealthConfig       `yaml:"health"`
	Proxy        ProxyConfig        `yaml:"proxy"`
	Backends     []BackendConfig    `yaml:"backends"`
	RateLimit    RateLimitConfig    `yaml:"rate_limit"`
}

type ServerConfig struct {
	Port int `yaml:"port"`
}

type LoadBalancerConfig struct {
	Strategy string `yaml:"strategy"`
}

type HealthConfig struct {
	Interval time.Duration `yaml:"interval"`
	Timeout  time.Duration `yaml:"timeout"`
}

type ProxyConfig struct {
	Timeout time.Duration `yaml:"timeout"`
	Retries int           `yaml:"retries"`
}

type BackendConfig struct {
	URL string `yaml:"url"`
}

type RateLimitConfig struct {
	Enabled           bool    `yaml:"enabled"`
	RequestsPerSecond float64 `yaml:"requests_per_second"`
	Burst             int     `yaml:"burst"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf(
			"read config: %w",
			err,
		)
	}

	var cfg Config

	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf(
			"parse config: %w",
			err,
		)
	}

	return &cfg, nil
}

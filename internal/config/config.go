package config

import (
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server       ServerConfig       `yaml:"server"`
	LoadBalancer LoadBalancerConfig `yaml:"load_balancer"`
	Health       HealthConfig       `yaml:"health"`
	Proxy        ProxyConfig        `yaml:"proxy"`
	RateLimit    RateLimitConfig    `yaml:"rate_limit"`
	Backends     []BackendConfig    `yaml:"backends"`
}

type RateLimitConfig struct {
	Enabled           bool    `yaml:"enabled"`
	RequestsPerSecond float64 `yaml:"requests_per_second"`
	Burst             int     `yaml:"burst"`
}

type ProxyConfig struct {
	Timeout time.Duration `yaml:"timeout"`
	Retries int           `yaml:"retries"`
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

type BackendConfig struct {
	Name string `yaml:"name"`
	URL  string `yaml:"url"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg Config

	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

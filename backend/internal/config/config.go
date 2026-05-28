package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// EndpointType controls how the proxy parses SSE and injects auth headers.
type EndpointType = string

const (
	TypeOpenAI          EndpointType = "openai"
	TypeAnthropic       EndpointType = "anthropic"
	TypeOpenAIResponses EndpointType = "openai_responses"
)

type Endpoint struct {
	Name       string `yaml:"name"`
	URL        string `yaml:"url"`
	Key        string `yaml:"key"`
	Type       string `yaml:"type"`        // openai | anthropic | openai_responses (default: openai)
	BearerAuth bool   `yaml:"bearer_auth"` // use Authorization: Bearer instead of x-api-key (for anthropic-type endpoints behind OpenAI-compatible gateways)
}

func (e *Endpoint) ResolvedType() string {
	if e.Type == "" {
		return TypeOpenAI
	}
	return e.Type
}

type Config struct {
	ProxyPort int        `yaml:"proxy_port"`
	MongoURI  string     `yaml:"mongodb_uri"`
	MongoDB   string     `yaml:"mongodb_db"`
	Default   string     `yaml:"default_endpoint"`
	Endpoints []Endpoint `yaml:"endpoints"`
	Debug     bool       `yaml:"debug"`
}

// Load merges: config.yaml → <base>.local.yaml → environment variables.
func Load(path string) (*Config, error) {
	cfg, err := loadYAML(path)
	if err != nil {
		return nil, err
	}

	localPath := strings.TrimSuffix(path, ".yaml") + ".local.yaml"
	if localCfg, err := loadYAML(localPath); err == nil {
		mergeConfig(cfg, localCfg)
	}

	applyEnv(cfg)

	if cfg.ProxyPort == 0 {
		cfg.ProxyPort = 8080
	}
	if cfg.MongoURI == "" {
		cfg.MongoURI = "mongodb://localhost:27017"
	}
	if cfg.MongoDB == "" {
		cfg.MongoDB = "api-tracker"
	}
	return cfg, nil
}

func loadYAML(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config %s: %w", path, err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config %s: %w", path, err)
	}
	return &cfg, nil
}

// mergeConfig overlays non-zero fields from src into dst.
func mergeConfig(dst, src *Config) {
	if src.ProxyPort != 0 {
		dst.ProxyPort = src.ProxyPort
	}
	if src.MongoURI != "" {
		dst.MongoURI = src.MongoURI
	}
	if src.MongoDB != "" {
		dst.MongoDB = src.MongoDB
	}
	if src.Default != "" {
		dst.Default = src.Default
	}
	for _, se := range src.Endpoints {
		found := false
		for i, de := range dst.Endpoints {
			if de.Name == se.Name {
				if se.Key != "" {
					dst.Endpoints[i].Key = se.Key
				}
				if se.URL != "" {
					dst.Endpoints[i].URL = se.URL
				}
				if se.Type != "" {
					dst.Endpoints[i].Type = se.Type
				}
				found = true
				break
			}
		}
		if !found {
			dst.Endpoints = append(dst.Endpoints, se)
		}
	}
}

// normalizeEnvName converts endpoint name to env var suffix: "openai-responses" → "OPENAI_RESPONSES".
func normalizeEnvName(name string) string {
	return strings.ToUpper(strings.ReplaceAll(name, "-", "_"))
}

// applyEnv reads AITRACE_* environment variables and overlays them (highest priority).
func applyEnv(cfg *Config) {
	if v := os.Getenv("APITRACKER_PROXY_PORT"); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			cfg.ProxyPort = p
		}
	}
	if v := os.Getenv("APITRACKER_MONGO_URI"); v != "" {
		cfg.MongoURI = v
	}
	if v := os.Getenv("APITRACKER_MONGO_DB"); v != "" {
		cfg.MongoDB = v
	}
	if v := os.Getenv("APITRACKER_DEFAULT_ENDPOINT"); v != "" {
		cfg.Default = v
	}
	if v := os.Getenv("APITRACKER_DEBUG"); v == "true" || v == "1" {
		cfg.Debug = true
	}
	for i, ep := range cfg.Endpoints {
		prefix := "APITRACKER_ENDPOINT_" + normalizeEnvName(ep.Name)
		if v := os.Getenv(prefix + "_KEY"); v != "" {
			cfg.Endpoints[i].Key = v
		}
		if v := os.Getenv(prefix + "_URL"); v != "" {
			cfg.Endpoints[i].URL = v
		}
	}
}

func (c *Config) EndpointMap() map[string]*Endpoint {
	m := make(map[string]*Endpoint, len(c.Endpoints))
	for i := range c.Endpoints {
		m[c.Endpoints[i].Name] = &c.Endpoints[i]
	}
	return m
}

func (c *Config) EndpointByName(name string) *Endpoint {
	if name == "" {
		return nil
	}
	for i := range c.Endpoints {
		if c.Endpoints[i].Name == name {
			return &c.Endpoints[i]
		}
	}
	return nil
}

func (c *Config) DefaultEndpoint() *Endpoint {
	if c.Default != "" {
		for i := range c.Endpoints {
			if c.Endpoints[i].Name == c.Default {
				return &c.Endpoints[i]
			}
		}
	}
	if len(c.Endpoints) > 0 {
		return &c.Endpoints[0]
	}
	return nil
}

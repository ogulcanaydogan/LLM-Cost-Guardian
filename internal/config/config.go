package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
)

// Config holds all LLM Cost Guardian configuration.
type Config struct {
	Storage  StorageConfig  `mapstructure:"storage"`
	Proxy    ProxyConfig    `mapstructure:"proxy"`
	Alerts   AlertsConfig   `mapstructure:"alerts"`
	Pricing  PricingConfig  `mapstructure:"pricing"`
	Logging  LoggingConfig  `mapstructure:"logging"`
	Defaults DefaultsConfig `mapstructure:"defaults"`
}

// StorageConfig defines database settings.
type StorageConfig struct {
	Path string `mapstructure:"path"`
}

// ProxyConfig defines transparent proxy settings.
type ProxyConfig struct {
	Listen        string `mapstructure:"listen"`
	ReadTimeout   string `mapstructure:"read_timeout"`
	WriteTimeout  string `mapstructure:"write_timeout"`
	MaxBodySize   int64  `mapstructure:"max_body_size"`
	DenyOnExceed  bool   `mapstructure:"deny_on_exceed"`
	AddCostHeaders bool  `mapstructure:"add_cost_headers"`
}

// AlertsConfig defines alerting integrations.
type AlertsConfig struct {
	Slack   SlackConfig   `mapstructure:"slack"`
	Webhook WebhookConfig `mapstructure:"webhook"`
}

// SlackConfig defines Slack webhook settings.
type SlackConfig struct {
	Enabled    bool   `mapstructure:"enabled"`
	WebhookURL string `mapstructure:"webhook_url"`
	Channel    string `mapstructure:"channel"`
}

// WebhookConfig defines generic webhook settings.
type WebhookConfig struct {
	Enabled bool   `mapstructure:"enabled"`
	URL     string `mapstructure:"url"`
	Secret  string `mapstructure:"secret"`
}

// PricingConfig defines pricing data settings.
type PricingConfig struct {
	Dir string `mapstructure:"dir"`
}

// LoggingConfig defines logging settings.
type LoggingConfig struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"`
}

// DefaultsConfig defines default values.
type DefaultsConfig struct {
	Project string `mapstructure:"project"`
}

// Load reads configuration from file and environment variables.
func Load(cfgFile string) (*Config, error) {
	v := viper.New()

	if cfgFile != "" {
		v.SetConfigFile(cfgFile)
	} else {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("find home directory: %w", err)
		}

		v.AddConfigPath(filepath.Join(home, ".lcg"))
		v.AddConfigPath(".")
		v.SetConfigName("config")
		v.SetConfigType("yaml")
	}

	// Defaults
	home, _ := os.UserHomeDir()
	v.SetDefault("storage.path", filepath.Join(home, ".lcg", "guardian.db"))
	v.SetDefault("proxy.listen", ":8080")
	v.SetDefault("proxy.read_timeout", "30s")
	v.SetDefault("proxy.write_timeout", "60s")
	v.SetDefault("proxy.max_body_size", 10*1024*1024) // 10 MB
	v.SetDefault("proxy.deny_on_exceed", false)
	v.SetDefault("proxy.add_cost_headers", true)
	v.SetDefault("pricing.dir", "pricing/")
	v.SetDefault("logging.level", "info")
	v.SetDefault("logging.format", "json")
	v.SetDefault("defaults.project", "default")
	v.SetDefault("alerts.slack.channel", "#llm-costs")

	// Environment variables
	v.SetEnvPrefix("LCG")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// Read config file (ignore if not found)
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("read config: %w", err)
		}
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}

	return &cfg, nil
}

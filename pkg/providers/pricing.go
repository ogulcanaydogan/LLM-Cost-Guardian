package providers

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// LoadPricing reads a YAML pricing file and returns the provider configuration.
func LoadPricing(path string) (*ProviderConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read pricing file %s: %w", path, err)
	}

	var cfg ProviderConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse pricing file %s: %w", path, err)
	}

	if cfg.Provider == "" {
		return nil, fmt.Errorf("pricing file %s: missing provider name", path)
	}
	if len(cfg.Models) == 0 {
		return nil, fmt.Errorf("pricing file %s: no models defined", path)
	}

	return &cfg, nil
}

// LoadPricingFromBytes parses YAML pricing data from raw bytes.
func LoadPricingFromBytes(data []byte) (*ProviderConfig, error) {
	var cfg ProviderConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse pricing data: %w", err)
	}
	return &cfg, nil
}

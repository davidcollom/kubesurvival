package simulate

import (
	"os"

	"gopkg.in/yaml.v2"
)

func NewConfig(provider, region string) *Config {
	cfg := &Config{
		Provider: provider,
		Nodes: ConfigNodes{
			AWS: &ProviderConfig{Region: region, InstanceTypes: []string{}},
			GCP: &ProviderConfig{Region: region, InstanceTypes: []string{}},
		},
	}
	// cfg.setProviderRegion(region)
	return cfg
}

func (c *Config) GetProviderConfig() ProviderConfig {
	switch c.Provider {
	case "aws":
		return *c.Nodes.AWS
	case "gcp":
		return *c.Nodes.GCP
	default:
		return *c.Nodes.AWS
	}
}

// func (c *Config) setProviderRegion(region string) {
// 	switch c.Provider {
// 	case "aws":
// 		c.Nodes.AWS.Region = region
// 	case "gcp":
// 		c.Nodes.GCP.Region = &region
// 	default:
// 		c.Nodes.AWS.Region = &region
// 	}
// }

func ParseConfig(filePath string) (*Config, error) {
	configFile, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	config := &Config{}
	err = yaml.Unmarshal(configFile, config)
	return config, err
}

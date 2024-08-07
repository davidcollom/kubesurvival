package simulate

type Result struct {
	InstanceType       string
	NodeCount          int64
	TotalPricePerMonth float64
}

type Config struct {
	Provider string      `yaml:"provider"`
	Mode     string      `yaml:"mode"`
	Nodes    ConfigNodes `yaml:"nodes"`
	Pods     string      `yaml:"pods"`
}

type ConfigNodes struct {
	MinNodes *int64          `yaml:"minNodes,omitempty"`
	MaxNodes *int64          `yaml:"maxNodes,omitempty"`
	GCP      *ProviderConfig `yaml:"gcp,omitempty"`
	AWS      *ProviderConfig `yaml:"aws,omitempty"`
}

type ProviderConfig struct {
	Region        string   `yaml:"region"`
	InstanceTypes []string `yaml:"instanceTypes"`
}

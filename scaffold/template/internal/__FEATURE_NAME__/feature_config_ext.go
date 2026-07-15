package __FEATURE_NAME__

import (
	"time"

	"github.com/zxzharmlesszxz/prometheus-exporter-framework/exporter/featurekit"
)

type Config struct {
	ConfigFile string
}

const DefaultRefreshInterval = time.Minute

var DefaultFeatureConfigFileName = "__FEATURE_CONFIG_FILE__"

// featureConfigFile is intentionally empty in the scaffold skeleton. Add YAML
// fields here and merge them into Config in ResolveFeatureConfig when the
// exporter grows domain-specific file configuration.
type featureConfigFile struct{}

// Use FeatureConfigFlagSpec.MarkSet for fields that need to distinguish an
// omitted CLI flag from an explicit CLI value when merging feature YAML.
var featureConfigFlagSpecs []featurekit.FeatureConfigFlagSpec[Config]

func NewDefaultConfig() Config {
	return Config{}
}

func FeatureConfigFile(config *Config) *string {
	return &config.ConfigFile
}

func ValidateFeatureConfig(_ Config) error {
	return nil
}

func FeatureRuntimeConfigEntries(_ featurekit.RuntimeConfigContext[Config], _ Config) []any {
	return nil
}

func ResolveFeatureConfig(featureName string, config Config) (Config, string, bool, error) {
	var fileConfig featureConfigFile
	cfgFile, loaded, err := featurekit.LoadFeatureConfigFile(featureName, config.ConfigFile, &fileConfig)
	// When the exporter adds domain-specific YAML fields, merge fileConfig into
	// config here before returning the resolved configuration.
	return config, cfgFile, loaded, err
}

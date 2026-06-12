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

type featureConfigFile struct{}

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
	return config, cfgFile, loaded, err
}

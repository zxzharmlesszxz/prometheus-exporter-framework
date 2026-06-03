package featurekit

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/alecthomas/kingpin/v2"
	framework "github.com/zxzharmlesszxz/prometheus-exporter-framework/exporter"
)

type extensionTestConfig struct {
	ConfigFile string
	Name       string
}

type extensionTestFileConfig struct {
	Name string `yaml:"name"`
}

type extensionTestSnapshot struct {
	Value float64
}

type extensionTestEngine struct {
	snapshot extensionTestSnapshot
}

func (e extensionTestEngine) Snapshot(context.Context, time.Time) extensionTestSnapshot {
	return e.snapshot
}

func TestRegisterFeatureConfigFlagSpecs(t *testing.T) {
	t.Parallel()

	type configFlagTestConfig struct {
		Name    string
		Targets []string
	}

	config := configFlagTestConfig{Name: "default"}
	app := kingpin.New("test", "")
	app.Terminate(func(int) {})
	RegisterFeatureConfigFlagSpecs(app, FlagContext{FeatureName: "demo"}, &config, []FeatureConfigFlagSpec[configFlagTestConfig]{
		{
			Name:        "name",
			Help:        "Demo name",
			Default:     "default",
			Placeholder: "example",
			Bind: func(flag *kingpin.FlagClause, config *configFlagTestConfig) {
				flag.StringVar(&config.Name)
			},
		},
		{
			Name: "target",
			Help: "Demo target",
			Bind: func(flag *kingpin.FlagClause, config *configFlagTestConfig) {
				flag.StringsVar(&config.Targets)
			},
		},
	})

	if _, err := app.Parse([]string{"--demo.name=custom", "--demo.target=node-a", "--demo.target=node-b"}); err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if config.Name != "custom" {
		t.Fatalf("Name = %q, want custom", config.Name)
	}
	if want := []string{"node-a", "node-b"}; !reflect.DeepEqual(config.Targets, want) {
		t.Fatalf("Targets = %v, want %v", config.Targets, want)
	}
}

func TestNewSnapshotExtensionFeatureDelegatesStableContract(t *testing.T) {
	t.Parallel()

	configFile := filepath.Join(t.TempDir(), "feature.yml")
	if err := os.WriteFile(configFile, []byte("name: from-file\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	feature := NewSnapshotExtensionFeature[extensionTestConfig, extensionTestSnapshot](
		SpecOptions{
			FeatureName:            "demo",
			DefaultRefreshInterval: 15 * time.Second,
		},
		SnapshotFeatureExtension[extensionTestConfig, extensionTestSnapshot]{
			DefaultRefreshInterval: time.Minute,
			DefaultConfigFunc: func() extensionTestConfig {
				return extensionTestConfig{Name: "default"}
			},
			ConfigFileFunc: func(config *extensionTestConfig) *string {
				return &config.ConfigFile
			},
			RegisterFlagsFunc: func(app *kingpin.Application, ctx FlagContext, config *extensionTestConfig) {
				app.Flag(ctx.FeatureName+".name", "test name").Default(config.Name).StringVar(&config.Name)
			},
			ResolveConfigFunc: func(featureName string, config extensionTestConfig) (extensionTestConfig, string, bool, error) {
				var fileConfig extensionTestFileConfig
				path, loaded, err := LoadFeatureConfigFile(featureName, config.ConfigFile, &fileConfig)
				if err != nil || !loaded {
					return config, path, loaded, err
				}
				if config.Name == "default" {
					config.Name = fileConfig.Name
				}
				return config, path, true, nil
			},
			RuntimeConfigFunc: func(_ RuntimeConfigContext[extensionTestConfig], config extensionTestConfig) []any {
				return []any{"name", config.Name}
			},
			SmokeFunc: func(ctx SmokeContext[extensionTestConfig]) SmokeSpec {
				return SmokeSpec{
					ServerArgs:  []string{"--" + ctx.FeatureName + ".name=" + ctx.Config.Name},
					WantMetrics: []string{ctx.FeatureName + "_metric 1"},
				}
			},
		},
	)

	app := kingpin.New("test", "")
	app.Terminate(func(int) {})
	feature.RegisterFlags(app)
	if _, err := app.Parse([]string{"--demo.config-file=" + configFile, "--demo.refresh-interval=20s"}); err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	config := feature.RuntimeConfig()
	if got := configValue(t, config, "refresh_interval"); got != 20*time.Second {
		t.Fatalf("refresh_interval = %v, want 20s", got)
	}
	if got := configValue(t, config, "config_file"); got != configFile {
		t.Fatalf("config_file = %v, want %s", got, configFile)
	}
	if got := configValue(t, config, "config_file_loaded"); got != true {
		t.Fatalf("config_file_loaded = %v, want true", got)
	}
	if got := configValue(t, config, "name"); got != "from-file" {
		t.Fatalf("name = %v, want from-file", got)
	}

	wantSmoke := SmokeSpec{
		ServerArgs:  []string{"--demo.name=default"},
		WantMetrics: []string{"demo_metric 1"},
	}
	if got := feature.SmokeSpec(); !reflect.DeepEqual(got, wantSmoke) {
		t.Fatalf("SmokeSpec() = %+v, want %+v", got, wantSmoke)
	}
}

func TestNewSnapshotExtensionFeatureSpecUsesEngineHooks(t *testing.T) {
	t.Parallel()

	defaultEngine := extensionTestEngine{snapshot: extensionTestSnapshot{Value: 1}}
	extension := SnapshotFeatureExtension[extensionTestConfig, extensionTestSnapshot]{
		DefaultSnapshotEngine: defaultEngine,
		NewSnapshotEngineFunc: func(ctx CollectorContext[extensionTestConfig]) (SnapshotEngine[extensionTestSnapshot], error) {
			if ctx.FeatureName != "demo" {
				t.Fatalf("FeatureName = %q, want demo", ctx.FeatureName)
			}
			return extensionTestEngine{snapshot: extensionTestSnapshot{Value: 2}}, nil
		},
	}

	if got := snapshotExtensionDefaultSnapshotter(extension).Snapshot(context.Background(), time.Unix(1700000000, 0)).Value; got != 1 {
		t.Fatalf("DefaultSnapshotter snapshot value = %v, want 1", got)
	}
	newSnapshotter := snapshotExtensionNewSnapshotter(extension)
	snapshotter, err := newSnapshotter(CollectorContext[extensionTestConfig]{FeatureName: "demo"})
	if err != nil {
		t.Fatalf("NewSnapshotterFunc() error = %v", err)
	}
	if got := snapshotter.Snapshot(context.Background(), time.Unix(1700000000, 0)).Value; got != 2 {
		t.Fatalf("NewSnapshotterFunc snapshot value = %v, want 2", got)
	}
}

func TestNewSnapshotExtensionFeatureSpecPrefersExplicitSnapshotterHooks(t *testing.T) {
	t.Parallel()

	extension := SnapshotFeatureExtension[extensionTestConfig, extensionTestSnapshot]{
		DefaultSnapshotEngine: extensionTestEngine{snapshot: extensionTestSnapshot{Value: 1}},
		NewSnapshotEngineFunc: func(CollectorContext[extensionTestConfig]) (SnapshotEngine[extensionTestSnapshot], error) {
			t.Fatal("NewSnapshotEngineFunc was called, want explicit NewSnapshotterFunc")
			return nil, nil
		},
		DefaultSnapshotter: extensionTestEngine{snapshot: extensionTestSnapshot{Value: 3}},
		NewSnapshotterFunc: func(CollectorContext[extensionTestConfig]) (framework.Snapshotter[extensionTestSnapshot], error) {
			return extensionTestEngine{snapshot: extensionTestSnapshot{Value: 4}}, nil
		},
	}

	if got := snapshotExtensionDefaultSnapshotter(extension).Snapshot(context.Background(), time.Unix(1700000000, 0)).Value; got != 3 {
		t.Fatalf("DefaultSnapshotter snapshot value = %v, want 3", got)
	}
	newSnapshotter := snapshotExtensionNewSnapshotter(extension)
	snapshotter, err := newSnapshotter(CollectorContext[extensionTestConfig]{FeatureName: "demo"})
	if err != nil {
		t.Fatalf("NewSnapshotterFunc() error = %v", err)
	}
	if got := snapshotter.Snapshot(context.Background(), time.Unix(1700000000, 0)).Value; got != 4 {
		t.Fatalf("NewSnapshotterFunc snapshot value = %v, want 4", got)
	}
}

func TestLoadFeatureConfigFileOptionalMissingAndStrictParsing(t *testing.T) {
	t.Parallel()

	missing := filepath.Join(t.TempDir(), "missing.yml")
	if path, loaded, err := LoadFeatureConfigFile("demo", missing, &extensionTestFileConfig{}); err == nil || loaded || path != missing {
		t.Fatalf("LoadFeatureConfigFile(explicit missing) = path %q loaded %v err %v, want error", path, loaded, err)
	}
	if path, loaded, err := LoadFeatureConfigFile("demo", "", &extensionTestFileConfig{}); err != nil || loaded || path != DefaultFeatureConfigFile("demo") {
		t.Fatalf("LoadFeatureConfigFile(default missing) = path %q loaded %v err %v, want optional miss", path, loaded, err)
	}

	configFile := filepath.Join(t.TempDir(), "bad.yml")
	if err := os.WriteFile(configFile, []byte("unexpected: value\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if _, _, err := LoadFeatureConfigFile("demo", configFile, &extensionTestFileConfig{}); err == nil {
		t.Fatal("LoadFeatureConfigFile(strict yaml) error = nil, want error")
	}
}

func TestResolveFeatureConfigUsesDefaultConfigFileWithoutResolver(t *testing.T) {
	t.Parallel()

	config := extensionTestConfig{}
	resolved, configFile, loaded, err := ResolveFeatureConfig("demo", config, func(config *extensionTestConfig) *string {
		return &config.ConfigFile
	}, nil)
	if err != nil {
		t.Fatalf("ResolveFeatureConfig() error = %v", err)
	}
	if !reflect.DeepEqual(resolved, config) {
		t.Fatalf("resolved = %+v, want %+v", resolved, config)
	}
	if configFile != DefaultFeatureConfigFile("demo") {
		t.Fatalf("configFile = %q, want %q", configFile, DefaultFeatureConfigFile("demo"))
	}
	if loaded {
		t.Fatal("loaded = true, want false")
	}
}

package featurekit

import "github.com/alecthomas/kingpin/v2"

type FeatureConfigFlagSpec[C any] struct {
	Name        string
	Help        string
	Default     string
	Placeholder string
	Bind        func(*kingpin.FlagClause, *C)
}

func RegisterFeatureConfigFlagSpecs[C any](app *kingpin.Application, ctx FlagContext, config *C, specs []FeatureConfigFlagSpec[C]) {
	for _, spec := range specs {
		flag := app.Flag(ctx.FeatureName+"."+spec.Name, spec.Help)
		if spec.Placeholder != "" {
			flag = flag.PlaceHolder(spec.Placeholder)
		}
		if spec.Default != "" {
			flag = flag.Default(spec.Default)
		}
		spec.Bind(flag, config)
	}
}

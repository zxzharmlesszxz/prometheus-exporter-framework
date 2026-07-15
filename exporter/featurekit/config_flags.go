package featurekit

import (
	"fmt"

	"github.com/alecthomas/kingpin/v2"
)

// FeatureConfigFlagSpec describes one feature-owned CLI flag.
//
// MarkSet is called only when the flag was explicitly provided on the command
// line. Use it when config-file merge logic must distinguish an omitted flag
// from a flag set to the same value as its default.
type FeatureConfigFlagSpec[C any] struct {
	Name        string
	Help        string
	Default     string
	Placeholder string
	MarkSet     func(*C)
	Bind        func(*kingpin.FlagClause, *C)
}

func RegisterFeatureConfigFlagSpecs[C any](app *kingpin.Application, ctx FlagContext, config *C, specs []FeatureConfigFlagSpec[C]) {
	for _, spec := range specs {
		if spec.Bind == nil {
			panic(fmt.Sprintf("feature config flag %q is missing Bind", ctx.FeatureName+"."+spec.Name))
		}
		flag := app.Flag(ctx.FeatureName+"."+spec.Name, spec.Help)
		if spec.Placeholder != "" {
			flag = flag.PlaceHolder(spec.Placeholder)
		}
		if spec.Default != "" {
			flag = flag.Default(spec.Default)
		}
		if spec.MarkSet != nil {
			flag = flag.Action(func(*kingpin.ParseContext) error {
				spec.MarkSet(config)
				return nil
			})
		}
		spec.Bind(flag, config)
	}
}

package exporter_test

import (
	"flag"
	"testing"

	"github.com/zxzharmlesszxz/prometheus-exporter-framework/exporter/internal/publicapitest"
)

const modulePath = "github.com/zxzharmlesszxz/prometheus-exporter-framework"

var updatePublicAPI = flag.Bool("update-public-api", false, "update exporter public API golden file")

func TestPublicAPISurface(t *testing.T) {
	publicapitest.Check(t, publicapitest.Options{
		GoldenPath:          "testdata/public_api.txt",
		Update:              *updatePublicAPI,
		UpdateCommand:       "go test ./exporter -update-public-api",
		ReadDirLabel:        "exporter",
		ModulePath:          modulePath,
		AliasInternalPrefix: modulePath + "/exporter/internal/",
	})
}

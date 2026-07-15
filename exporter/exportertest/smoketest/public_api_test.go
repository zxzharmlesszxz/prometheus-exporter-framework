package smoketest

import (
	"flag"
	"testing"

	"github.com/zxzharmlesszxz/prometheus-exporter-framework/exporter/internal/publicapitest"
)

var updatePublicAPI = flag.Bool("update-public-api", false, "update exporter/exportertest/smoketest public API golden file")

func TestPublicAPISurface(t *testing.T) {
	publicapitest.Check(t, publicapitest.Options{
		GoldenPath:    "testdata/public_api.txt",
		Update:        *updatePublicAPI,
		UpdateCommand: "go test ./exporter/exportertest/smoketest -update-public-api",
		ReadDirLabel:  "exporter/exportertest/smoketest",
	})
}

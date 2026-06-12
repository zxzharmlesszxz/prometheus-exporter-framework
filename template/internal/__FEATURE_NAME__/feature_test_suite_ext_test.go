package __FEATURE_NAME__

import (
	"fmt"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"__GO_MODULE__/internal/__FEATURE_NAME__check"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/zxzharmlesszxz/prometheus-exporter-framework/exporter/exportertest/featuretest"
)

func TestFeatureContract(t *testing.T) {
	suite := NewFeatureTestSuite(NewFeatureTestSpec())
	RegisterFeatureTests(suite)
	suite.RunTests(t)
}

func NewFeatureTestSpec() FeatureTestSpec {
	return FeatureTestSpec{
		SuccessfulSnapshot: func(at time.Time) Snapshot {
			return Snapshot{
				__FEATURE_NAME__: __FEATURE_NAME__check.Snapshot{
					AttemptTime: at,
					Success:     true,
					Value:       1,
				},
			}
		},
		FailedSnapshot: func(at time.Time, err error) Snapshot {
			return Snapshot{
				__FEATURE_NAME__: __FEATURE_NAME__check.Snapshot{
					AttemptTime: at,
					Success:     false,
					Err:         err,
				},
			}
		},
		CheckDefaultSnapshotter: true,
	}
}

func RegisterFeatureTests(suite *FeatureTestSuite) {
	suite.Register("collector_exports_snapshot", func(t *testing.T) {
		testCollectorExportsSnapshot(t, suite)
	})
	suite.Register("smoke_spec_includes_skeleton_metric", func(t *testing.T) {
		testSmokeSpecIncludesSkeletonMetric(t, suite)
	})
}

func testCollectorExportsSnapshot(t *testing.T, suite *FeatureTestSuite) {
	now := time.Unix(1700000000, 0)
	collector := suite.NewCollectorWithNow(
		testFeatureName,
		testMetricNamespace,
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		suite.NewFakeSnapshotter(Snapshot{
			__FEATURE_NAME__: __FEATURE_NAME__check.Snapshot{
				AttemptTime: now,
				Success:     true,
				Value:       42,
			},
		}),
		testRefreshInterval,
		func() time.Time { return now },
	)

	expected := fmt.Sprintf(`
# HELP %[1]s Example %[5]s metric emitted by the generated exporter skeleton
# TYPE %[1]s gauge
%[1]s 42
# HELP %[2]s Whether the last %[5]s data collection succeeded
# TYPE %[2]s gauge
%[2]s 1
# HELP %[3]s Unix timestamp of the last %[5]s data collection attempt
# TYPE %[3]s gauge
%[3]s 1.7e+09
# HELP %[4]s Unix timestamp of the last successful %[5]s data collection
# TYPE %[4]s gauge
%[4]s 1.7e+09
`, suite.MetricName(testFeatureName, "", metricExampleValue), testLastSuccess, testLastTimestamp, testLastSuccessfulTS, testFeatureName)

	if err := testutil.CollectAndCompare(collector, strings.NewReader(expected),
		suite.MetricName(testFeatureName, "", metricExampleValue),
		testLastSuccess,
		testLastTimestamp,
		testLastSuccessfulTS,
	); err != nil {
		t.Fatalf("CollectAndCompare() error = %v", err)
	}
}

func testSmokeSpecIncludesSkeletonMetric(t *testing.T, suite *FeatureTestSuite) {
	spec := suite.NewNamedFeature().SmokeSpec()
	want := suite.MetricName(testFeatureName, "", metricExampleValue) + " 1"
	if !featuretest.HasString(spec.WantMetrics, want) {
		t.Fatalf("SmokeSpec().WantMetrics = %v, want %q", spec.WantMetrics, want)
	}
}

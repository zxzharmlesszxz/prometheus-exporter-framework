package featurekit

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
)

func TestFeatureMetricName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		spec FeatureMetricSpec
		want string
	}{
		{
			name: "feature scope",
			spec: FeatureMetricSpec{Scope: MetricScopeFeature, Name: "_value"},
			want: "feature_value",
		},
		{
			name: "namespace scope",
			spec: FeatureMetricSpec{Scope: MetricScopeNamespace, Name: "_value"},
			want: "namespace_value",
		},
		{
			name: "absolute scope",
			spec: FeatureMetricSpec{Scope: MetricScopeAbsolute, Name: "absolute_value"},
			want: "absolute_value",
		},
		{
			name: "unknown scope fallback",
			spec: FeatureMetricSpec{Scope: MetricScope(99), Name: "fallback_value"},
			want: "fallback_value",
		},
	}
	for _, tt := range tests {
		if got := tt.spec.MetricName("feature", "namespace"); got != tt.want {
			t.Fatalf("%s: MetricName() = %q, want %q", tt.name, got, tt.want)
		}
	}

	specs := []FeatureMetricSpec{
		{ID: "known", Scope: MetricScopeFeature, Name: "_known"},
	}
	if got := FeatureMetricName("feature", "namespace", "known", specs); got != "feature_known" {
		t.Fatalf("FeatureMetricName(known) = %q, want feature_known", got)
	}
	if got := FeatureMetricName("feature", "namespace", "missing", specs); got != "missing" {
		t.Fatalf("FeatureMetricName(missing) = %q, want missing", got)
	}
}

func TestLoadFeatureMetricDescriptorsPreservesSpecOrder(t *testing.T) {
	t.Parallel()

	specs := []FeatureMetricSpec{
		{ID: "first", Scope: MetricScopeFeature, Name: "_first", Help: "First metric"},
		{ID: "second", Scope: MetricScopeNamespace, Name: "_second", Help: "Second metric"},
	}
	descriptors := LoadFeatureMetricDescriptors("feature", "namespace", specs)
	ch := make(chan *prometheus.Desc, len(specs))
	descriptors.Describe(ch)
	close(ch)

	got := make([]*prometheus.Desc, 0, len(specs))
	for desc := range ch {
		got = append(got, desc)
	}
	want := []*prometheus.Desc{descriptors.Get("first"), descriptors.Get("second")}
	if len(got) != len(want) {
		t.Fatalf("Describe() emitted %d descriptors, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("Describe()[%d] = %v, want %v", i, got[i], want[i])
		}
	}
}

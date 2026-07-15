// Package featuretest provides a shared test suite for scaffold-generated
// featurekit snapshot exporters.
//
// It verifies the common feature contract: default runtime config, config-file
// loading, collector registration, framework collection metrics, smoke-test
// metadata, metric naming, and snapshot refresh behavior. Concrete exporters
// provide domain-specific config, snapshot, metrics, and tests through
// FeatureTestSpec and FeatureTestSuite.
package featuretest

package exporter

import snapshotpkg "github.com/zxzharmlesszxz/prometheus-exporter-framework/exporter/internal/snapshot"

const DefaultSnapshotRefreshInterval = snapshotpkg.DefaultSnapshotRefreshInterval

type Snapshotter[T any] = snapshotpkg.Snapshotter[T]
type SnapshotStatus = snapshotpkg.SnapshotStatus
type SnapshotCollectorOptions[T any] = snapshotpkg.SnapshotCollectorOptions[T]
type SnapshotCollector[T any] = snapshotpkg.SnapshotCollector[T]

func NewSnapshotCollector[T any](options SnapshotCollectorOptions[T]) *SnapshotCollector[T] {
	return snapshotpkg.NewSnapshotCollector(options)
}

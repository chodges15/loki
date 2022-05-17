package series

import (
	"context"
	"testing"
	"time"

	"github.com/go-kit/log"
	"github.com/grafana/dskit/flagext"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"
	"github.com/weaveworks/common/user"

	"github.com/grafana/loki/pkg/storage/chunk/cache"
	"github.com/grafana/loki/pkg/storage/config"
	"github.com/grafana/loki/pkg/storage/stores/series/index"
	"github.com/grafana/loki/pkg/validation"
)

type mockStore struct {
	index.Client
	queries []index.Query
	results index.ReadBatchResult
}

func (m *mockStore) QueryPages(ctx context.Context, queries []index.Query, callback index.QueryPagesCallback) error {
	for _, query := range queries {
		callback(query, m.results)
	}
	m.queries = append(m.queries, queries...)
	return nil
}

func defaultLimits() (*validation.Overrides, error) {
	var defaults validation.Limits
	flagext.DefaultValues(&defaults)
	defaults.CardinalityLimit = 5
	return validation.NewOverrides(defaults, nil)
}

func BenchmarkIndexStore(b *testing.B) {
	store := &mockStore{
		results: index.ReadBatch{
			Entries: []index.CacheEntry{{
				Column: []byte("foo"),
				Value:  []byte("bar"),
			}},
		},
	}
	limits, err := defaultLimits()
	require.NoError(b, err)
	logger := log.NewNopLogger()
	cache := cache.NewFifoCache("test", cache.FifoCacheConfig{MaxSizeItems: 10, TTL: 10 * time.Second}, nil, logger)
	client := index.NewCachingIndexClient(store, cache, 100*time.Millisecond, limits, logger, false)

	schemaCfg := config.SchemaConfig{
		Configs: []config.PeriodConfig{
			{From: config.DayTime{Time: model.Now().Add(-24 * time.Hour)}, Schema: "v12", RowShards: 16},
		},
	}
	schema, err := index.CreateSchema(schemaCfg.Configs[0])
	require.NoError(b, err)
	idxStore := NewIndexStore(schemaCfg, schema, client, nil, 10)

	userID := "fake"
	from := model.Now().Add(-1 * time.Hour)
	through := model.Now()
	ctx := user.InjectOrgID(context.Background(), userID)

	b.ResetTimer()
	b.ReportAllocs()
	for x := 0; x < b.N; x++ {
		res, err := idxStore.LabelValuesForMetricName(ctx, userID, from, through, "logs", "foo")
		b.Log(res, err)
	}
}

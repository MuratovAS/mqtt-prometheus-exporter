package prometheus

import (
	"fmt"
	"strings"
	"time"

	gocache "github.com/patrickmn/go-cache"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/torilabs/mqtt-prometheus-exporter/config"
	"github.com/torilabs/mqtt-prometheus-exporter/log"
	"go.uber.org/zap"
)

// Collector is an extended interface of prometheus.Collector.
type Collector interface {
	prometheus.Collector
	Observe(metric config.Metric, topic string, v float64, expiration time.Duration, labelValues ...string)
}

type memoryCachedCollector struct {
	cache        *gocache.Cache
	descriptions []*prometheus.Desc
	metrics      []config.Metric
}

type collectorEntry struct {
	m  prometheus.Metric
	ts time.Time
}

// NewCollector constructs collector for incoming prometheus metrics.
func NewCollector(expiration time.Duration, possibleMetrics []config.Metric) Collector {
	if len(possibleMetrics) == 0 {
		log.Logger.Warn("No metrics are configured.")
	}
	var descs []*prometheus.Desc
	for _, m := range possibleMetrics {
		descs = append(descs, m.PrometheusDescription())
	}
	return &memoryCachedCollector{
		cache:        gocache.New(expiration, expiration*10),
		descriptions: descs,
		metrics:      possibleMetrics,
	}
}

func (c *memoryCachedCollector) Observe(metric config.Metric, topic string, v float64, expiration time.Duration, labelValues ...string) {
	m, err := prometheus.NewConstMetric(metric.PrometheusDescription(), metric.PrometheusValueType(), v, labelValues...)
	if err != nil {
		log.Logger.With(zap.Error(err)).Warnf("Creation of prometheus metric failed.")
		return
	}
	key := fmt.Sprintf("%s|%s", metric.PrometheusName, topic)
	c.cache.Set(key, &collectorEntry{m: m, ts: time.Now()}, expiration)
}

func (c *memoryCachedCollector) Describe(ch chan<- *prometheus.Desc) {
	for i := range c.descriptions {
		ch <- c.descriptions[i]
	}
}

func (c *memoryCachedCollector) Collect(mc chan<- prometheus.Metric) {
	log.Logger.Debugf("Collecting. Returned '%d' metrics.", c.cache.ItemCount())
	for key, rawItem := range c.cache.Items() {
		item := rawItem.Object.(*collectorEntry)
		metricName := strings.Split(key, "|")[0]
		if metric, ok := c.getMetricByName(metricName); ok {
			if metric.FakeTS {
				mc <- prometheus.NewMetricWithTimestamp(time.Now(), item.m)
			} else {
				mc <- prometheus.NewMetricWithTimestamp(item.ts, item.m)
			}
		}
	}
}

func (c *memoryCachedCollector) getMetricByName(name string) (config.Metric, bool) {
	for _, metric := range c.metrics {
		if metric.PrometheusName == name {
			return metric, true
		}
	}
	return config.Metric{}, false
}

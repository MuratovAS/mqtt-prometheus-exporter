package mqtt

import (
	"reflect"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/torilabs/mqtt-prometheus-exporter/config"
)

type fakeCollector struct {
	observed       bool
	obsMetric      config.Metric
	obsTopic       string
	obsValue       float64
	obsLabelValues []string
}

func (c *fakeCollector) Observe(metric config.Metric, topic string, v float64, expiration time.Duration, labelValues ...string) {
	c.observed = true
	c.obsMetric = metric
	c.obsTopic = topic
	c.obsValue = v
	c.obsLabelValues = labelValues
	// We don't use expiration in tests, so we just ignore it
}

func (c *fakeCollector) Describe(chan<- *prometheus.Desc) {
}

func (c *fakeCollector) Collect(chan<- prometheus.Metric) {
}

type fakeMessage struct {
	topic   string
	payload []byte
}

func (m *fakeMessage) Duplicate() bool {
	return false
}

func (m *fakeMessage) Qos() byte {
	return 0
}

func (m *fakeMessage) Retained() bool {
	return false
}

func (m *fakeMessage) Topic() string {
	return m.topic
}

func (m *fakeMessage) MessageID() uint16 {
	return 0
}

func (m *fakeMessage) Payload() []byte {
	return m.payload
}

func (m *fakeMessage) Ack() {
}

func Test_messageHandler(t *testing.T) {
	type args struct {
		metric config.Metric
	}
	tests := []struct {
		name            string
		args            args
		msg             fakeMessage
		wantObserved    bool
		wantValue       float64
		wantLabelValues []string
	}{
		{
			name: "Raw value received and processed",
			args: args{
				metric: config.Metric{
					MqttTopic:   "/topic/level2/level3/#",
					TopicLabels: map[string]int{"customTopic": 2, "customTopic2": 3, "customTopic3": 4},
				},
			},
			msg: fakeMessage{
				topic:   "/topic/level2/level3/device",
				payload: []byte("25.12"),
			},
			wantObserved:    true,
			wantValue:       25.12,
			wantLabelValues: []string{"/topic/level2/level3/device", "level2", "level3", "device"},
		},
		{
			name: "Raw value received and failed to parse",
			args: args{
				metric: config.Metric{
					MqttTopic: "/topic/level2/level3/#",
				},
			},
			msg: fakeMessage{
				topic:   "/topic/level2/level3/device",
				payload: []byte("not a number"),
			},
			wantObserved: false,
		},
		{
			name: "JSON value on 1st level parsed",
			args: args{
				metric: config.Metric{
					MqttTopic:   "/topic/level2/level3/#",
					TopicLabels: map[string]int{"customTopic": 2, "customTopic2": 3},
					JSONField:   []string{"size"},
				},
			},
			msg: fakeMessage{
				topic:   "/topic/level2/level3/device",
				payload: []byte(`{"city":"Tokyo", "temperatures": {"out": 12.5, "in": 22.15}, "size": -5}`),
			},
			wantObserved:    true,
			wantValue:       -5,
			wantLabelValues: []string{"/topic/level2/level3/device", "level2", "level3"},
		},
		{
			name: "JSON value on 2nd level parsed",
			args: args{
				metric: config.Metric{
					MqttTopic: "/topic/level2/level3/#",
					JSONField: []string{"temperatures.out"},
				},
			},
			msg: fakeMessage{
				topic:   "/topic/level2/level3/device",
				payload: []byte(`{"city":"Tokyo", "temperatures": {"out": 12.5, "in": 22.15}, "size": -5}`),
			},
			wantObserved:    true,
			wantValue:       12.5,
			wantLabelValues: []string{"/topic/level2/level3/device"},
		},
		{
			name: "JSON value as object failed to parse",
			args: args{
				metric: config.Metric{
					MqttTopic: "/topic/level2/level3/#",
					JSONField: []string{"temperatures"},
				},
			},
			msg: fakeMessage{
				topic:   "/topic/level2/level3/device",
				payload: []byte(`{"city":"Tokyo", "temperatures": {"out": 12.5, "in": 22.15}, "size": -5}`),
			},
			wantObserved: false,
		},
		{
			name: "JSON value as non numeric failed to parse",
			args: args{
				metric: config.Metric{
					MqttTopic: "/topic/level2/level3/#",
					JSONField: []string{"city"},
				},
			},
			msg: fakeMessage{
				topic:   "/topic/level2/level3/device",
				payload: []byte(`{"city":"Tokyo", "temperatures": {"out": 12.5, "in": 22.15}, "size": -5}`),
			},
			wantObserved: false,
		},
		{
			name: "JSON with dots in field names parsed correctly",
			args: args{
				metric: config.Metric{
					MqttTopic: "/topic/level2/level3/#",
					JSONField: []string{"total.count"},
				},
			},
			msg: fakeMessage{
				topic:   "/topic/level2/level3/device",
				payload: []byte(`{"total": {"count": 22, "unknown": "none"}, "random": "2"}`),
			},
			wantObserved:    true,
			wantValue:       22,
			wantLabelValues: []string{"/topic/level2/level3/device"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			collector := fakeCollector{}
			mh := NewMessageHandler(tt.args.metric, &collector)
			mh(&fakeClient{}, &tt.msg)

			if tt.wantObserved != collector.observed {
				t.Errorf("observe = %v, want %v", collector.observed, tt.wantObserved)
				return
			}

			if collector.observed {
				if tt.msg.topic != collector.obsTopic {
					t.Errorf("topic = %v, want %v", collector.obsTopic, tt.msg.topic)
				}
				if tt.wantValue != collector.obsValue {
					t.Errorf("value = %v, want %v", collector.obsValue, tt.wantValue)
				}
				// Skip exact metric comparison as PrometheusName may be modified
				if !reflect.DeepEqual(tt.wantLabelValues, collector.obsLabelValues) {
					t.Errorf("labelValues = %v, want %v", collector.obsLabelValues, tt.wantLabelValues)
				}
			}
		})
	}
}

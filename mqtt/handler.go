package mqtt

import (
	"encoding/json"
	"fmt"
	"strconv"

	pahomqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/torilabs/mqtt-prometheus-exporter/config"
	"github.com/torilabs/mqtt-prometheus-exporter/log"
	"github.com/torilabs/mqtt-prometheus-exporter/prometheus"
	"go.uber.org/zap"
)

type messageHandler struct {
	metric    config.Metric
	collector prometheus.Collector
}

// NewMessageHandler constructs handler for single metric.
func NewMessageHandler(metric config.Metric, collector prometheus.Collector) pahomqtt.MessageHandler {
	// Save base metric name
	metric.BaseName = metric.PrometheusName
	mh := &messageHandler{
		metric:    metric,
		collector: collector,
	}
	if len(metric.JSONField) > 0 {
		return mh.getJSONMessageHandler()
	}
	return mh.getMessageHandler()
}

func (h *messageHandler) getMessageHandler() pahomqtt.MessageHandler {
	return func(_ pahomqtt.Client, msg pahomqtt.Message) {
		strValue := string(msg.Payload())
		log.Logger.Debugf("Received MQTT msg '%s' from '%s' topic. Listener for: '%s'.", strValue, msg.Topic(), h.metric.MqttTopic)
		floatValue, err := strconv.ParseFloat(strValue, 64)
		if err != nil {
			log.Logger.With(zap.Error(err)).Warnf("Got data with unexpected value '%s' and failed to parse to float.", strValue)
			return
		}
		labelValues := []string{msg.Topic()}
		for _, tl := range h.metric.TopicLabels.KeysInOrder() {
			labelValues = append(labelValues, getTopicPart(msg.Topic(), h.metric.TopicLabels[tl]))
		}
		h.collector.Observe(h.metric, msg.Topic(), floatValue, h.metric.Expiration, labelValues...)
	}
}

func (h *messageHandler) getJSONMessageHandler() pahomqtt.MessageHandler {
	return func(_ pahomqtt.Client, msg pahomqtt.Message) {
		log.Logger.Debugf("Received MQTT msg '%s' from '%s' topic. Listener for: '%s'.", msg.Payload(), msg.Topic(), h.metric.MqttTopic)

		jsonMap := make(map[string]interface{})
		if err := json.Unmarshal(msg.Payload(), &jsonMap); err != nil {
			log.Logger.With(zap.Error(err)).Warnf("Got an invalid JSON value '%s' and failed to unmarshal.", msg.Payload())
			return
		}

		labelValues := []string{msg.Topic()}
		for _, tl := range h.metric.TopicLabels.KeysInOrder() {
			labelValues = append(labelValues, getTopicPart(msg.Topic(), h.metric.TopicLabels[tl]))
		}

		for _, field := range h.metric.JSONField {
			if value, ok := findInJSON(jsonMap, field); ok {
				floatValue, err := strconv.ParseFloat(fmt.Sprintf("%v", value), 64)
				if err != nil {
					log.Logger.With(zap.Error(err)).Warnf("Got data with unexpected value '%s' for field '%s' and failed to parse to float.", value, field)
					continue
				}
				// Generate field-specific metric name
				origName := h.metric.PrometheusName
				h.metric.PrometheusName = fmt.Sprintf("%s_%s", h.metric.BaseName, field)
				log.Logger.Debugf("Observing metric %s for field %s with value %f", 
					h.metric.PrometheusName, field, floatValue)
				h.collector.Observe(h.metric, msg.Topic(), floatValue, h.metric.Expiration, labelValues...)
				// Restore original name
				h.metric.PrometheusName = origName
			}
		}
	}
}

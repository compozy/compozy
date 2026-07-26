package network

import (
	"sort"
	"strings"
	"sync"

	"github.com/compozy/agh/internal/store"
)

const (
	statsChannelKey = "channel"
	statsStateKey   = "state"
	statsSurfaceKey = "surface"
)

// KindMetric is the runtime per-kind network activity snapshot surfaced by
// status APIs.
type KindMetric struct {
	Kind      Kind
	Sent      int64
	Received  int64
	Rejected  int64
	Delivered int64
}

// MetricSample is one low-cardinality runtime network metric sample.
type MetricSample struct {
	Name   string
	Labels map[string]string
	Value  int64
}

type statsSnapshot struct {
	MessagesSent         int64
	MessagesReceived     int64
	MessagesRejected     int64
	MessagesDelivered    int64
	WorkflowTaggedEvents int64
	HandoffTaggedEvents  int64
	KindMetrics          []KindMetric
	OpenThreads          int64
	OpenDirectRooms      int64
	OpenWorkItems        int64
	ConversationMessages int64
	WorkTransitions      int64
	DirectResolves       int64
	Metrics              []MetricSample
}

type runtimeStats struct {
	mu sync.Mutex

	messagesSent          int64
	messagesReceived      int64
	messagesRejected      int64
	messagesDelivered     int64
	workflowTaggedEvents  int64
	handoffTaggedEvents   int64
	kindMetrics           map[Kind]*KindMetric
	openThreads           int64
	openDirectRooms       int64
	openWorkItems         int64
	conversationMessages  int64
	workTransitions       int64
	directResolves        int64
	messageMetrics        map[messageMetricKey]int64
	conversationMetrics   map[surfaceMetricKey]int64
	threadOpenMetrics     map[channelMetricKey]int64
	directOpenMetrics     map[channelMetricKey]int64
	workOpenMetrics       map[surfaceMetricKey]int64
	workTransitionMetrics map[workTransitionMetricKey]int64
	openWorkMetrics       map[surfaceMetricKey]int64
	directResolveMetrics  map[directResolveMetricKey]int64
}

func newRuntimeStats() *runtimeStats {
	return &runtimeStats{
		kindMetrics:           make(map[Kind]*KindMetric),
		messageMetrics:        make(map[messageMetricKey]int64),
		conversationMetrics:   make(map[surfaceMetricKey]int64),
		threadOpenMetrics:     make(map[channelMetricKey]int64),
		directOpenMetrics:     make(map[channelMetricKey]int64),
		workOpenMetrics:       make(map[surfaceMetricKey]int64),
		workTransitionMetrics: make(map[workTransitionMetricKey]int64),
		openWorkMetrics:       make(map[surfaceMetricKey]int64),
		directResolveMetrics:  make(map[directResolveMetricKey]int64),
	}
}

func (s *runtimeStats) recordSent(envelope Envelope) {
	s.record(envelope, func(metric *KindMetric) {
		metric.Sent++
		s.messagesSent++
		s.messageMetrics[messageMetricKeyFromEnvelope(envelope, AuditDirectionSent, "published")]++
	})
}

func (s *runtimeStats) recordReceived(envelope Envelope) {
	s.record(envelope, func(metric *KindMetric) {
		metric.Received++
		s.messagesReceived++
		s.messageMetrics[messageMetricKeyFromEnvelope(envelope, AuditDirectionReceived, "accepted")]++
	})
}

func (s *runtimeStats) recordRejected(envelope Envelope) {
	s.record(envelope, func(metric *KindMetric) {
		metric.Rejected++
		s.messagesRejected++
		s.messageMetrics[messageMetricKeyFromEnvelope(envelope, "unknown", "rejected")]++
	})
}

func (s *runtimeStats) recordDelivered(envelope Envelope) {
	s.record(envelope, func(metric *KindMetric) {
		metric.Delivered++
		s.messagesDelivered++
		s.messageMetrics[messageMetricKeyFromEnvelope(envelope, AuditDirectionReceived, "delivered")]++
	})
}

func (s *runtimeStats) recordConversationWrite(
	entry store.NetworkConversationMessage,
	result store.NetworkConversationWriteResult,
) {
	if s == nil || result.Duplicate {
		return
	}

	key := surfaceMetricKeyFromMessage(entry)
	s.mu.Lock()
	defer s.mu.Unlock()

	s.conversationMessages++
	s.conversationMetrics[key]++
	s.messageMetrics[messageMetricKeyFromMessage(entry, "persisted")]++
	if strings.TrimSpace(entry.Surface) == store.NetworkSurfaceDirect {
		resolveKey := directResolveMetricKey{channel: key.channel, result: directResolveResult(result)}
		s.directResolves++
		s.directResolveMetrics[resolveKey]++
	}
	if result.ConversationOpened {
		switch strings.TrimSpace(entry.Surface) {
		case store.NetworkSurfaceThread:
			s.openThreads++
			s.threadOpenMetrics[channelMetricKey{channel: key.channel}]++
		case store.NetworkSurfaceDirect:
			s.openDirectRooms++
			s.directOpenMetrics[channelMetricKey{channel: key.channel}]++
		}
	}
	if result.WorkOpened {
		s.workOpenMetrics[key]++
		if !networkWorkStateIsTerminal(result.WorkState) {
			s.openWorkItems++
			s.openWorkMetrics[key]++
		}
	}
	if result.WorkTransitioned {
		transitionKey := workTransitionMetricKey{
			channel: key.channel,
			surface: key.surface,
			state:   strings.TrimSpace(result.WorkState),
		}
		s.workTransitions++
		s.workTransitionMetrics[transitionKey]++
		if networkWorkStateIsTerminal(result.WorkState) && s.openWorkMetrics[key] > 0 {
			s.openWorkItems--
			s.openWorkMetrics[key]--
		}
	}
}

func (s *runtimeStats) snapshot() statsSnapshot {
	if s == nil {
		return statsSnapshot{}
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	metrics := make([]KindMetric, 0, len(s.kindMetrics))
	for _, metric := range s.kindMetrics {
		if metric == nil {
			continue
		}
		if metric.Sent == 0 && metric.Received == 0 && metric.Rejected == 0 && metric.Delivered == 0 {
			continue
		}
		metrics = append(metrics, *metric)
	}
	sort.Slice(metrics, func(i, j int) bool {
		return string(metrics[i].Kind) < string(metrics[j].Kind)
	})
	metricSamples := s.metricSamplesLocked()

	return statsSnapshot{
		MessagesSent:         s.messagesSent,
		MessagesReceived:     s.messagesReceived,
		MessagesRejected:     s.messagesRejected,
		MessagesDelivered:    s.messagesDelivered,
		WorkflowTaggedEvents: s.workflowTaggedEvents,
		HandoffTaggedEvents:  s.handoffTaggedEvents,
		KindMetrics:          metrics,
		OpenThreads:          s.openThreads,
		OpenDirectRooms:      s.openDirectRooms,
		OpenWorkItems:        s.openWorkItems,
		ConversationMessages: s.conversationMessages,
		WorkTransitions:      s.workTransitions,
		DirectResolves:       s.directResolves,
		Metrics:              metricSamples,
	}
}

func (s *runtimeStats) record(envelope Envelope, update func(*KindMetric)) {
	if s == nil || update == nil {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	metric := s.kindMetricLocked(envelope.Kind)
	update(metric)

	if hasWorkflowID(envelope.Ext) {
		s.workflowTaggedEvents++
	}
	if hasHandoffVersion(envelope.Ext) {
		s.handoffTaggedEvents++
	}
}

func (s *runtimeStats) kindMetricLocked(kind Kind) *KindMetric {
	metric, ok := s.kindMetrics[kind]
	if ok && metric != nil {
		return metric
	}
	metric = &KindMetric{Kind: kind}
	s.kindMetrics[kind] = metric
	return metric
}

type messageMetricKey struct {
	channel   string
	surface   string
	kind      string
	direction string
	result    string
}

type surfaceMetricKey struct {
	channel string
	surface string
}

type channelMetricKey struct {
	channel string
}

type workTransitionMetricKey struct {
	channel string
	surface string
	state   string
}

type directResolveMetricKey struct {
	channel string
	result  string
}

func (s *runtimeStats) metricSamplesLocked() []MetricSample {
	samples := make([]MetricSample, 0)
	for key, value := range s.messageMetrics {
		samples = append(samples, MetricSample{
			Name: "network_messages_total",
			Labels: map[string]string{
				statsChannelKey: key.channel,
				statsSurfaceKey: key.surface,
				"kind":          key.kind,
				"direction":     key.direction,
				"result":        key.result,
			},
			Value: value,
		})
	}
	for key, value := range s.conversationMetrics {
		samples = append(samples, surfaceMetricSample("network_conversation_messages_total", key, value))
	}
	for key, value := range s.threadOpenMetrics {
		samples = append(samples, MetricSample{
			Name:   "network_threads_open_total",
			Labels: map[string]string{statsChannelKey: key.channel},
			Value:  value,
		})
	}
	for key, value := range s.directOpenMetrics {
		samples = append(samples, MetricSample{
			Name:   "network_direct_rooms_open_total",
			Labels: map[string]string{statsChannelKey: key.channel},
			Value:  value,
		})
	}
	for key, value := range s.workOpenMetrics {
		samples = append(samples, surfaceMetricSample("network_work_open_total", key, value))
	}
	for key, value := range s.workTransitionMetrics {
		samples = append(samples, MetricSample{
			Name: "network_work_transitions_total",
			Labels: map[string]string{
				statsChannelKey: key.channel,
				statsSurfaceKey: key.surface,
				statsStateKey:   key.state,
			},
			Value: value,
		})
	}
	for key, value := range s.openWorkMetrics {
		samples = append(samples, surfaceMetricSample("network_open_work_items", key, value))
	}
	for key, value := range s.directResolveMetrics {
		samples = append(samples, MetricSample{
			Name: "network_direct_resolve_total",
			Labels: map[string]string{
				statsChannelKey: key.channel,
				"result":        key.result,
			},
			Value: value,
		})
	}
	sortMetricSamples(samples)
	return samples
}

func surfaceMetricSample(name string, key surfaceMetricKey, value int64) MetricSample {
	return MetricSample{
		Name: name,
		Labels: map[string]string{
			statsChannelKey: key.channel,
			statsSurfaceKey: key.surface,
		},
		Value: value,
	}
}

func messageMetricKeyFromEnvelope(envelope Envelope, direction string, result string) messageMetricKey {
	return messageMetricKey{
		channel:   strings.TrimSpace(envelope.Channel),
		surface:   surfaceLabel(envelope.Surface),
		kind:      strings.TrimSpace(string(envelope.Kind)),
		direction: strings.TrimSpace(direction),
		result:    strings.TrimSpace(result),
	}
}

func messageMetricKeyFromMessage(entry store.NetworkConversationMessage, result string) messageMetricKey {
	return messageMetricKey{
		channel:   strings.TrimSpace(entry.Channel),
		surface:   strings.TrimSpace(entry.Surface),
		kind:      strings.TrimSpace(entry.Kind),
		direction: strings.TrimSpace(entry.Direction),
		result:    strings.TrimSpace(result),
	}
}

func surfaceMetricKeyFromMessage(entry store.NetworkConversationMessage) surfaceMetricKey {
	return surfaceMetricKey{
		channel: strings.TrimSpace(entry.Channel),
		surface: strings.TrimSpace(entry.Surface),
	}
}

func directResolveResult(result store.NetworkConversationWriteResult) string {
	if result.ConversationOpened {
		return "opened"
	}
	return "existing"
}

func surfaceLabel(surface *Surface) string {
	if surface == nil {
		return ""
	}
	return strings.TrimSpace(string(*surface))
}

func networkWorkStateIsTerminal(state string) bool {
	switch strings.TrimSpace(state) {
	case store.NetworkWorkStateCompleted, store.NetworkWorkStateFailed, store.NetworkWorkStateCanceled:
		return true
	default:
		return false
	}
}

func sortMetricSamples(samples []MetricSample) {
	sort.Slice(samples, func(i, j int) bool {
		left := metricSortKey(samples[i])
		right := metricSortKey(samples[j])
		return left < right
	})
}

func metricSortKey(sample MetricSample) string {
	labels := make([]string, 0, len(sample.Labels))
	for key, value := range sample.Labels {
		labels = append(labels, key+"="+value)
	}
	sort.Strings(labels)
	return sample.Name + "|" + strings.Join(labels, "|")
}

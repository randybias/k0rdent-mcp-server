package events

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	eventsv1 "k8s.io/api/events/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
)

// Provider wraps Kubernetes Event access, transparently falling back between events.k8s.io/v1 and core/v1.
type Provider struct {
	client           kubernetes.Interface
	useEventsV1      bool
	serverTimeSource func() time.Time
}

// ListOptions define filters accepted by the events list tool.
type ListOptions struct {
	Types          []string
	ForKind        string
	ForName        string
	SinceSeconds   *int64
	Limit          *int
	FieldSelectors []string // reserved for future use; currently unused
}

// WatchOptions define filters for event subscriptions.
type WatchOptions struct {
	Types    []string
	ForKind  string
	ForName  string
	Selector string
}

// Delta describes a change observed from a watch stream.
type Delta struct {
	Type  watch.EventType `json:"type"`
	Event Event           `json:"event"`
}

// Event represents a Kubernetes Event in a transport-friendly structure.
type Event struct {
	Name                string           `json:"name"`
	Namespace           string           `json:"namespace"`
	Reason              string           `json:"reason"`
	Message             string           `json:"message"`
	Type                string           `json:"type"`
	ReportingController string           `json:"reportingController,omitempty"`
	ReportingInstance   string           `json:"reportingInstance,omitempty"`
	Count               int32            `json:"count,omitempty"`
	FirstTimestamp      *time.Time       `json:"firstTimestamp,omitempty"`
	LastTimestamp       *time.Time       `json:"lastTimestamp,omitempty"`
	EventTime           *time.Time       `json:"eventTime,omitempty"`
	InvolvedObject      InvolvedObject   `json:"involvedObject"`
	Series              *EventSeriesInfo `json:"series,omitempty"`
}

// InvolvedObject captures minimal reference data about the object related to the event.
type InvolvedObject struct {
	Namespace string `json:"namespace,omitempty"`
	Name      string `json:"name,omitempty"`
	Kind      string `json:"kind,omitempty"`
	UID       string `json:"uid,omitempty"`
}

// EventSeriesInfo mirrors the Kubernetes series struct while using Go types that JSON encode cleanly.
type EventSeriesInfo struct {
	LastObservedTime *time.Time `json:"lastObservedTime,omitempty"`
	Count            int32      `json:"count,omitempty"`
}

// NewProvider discovers available APIs and returns an Event provider.
func NewProvider(ctx context.Context, client kubernetes.Interface) (*Provider, error) {
	if client == nil {
		return nil, errors.New("kubernetes client is nil")
	}

	useV1, err := supportsEventsV1(ctx, client)
	if err != nil {
		// If discovery fails for non-permission reasons, surface the error.
		// For permission failures, fall back silently to core/v1.
		if !apierrors.IsForbidden(err) && !apierrors.IsNotFound(err) {
			return nil, fmt.Errorf("discover events.k8s.io/v1 support: %w", err)
		}
		useV1 = false
	}

	return &Provider{
		client:           client,
		useEventsV1:      useV1,
		serverTimeSource: time.Now,
	}, nil
}

// List returns events within the namespace that satisfy the provided filters.
func (p *Provider) List(ctx context.Context, namespace string, opts ListOptions) ([]Event, error) {
	if namespace == "" {
		return nil, errors.New("namespace is required")
	}

	var events []Event
	var err error
	if p.useEventsV1 {
		events, err = p.listEventsV1(ctx, namespace, opts)
	} else {
		events, err = p.listCoreEvents(ctx, namespace, opts)
	}
	if err != nil {
		return nil, err
	}

	filtered := p.filterEvents(events, opts)
	return p.enforceLimit(filtered, opts.Limit), nil
}

// WatchNamespace streams event deltas for the namespace until the context is cancelled.
func (p *Provider) WatchNamespace(ctx context.Context, namespace string, opts WatchOptions) (<-chan Delta, <-chan error, error) {
	if namespace == "" {
		return nil, nil, errors.New("namespace is required")
	}

	listOpts := metav1.ListOptions{
		AllowWatchBookmarks: true,
	}

	watcher, err := p.startWatch(ctx, namespace, listOpts)
	if err != nil {
		return nil, nil, err
	}

	eventCh := make(chan Delta)
	errCh := make(chan error, 1)

	go func() {
		defer close(eventCh)
		defer close(errCh)
		defer watcher.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case event, ok := <-watcher.ResultChan():
				if !ok {
					errCh <- errors.New("event watch channel closed")
					return
				}
				if event.Type == watch.Bookmark {
					continue
				}

				converted, convErr := p.convertWatchEvent(event)
				if convErr != nil {
					errCh <- convErr
					continue
				}

				if !matchesWatchFilters(converted.Event, opts) {
					continue
				}

				select {
				case <-ctx.Done():
					return
				case eventCh <- converted:
				}
			}
		}
	}()

	return eventCh, errCh, nil
}

func (p *Provider) listEventsV1(ctx context.Context, namespace string, opts ListOptions) ([]Event, error) {
	result, err := p.client.EventsV1().Events(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		// Fallback to core/v1 if the API becomes unavailable.
		if apierrors.IsNotFound(err) || apierrors.IsForbidden(err) {
			p.useEventsV1 = false
			return p.listCoreEvents(ctx, namespace, opts)
		}
		return nil, fmt.Errorf("list events.v1: %w", err)
	}

	events := make([]Event, 0, len(result.Items))
	for _, item := range result.Items {
		events = append(events, convertEventV1(&item))
	}
	return events, nil
}

func (p *Provider) listCoreEvents(ctx context.Context, namespace string, opts ListOptions) ([]Event, error) {
	result, err := p.client.CoreV1().Events(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list core events: %w", err)
	}

	events := make([]Event, 0, len(result.Items))
	for _, item := range result.Items {
		events = append(events, convertCoreEvent(&item))
	}
	return events, nil
}

func (p *Provider) filterEvents(events []Event, opts ListOptions) []Event {
	if len(events) == 0 {
		return events
	}

	var filtered []Event
	typeSet := make(map[string]struct{}, len(opts.Types))
	for _, t := range opts.Types {
		typeSet[strings.ToLower(t)] = struct{}{}
	}

	var sinceThreshold time.Time
	if opts.SinceSeconds != nil && *opts.SinceSeconds > 0 {
		sinceThreshold = p.serverTimeSource().Add(-time.Duration(*opts.SinceSeconds) * time.Second)
	}

	for _, event := range events {
		if len(typeSet) > 0 {
			if _, ok := typeSet[strings.ToLower(event.Type)]; !ok {
				continue
			}
		}

		if opts.ForKind != "" && !strings.EqualFold(event.InvolvedObject.Kind, opts.ForKind) {
			continue
		}
		if opts.ForName != "" && !strings.EqualFold(event.InvolvedObject.Name, opts.ForName) {
			continue
		}

		if !sinceThreshold.IsZero() && !eventOccurredAfter(event, sinceThreshold) {
			continue
		}

		filtered = append(filtered, event)
	}

	return filtered
}

func (p *Provider) enforceLimit(events []Event, limit *int) []Event {
	if limit == nil || *limit <= 0 {
		return events
	}
	if len(events) <= *limit {
		return events
	}
	return events[:*limit]
}

func eventOccurredAfter(event Event, since time.Time) bool {
	if event.EventTime != nil && event.EventTime.After(since) {
		return true
	}
	if event.LastTimestamp != nil && event.LastTimestamp.After(since) {
		return true
	}
	if event.Series != nil && event.Series.LastObservedTime != nil && event.Series.LastObservedTime.After(since) {
		return true
	}
	return false
}

func (p *Provider) startWatch(ctx context.Context, namespace string, opts metav1.ListOptions) (watch.Interface, error) {
	if p.useEventsV1 {
		watcher, err := p.client.EventsV1().Events(namespace).Watch(ctx, opts)
		if err == nil {
			return watcher, nil
		}
		if !apierrors.IsNotFound(err) && !apierrors.IsForbidden(err) {
			return nil, fmt.Errorf("watch events.v1: %w", err)
		}
		// fall back to core/v1
		p.useEventsV1 = false
	}
	watcher, err := p.client.CoreV1().Events(namespace).Watch(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("watch core events: %w", err)
	}
	return watcher, nil
}

func (p *Provider) convertWatchEvent(event watch.Event) (Delta, error) {
	var converted Event
	switch obj := event.Object.(type) {
	case *eventsv1.Event:
		converted = convertEventV1(obj)
	case *corev1.Event:
		converted = convertCoreEvent(obj)
	case *metav1.PartialObjectMetadata:
		// Partial metadata is received on deletion events when using watches with metadata-only.
		converted = Event{
			Name:      obj.GetName(),
			Namespace: obj.GetNamespace(),
			InvolvedObject: InvolvedObject{
				Namespace: obj.GetNamespace(),
				Name:      obj.GetName(),
			},
		}
	default:
		// Ignore unexpected object types to keep stream resilient.
		return Delta{}, fmt.Errorf("unexpected event object type %T", obj)
	}

	return Delta{
		Type:  event.Type,
		Event: converted,
	}, nil
}

func supportsEventsV1(ctx context.Context, client kubernetes.Interface) (bool, error) {
	_, err := client.Discovery().ServerResourcesForGroupVersion(eventsv1.SchemeGroupVersion.String())
	if err != nil {
		if apierrors.IsNotFound(err) || apierrors.IsForbidden(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func convertEventV1(event *eventsv1.Event) Event {
	if event == nil {
		return Event{}
	}

	e := Event{
		Name:                event.Name,
		Namespace:           event.Namespace,
		Reason:              event.Reason,
		Message:             event.Note,
		Type:                event.Type,
		ReportingController: event.ReportingController,
		ReportingInstance:   event.ReportingInstance,
		Count:               event.DeprecatedCount,
		InvolvedObject: InvolvedObject{
			Namespace: event.Namespace,
			Name:      event.Regarding.Name,
			Kind:      event.Regarding.Kind,
			UID:       string(event.Regarding.UID),
		},
	}

	if event.Series != nil {
		e.Series = &EventSeriesInfo{
			Count: event.Series.Count,
		}
		if !event.Series.LastObservedTime.IsZero() {
			last := event.Series.LastObservedTime.Time
			e.Series.LastObservedTime = &last
			e.LastTimestamp = &last
		}
	}

	if !event.EventTime.IsZero() {
		t := event.EventTime.Time
		e.EventTime = &t
	}
	if !event.DeprecatedFirstTimestamp.IsZero() {
		ts := event.DeprecatedFirstTimestamp.Time
		e.FirstTimestamp = &ts
	}
	if !event.DeprecatedLastTimestamp.IsZero() {
		ts := event.DeprecatedLastTimestamp.Time
		e.LastTimestamp = &ts
	}

	return e
}

func convertCoreEvent(event *corev1.Event) Event {
	if event == nil {
		return Event{}
	}

	e := Event{
		Name:      event.Name,
		Namespace: event.Namespace,
		Reason:    event.Reason,
		Message:   event.Message,
		Type:      event.Type,
		Count:     event.Count,
		InvolvedObject: InvolvedObject{
			Namespace: event.InvolvedObject.Namespace,
			Name:      event.InvolvedObject.Name,
			Kind:      event.InvolvedObject.Kind,
			UID:       string(event.InvolvedObject.UID),
		},
	}

	if !event.FirstTimestamp.IsZero() {
		ts := event.FirstTimestamp.Time
		e.FirstTimestamp = &ts
	}
	if !event.LastTimestamp.IsZero() {
		ts := event.LastTimestamp.Time
		e.LastTimestamp = &ts
	}
	if !event.EventTime.IsZero() {
		ts := event.EventTime.Time
		e.EventTime = &ts
	}

	if event.Series != nil {
		e.Series = &EventSeriesInfo{
			Count: event.Series.Count,
		}
		if !event.Series.LastObservedTime.IsZero() {
			ts := event.Series.LastObservedTime.Time
			e.Series.LastObservedTime = &ts
			e.LastTimestamp = &ts
		}
	}

	return e
}

func matchesWatchFilters(event Event, opts WatchOptions) bool {
	if len(opts.Types) > 0 {
		match := false
		for _, t := range opts.Types {
			if strings.EqualFold(t, event.Type) {
				match = true
				break
			}
		}
		if !match {
			return false
		}
	}

	if opts.ForKind != "" && !strings.EqualFold(event.InvolvedObject.Kind, opts.ForKind) {
		return false
	}
	if opts.ForName != "" && !strings.EqualFold(event.InvolvedObject.Name, opts.ForName) {
		return false
	}
	return true
}

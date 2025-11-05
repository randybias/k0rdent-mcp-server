package logs

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// Provider exposes helpers for retrieving pod logs.
type Provider struct {
	client kubernetes.Interface
}

// Options describe the adjustable parameters for log retrieval.
type Options struct {
	Container    string
	TailLines    *int64
	SinceSeconds *int64
	Previous     bool
}

// StreamOptions describes the configuration for a log stream.
type StreamOptions struct {
	Options
}

// NewProvider constructs a log provider for the supplied Kubernetes client.
func NewProvider(client kubernetes.Interface) (*Provider, error) {
	if client == nil {
		return nil, errors.New("kubernetes client is nil")
	}
	return &Provider{client: client}, nil
}

// Get fetches pod logs using the provided options.
func (p *Provider) Get(ctx context.Context, namespace, pod string, opts Options) (string, error) {
	if err := validatePodRef(namespace, pod); err != nil {
		return "", err
	}

	container, err := p.resolveContainer(ctx, namespace, pod, opts.Container)
	if err != nil {
		return "", err
	}

	req := p.client.CoreV1().Pods(namespace).GetLogs(pod, buildLogOptions(container, opts, false))
	stream, err := req.Stream(ctx)
	if err != nil {
		return "", fmt.Errorf("open log stream: %w", err)
	}
	defer stream.Close()

	data, err := io.ReadAll(stream)
	if err != nil {
		return "", fmt.Errorf("read log stream: %w", err)
	}
	return string(data), nil
}

// Stream tails pod logs and returns channels for log lines and terminal errors.
func (p *Provider) Stream(ctx context.Context, namespace, pod string, opts StreamOptions) (<-chan string, <-chan error, error) {
	if err := validatePodRef(namespace, pod); err != nil {
		return nil, nil, err
	}

	container, err := p.resolveContainer(ctx, namespace, pod, opts.Container)
	if err != nil {
		return nil, nil, err
	}

	req := p.client.CoreV1().Pods(namespace).GetLogs(pod, buildLogOptions(container, opts.Options, true))
	stream, err := req.Stream(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("open log stream: %w", err)
	}

	lineCh := make(chan string)
	errCh := make(chan error, 1)

	go func() {
		defer close(lineCh)
		defer close(errCh)
		defer stream.Close()

		scanner := bufio.NewScanner(stream)
		// Allow log lines up to 1MB.
		buf := make([]byte, 64*1024)
		scanner.Buffer(buf, 1024*1024)

		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			if !scanner.Scan() {
				if err := scanner.Err(); err != nil && !errors.Is(err, context.Canceled) {
					errCh <- err
				}
				return
			}

			select {
			case <-ctx.Done():
				return
			case lineCh <- scanner.Text():
			}
		}
	}()

	return lineCh, errCh, nil
}

func (p *Provider) resolveContainer(ctx context.Context, namespace, pod, container string) (string, error) {
	if container != "" {
		return container, nil
	}

	podObj, err := p.client.CoreV1().Pods(namespace).Get(ctx, pod, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return "", fmt.Errorf("pod %s/%s not found", namespace, pod)
		}
		return "", fmt.Errorf("get pod: %w", err)
	}

	containers := aggregateContainers(podObj)
	if len(containers) == 0 {
		return "", fmt.Errorf("pod %s/%s has no containers", namespace, pod)
	}
	if len(containers) > 1 {
		names := make([]string, 0, len(containers))
		for _, c := range containers {
			names = append(names, c.Name)
		}
		return "", fmt.Errorf("pod %s/%s has multiple containers (%s); container parameter is required", namespace, pod, strings.Join(names, ", "))
	}
	return containers[0].Name, nil
}

func aggregateContainers(pod *corev1.Pod) []corev1.Container {
	if pod == nil {
		return nil
	}
	containers := make([]corev1.Container, 0, len(pod.Spec.InitContainers)+len(pod.Spec.Containers)+len(pod.Spec.EphemeralContainers))
	containers = append(containers, pod.Spec.InitContainers...)
	containers = append(containers, pod.Spec.Containers...)
	for _, c := range pod.Spec.EphemeralContainers {
		containers = append(containers, corev1.Container{Name: c.Name})
	}
	return containers
}

func buildLogOptions(container string, opts Options, follow bool) *corev1.PodLogOptions {
	logOpts := &corev1.PodLogOptions{
		Container:  container,
		Follow:     follow,
		Previous:   opts.Previous,
		Timestamps: false,
	}
	if opts.TailLines != nil {
		logOpts.TailLines = opts.TailLines
	}
	if opts.SinceSeconds != nil {
		logOpts.SinceSeconds = opts.SinceSeconds
	}
	return logOpts
}

func validatePodRef(namespace, pod string) error {
	if namespace == "" {
		return errors.New("namespace is required")
	}
	if pod == "" {
		return errors.New("pod name is required")
	}
	return nil
}

// ToPointer converts an int value to *int64 suitable for TailLines or SinceSeconds.
func ToPointer[T ~int | ~int64](v T) *int64 {
	value := int64(v)
	return &value
}

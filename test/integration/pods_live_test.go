//go:build live

package integration

import (
    "context"
    "encoding/json"
    "testing"
    "time"

    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    "k8s.io/client-go/kubernetes"
    "k8s.io/client-go/tools/clientcmd"
)

type podLogsResult struct {
    Logs string `json:"logs"`
}

func TestPodLogsLive(t *testing.T) {
    kubeconfig, _ := requireLiveEnv(t)
    client := newLiveClient(t)

    restCfg, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
    if err != nil {
        t.Fatalf("load kubeconfig: %v", err)
    }

    kubeClient, err := kubernetes.NewForConfig(restCfg)
    if err != nil {
        t.Fatalf("create kubernetes client: %v", err)
    }

    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    podList, err := kubeClient.CoreV1().Pods("").List(ctx, metav1.ListOptions{})
    if err != nil {
        t.Fatalf("list pods: %v", err)
    }
    if len(podList.Items) == 0 {
        t.Fatalf("no pods found in cluster")
    }

    var namespace, podName, container string
    for _, pod := range podList.Items {
        if len(pod.Spec.Containers) == 0 {
            continue
        }
        namespace = pod.Namespace
        podName = pod.Name
        container = pod.Spec.Containers[0].Name
        break
    }
    if namespace == "" {
        t.Fatalf("found pods with no containers; cannot run log test")
    }

    raw := client.CallTool(t, "k0rdent.mgmt.podLogs.get", map[string]any{
        "namespace": namespace,
        "pod":       podName,
        "container": container,
        "follow":    false,
    })

    var result podLogsResult
    if err := json.Unmarshal(raw, &result); err != nil {
        t.Fatalf("decode pod logs result: %v", err)
    }
    if result.Logs == "" {
        t.Fatalf("expected logs for pod %s/%s", namespace, podName)
    }
}

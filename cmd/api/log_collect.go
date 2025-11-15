package main

import (
	"bufio"
	"context"
	"encoding/json"
	"sync"

	"fl-observer/internal/kubeclient"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Envelope struct {
	Event   string          `json:"event"`
	Payload json.RawMessage `json:"payload"`
}

func (app *application) runLogCollector(ctx context.Context, out chan<- Envelope) {
	// get pod names from namespace and labelSelector
	pods, err := app.kube.Pods(namespace).List(context.Background(), metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		app.logger.Errorf("Failed to get demo pod from cluster %v", kubeclient.KubeClient.Config.Host)

		return
	}

	var wg sync.WaitGroup

	// running streamPodLogs
	for _, pod := range pods.Items {
		podName := pod.Name
		app.logger.Info(podName)

		wg.Add(1)
		go func(podName string) {
			defer wg.Done()
			app.streamPodLogs(ctx, podName)
		}(podName)
	}

	// waiting for ctx.Done()
	go func() {
		<-ctx.Done()
		app.logger.Info("log collector context canceled, waiting for streams to end")
		wg.Wait()
		close(out)
	}()
}

func (app *application) streamPodLogs(ctx context.Context, podName string) {
	PodLogsConnection := app.kube.Pods(namespace).GetLogs(podName, &corev1.PodLogOptions{})

	LogStream, err := PodLogsConnection.Stream(ctx)
	if err != nil {
		app.logger.Infow("logstream start error", "error", err)

		return
	}
	defer LogStream.Close()

	reader := bufio.NewScanner(LogStream)

	var line string

	for {
		for reader.Scan() {
			select {
			case <-ctx.Done():
				break
			default:
				line = reader.Text()
				app.logger.Infof("Pod: %v line: %v\n", podName, line)
			}
		}

		if reader.Err() != nil {
			app.logger.Errorf("error in logs inpput for pod: %v due to: %v\n", podName, reader.Err())

			break
		}
	}
}

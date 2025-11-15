package main

import (
	"context"
	"expvar"
	"runtime"

	"fl-observer/internal/env"
	"fl-observer/internal/kubeclient"

	"go.uber.org/zap"
)

const version = "1.1.0"

const (
	namespace     = "flwr"
	labelSelector = "name=superexec"
)

func main() {
	cfg := config{
		addr:       env.GetString("ADDR", ":8080"),
		apiURL:     env.GetString("EXTERNAL_URL", "localhost:8080"),
		env:        env.GetString("ENV", "development"),
		kubeconfig: env.GetString("KUBECONFIG", ""),
	}

	logger := zap.Must(zap.NewProduction()).Sugar()
	defer logger.Sync()

	kube := kubeclient.New(cfg.kubeconfig)
	if kube == nil {
		logger.Fatalw("failed to init kube client", "kubeconfig", cfg.kubeconfig)
	}

	app := &application{
		config: cfg,
		logger: logger,
		kube:   kube,
	}
	expvar.NewString("version").Set(version)
	expvar.Publish("goroutines", expvar.Func(func() any {
		return runtime.NumGoroutine()
	}))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	events := make(chan Envelope, 1000)
	// run kube observer
	app.runLogCollector(ctx, events)

	mux := app.mount()
	logger.Fatal(app.run(mux))
}

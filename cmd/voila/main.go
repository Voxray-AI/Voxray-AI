// Package main is the Voila Go server entrypoint.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"voila-go/pkg/config"
	"voila-go/pkg/logger"
	"voila-go/pkg/pipeline"
	"voila-go/pkg/processors"
	"voila-go/pkg/processors/aggregator"
	"voila-go/pkg/processors/echo"
	proclog "voila-go/pkg/processors/logger"
	"voila-go/pkg/processors/voice"
	"voila-go/pkg/services"
	"voila-go/pkg/transport/websocket"
)

func main() {
	configPath := flag.String("config", "config.json", "Path to config file")
	initCmd := flag.Bool("init", false, "Scaffold config.json and dirs (plugins, logs) then exit")
	flag.Parse()
	// Support subcommand: Voila init [config path]
	if len(flag.Args()) >= 1 && flag.Arg(0) == "init" {
		path := *configPath
		if len(flag.Args()) >= 2 {
			path = flag.Arg(1)
		}
		if err := runInit(path, true); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		return
	}
	if *initCmd {
		if err := runInit(*configPath, true); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	if err := run(*configPath); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run(configPath string) error {
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	// Register built-in processors for plugin registry (dynamic loading from config)
	pipeline.RegisterProcessor("echo", func(name string) processors.Processor { return echo.New(name) })
	pipeline.RegisterProcessor("logger", func(name string) processors.Processor { return proclog.New(name) })
	pipeline.RegisterProcessor("aggregator", func(name string) processors.Processor { return aggregator.New(name, "", 0) })

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		logger.Info("shutting down")
		cancel()
	}()

	port := cfg.Port
	if port == 0 {
		port = 8080
	}

	// Prefer voice pipeline if config has provider/model (LLM/STT/TTS); otherwise echo pipeline from plugins
	useVoice := cfg.Provider != "" && cfg.Model != ""

	wsServer := &websocket.Server{
		Host: cfg.Host,
		Port: port,
		OnConn: func(ctx context.Context, tr *websocket.ConnTransport) {
			var pl *pipeline.Pipeline
			if useVoice {
				llm, sttSvc, ttsSvc := services.NewServicesFromConfig(cfg)
				pl = pipeline.New()
				pl.Add(voice.NewSTTProcessor("stt", sttSvc, 16000, 1))
				pl.Add(voice.NewLLMProcessor("llm", llm))
				pl.Add(voice.NewTTSProcessor("tts", ttsSvc, 24000))
				pl.Add(pipeline.NewSink("sink", tr.Output()))
			} else {
				pl = pipeline.New()
				if err := pl.AddFromConfig(cfg); err != nil {
					// Fallback to echo if plugin names unknown
					pl.Add(echo.New("echo"))
				}
				pl.Add(pipeline.NewSink("sink", tr.Output()))
			}
			runner := pipeline.NewRunner(pl, tr)
			go func() {
				_ = runner.Run(ctx)
			}()
		},
	}

	logger.Info("starting WebSocket server on %s:%d", cfg.Host, port)
	return wsServer.ListenAndServe(ctx)
}

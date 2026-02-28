// Package main is the Voila Go server entrypoint.
//
// @title Voila API
// @version 1.0
// @description Voila voice pipeline server: WebSocket and WebRTC transport endpoints.
// @host localhost:8080
// @BasePath /
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"voila-go/pkg/audio/turn"
	"voila-go/pkg/audio/vad"
	"voila-go/pkg/config"
	"voila-go/pkg/frames"
	"voila-go/pkg/logger"
	"voila-go/pkg/observers"
	"voila-go/pkg/pipeline"
	"voila-go/pkg/processors"
	"voila-go/pkg/processors/aggregator"
	"voila-go/pkg/processors/echo"
	proclog "voila-go/pkg/processors/logger"
	"voila-go/pkg/processors/voice"
	"voila-go/pkg/server"
	"voila-go/pkg/services"
	"voila-go/pkg/transport"
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

	// Prefer voice pipeline if config has provider/model (LLM/STT/TTS); otherwise echo pipeline from plugins.
	useVoice := cfg.Provider != "" && cfg.Model != ""

	buildPipeline := func(tr pipeline.Transport) *pipeline.Pipeline {
		var pl *pipeline.Pipeline
		if useVoice {
			llm, sttSvc, ttsSvc := services.NewServicesFromConfig(cfg)
			pl = pipeline.New()
			// Composite observer for turn tracking, latency, and metrics.
			metrics := observers.NewMetrics()
			turnObs := observers.NewTurnTrackingObserver()
			latencyObs := observers.NewUserBotLatencyObserver()
			withMetrics := observers.NewObserverWithMetrics(
				observers.NewCompositeObserver(turnObs, latencyObs),
				metrics,
			)
			wrap := func(proc processors.Processor) processors.Processor {
				return observers.WrapWithObserver(proc, withMetrics)
			}
			if cfg.TurnEnabled() {
				// Construct VAD based on config.
				var vadDetector vad.Detector
				switch cfg.VADBackendOrDefault() {
				case "silero":
					if a, err := vad.NewSileroAnalyzer(vad.Params{
						Confidence: cfg.VADParams().Confidence,
						StartSecs:  cfg.VADParams().StartSecs,
						StopSecs:   cfg.VADParams().StopSecs,
						MinVolume:  cfg.VADParams().MinVolume,
					}, 16000); err == nil {
						vadDetector = &vad.AnalyzerDetector{Analyzer: a}
					} else {
						logger.Info("silero VAD unavailable (%v), falling back to energy VAD", err)
					}
				}
				if vadDetector == nil {
					ed := vad.NewEnergyDetector()
					if cfg.VadThreshold > 0 {
						ed.Threshold = cfg.VadThreshold
					}
					vadDetector = ed
				}
				turnParams := turn.Params{
					StopSecs:        cfg.TurnStopSecs,
					PreSpeechMs:     cfg.TurnPreSpeechMs,
					MaxDurationSecs: cfg.TurnMaxDurationSecs,
				}
				if turnParams.StopSecs <= 0 {
					turnParams.StopSecs = turn.DefaultStopSecs
				}
				if turnParams.PreSpeechMs <= 0 {
					turnParams.PreSpeechMs = turn.DefaultPreSpeechMs
				}
				if turnParams.MaxDurationSecs <= 0 {
					turnParams.MaxDurationSecs = turn.DefaultMaxDurationSecs
				}
				analyzer := turn.NewSilenceTurnAnalyzer(turnParams)
				if cfg.VADStartSecs != 0 {
					analyzer.UpdateVADStartSecs(cfg.VADStartSecs)
				}
				// Derive user turn/idle timeouts, falling back to sensible defaults.
				userTurnStopTimeout := cfg.UserTurnStopTimeoutSecs
				if userTurnStopTimeout <= 0 {
					if cfg.TurnStopSecs > 0 {
						userTurnStopTimeout = cfg.TurnStopSecs
					} else {
						userTurnStopTimeout = 5 // seconds
					}
				}
				userIdleTimeout := cfg.UserIdleTimeoutSecs
				pl.Add(wrap(voice.NewTurnProcessorWithUserTurn(
					"turn",
					vadDetector,
					analyzer,
					16000,
					1,
					cfg.TurnAsync,
					userTurnStopTimeout,
					userIdleTimeout,
				)))
			}
			pl.Add(wrap(voice.NewSTTProcessor("stt", sttSvc, 16000, 1)))
			pl.Add(wrap(voice.NewLLMProcessor("llm", llm)))
			pl.Add(wrap(voice.NewTTSProcessor("tts", ttsSvc, 24000)))
			pl.Add(wrap(pipeline.NewSink("sink", tr.Output())))
			_ = metrics // metrics available for OTELExport or logging if needed
		} else {
			pl = pipeline.New()
			if err := pl.AddFromConfig(cfg); err != nil {
				// Fallback to echo if plugin names unknown
				pl.Add(echo.New("echo"))
			}
			pl.Add(pipeline.NewSink("sink", tr.Output()))
		}
		return pl
	}

	onTransport := func(ctx context.Context, tr transport.Transport) {
		pl := buildPipeline(tr)
		startFrame := frames.NewStartFrame()
		startFrame.AllowInterruptions = cfg.AllowInterruptions
		runner := pipeline.NewRunner(pl, tr, startFrame)
		go func() {
			_ = runner.Run(ctx)
		}()
	}

	return server.StartServers(ctx, cfg, onTransport)
}

// Package server provides transport servers (e.g. WebSocket, SmallWebRTC) for Voila.
package server

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"os"

	"github.com/google/uuid"
	"github.com/swaggo/http-swagger"
	_ "voila-go/docs" // register generated Swagger spec
	"voila-go/pkg/config"
	"voila-go/pkg/logger"
	"voila-go/pkg/runner"
	"voila-go/pkg/transport"
	"voila-go/pkg/runner/daily"
	"voila-go/pkg/transport/smallwebrtc"
	ws "voila-go/pkg/transport/websocket"
)

// webrtcOfferResponse is the JSON body for successful POST /webrtc/offer responses.
type webrtcOfferResponse struct {
	Answer string `json:"answer"`
}

// WebrtcOfferDoc documents the WebRTC offer HTTP endpoint for Swagger.
// POST /webrtc/offer accepts a JSON body with an "offer" (SDP offer string), creates a new WebRTC transport, and returns an SDP answer.
// The endpoint returns 400 for invalid or missing offer, 405 for non-POST, and 503 when the server cannot complete the connection (e.g. Opus encoder unavailable).
// Each request gets its own transport; onTransport is invoked in a new goroutine.
//
// @Summary Submit WebRTC offer
// @Description Accepts a WebRTC SDP offer and returns an SDP answer. Available when transport is smallwebrtc or both.
// @Tags webrtc
// @Accept json
// @Produce json
// @Param body body object true "JSON body with 'offer' (SDP offer string)"
// @Success 200 {object} webrtcOfferResponse
// @Failure 400 {string} string "Invalid offer or handling failed"
// @Failure 405 {string} string "Method not allowed"
// @Router /webrtc/offer [post]
func WebrtcOfferDoc() {}

// registerHandlers registers the web file server (when web/ exists), Swagger, Pipecat-style /start and /sessions when WebRTC is enabled, and the WebRTC /webrtc/offer handler on mux.
func registerHandlers(mux *http.ServeMux, cfg *config.Config, ctx context.Context, onTransport func(context.Context, transport.Transport), sessionStore *runner.SessionStore) {
	// Swagger first
	mux.Handle("/swagger/", httpSwagger.Handler(
		httpSwagger.URL("/swagger/doc.json"),
		httpSwagger.DeepLinking(true),
	))
	mode := cfg.Transport
	if mode == "" {
		mode = "websocket"
	}
	enableWebRTC := mode == "smallwebrtc" || mode == "both"
	runnerMode := enableWebRTC || cfg.RunnerTransport == "daily"
	if runnerMode {
		if sessionStore == nil {
			sessionStore = runner.NewSessionStore()
		}
		registerRunnerWebRTCRoutes(mux, cfg, ctx, onTransport, sessionStore)
	}
	if enableWebRTC {
		mux.HandleFunc("/webrtc/offer", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		defer r.Body.Close()

		var req struct {
			Offer string `json:"offer"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Offer == "" {
			http.Error(w, "invalid offer payload", http.StatusBadRequest)
			return
		}

		tr := smallwebrtc.NewTransport(&smallwebrtc.Config{
			ICEServers: cfg.WebRTCICEServers,
		})
		answer, err := tr.HandleOffer(req.Offer)
		if err != nil {
			logger.Error("smallwebrtc handle offer: %v", err)
			// Return 503 with error message when server cannot send TTS (e.g. Opus encoder unavailable)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(w).Encode(struct {
				Error string `json:"error"`
			}{Error: err.Error()})
			return
		}
		if err := tr.Start(ctx); err != nil {
			logger.Error("smallwebrtc start: %v", err)
			http.Error(w, "failed to start transport", http.StatusInternalServerError)
			return
		}

		if onTransport != nil {
			go onTransport(ctx, tr)
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(struct {
			Answer string `json:"answer"`
		}{
			Answer: answer,
		}); err != nil {
			logger.Error("encode webrtc answer: %v", err)
		}
	})
	}
	// Telephony routes (Pipecat runner: twilio, telnyx, plivo, exotel)
	registerTelephonyRoutes(mux, cfg, ctx, onTransport)

	// Daily routes (Pipecat runner: daily)
	registerDailyRoutes(mux, cfg, ctx, onTransport)

	// Web root last so it doesn't override /start or /sessions (skip when telephony or daily uses /)
	telephonyMode := cfg.RunnerTransport == "twilio" || cfg.RunnerTransport == "telnyx" || cfg.RunnerTransport == "plivo" || cfg.RunnerTransport == "exotel"
	dailyMode := cfg.RunnerTransport == "daily"
	if !telephonyMode && !dailyMode {
		if st, err := os.Stat("web"); err == nil && st.IsDir() {
			mux.Handle("/", http.FileServer(http.Dir("web")))
		}
	}
}

// registerTelephonyRoutes adds POST / (XML webhook) and /telephony/ws when RunnerTransport is a telephony provider.
func registerTelephonyRoutes(mux *http.ServeMux, cfg *config.Config, ctx context.Context, onTransport func(context.Context, transport.Transport)) {
	t := cfg.RunnerTransport
	if t != "twilio" && t != "telnyx" && t != "plivo" && t != "exotel" {
		return
	}
	// POST / returns provider-specific XML that points Stream/Connect to wss://{host}/telephony/ws
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		if r.Method == http.MethodPost {
			host := cfg.ProxyHost
			if host == "" {
				host = r.Host
			}
			scheme := "wss"
			if r.TLS == nil {
				scheme = "ws"
			}
			wsURL := scheme + "://" + host + "/telephony/ws"
			switch t {
			case "twilio":
				w.Header().Set("Content-Type", "application/xml")
				_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?><Response><Connect><Stream url="` + wsURL + `"/></Connect></Response>`))
			case "telnyx":
				w.Header().Set("Content-Type", "application/xml")
				_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?><Response><Connect><Stream url="` + wsURL + `"/></Connect></Response>`))
			case "plivo":
				w.Header().Set("Content-Type", "application/xml")
				_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?><Response><Connect><Stream url="` + wsURL + `"/></Connect></Response>`))
			case "exotel":
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"error":       "Exotel doesn't use POST webhooks",
					"websocket_url": wsURL,
					"note":        "Configure the WebSocket URL above in your Exotel App Bazaar Voicebot Applet",
				})
			}
			return
		}
		if r.Method == http.MethodGet {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "Bot started with " + t})
			return
		}
	})
	// /telephony/ws: WebSocket with provider detection from first message(s)
	mux.HandleFunc("/telephony/ws", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		conn, err := ws.Upgrade(w, r)
		if err != nil {
			logger.Error("telephony ws upgrade: %v", err)
			return
		}
		defer conn.Close()
		var first, second []byte
		_, first, _ = conn.ReadMessage()
		_, second, _ = conn.ReadMessage()
		data, ok := runner.ParseTelephonyMessage(first, second)
		if !ok || data.Provider == "" {
			logger.Error("telephony: could not detect provider from handshake")
			return
		}
		getKey := func(service, envVar string) string { return cfg.GetAPIKey(service, envVar) }
		ser := runner.BuildTelephonySerializer(data, getKey)
		if ser == nil {
			logger.Error("telephony: no serializer for provider %s", data.Provider)
			return
		}
		tr := ws.NewConnTransport(conn, 64, 64, ser)
		if err := tr.Start(ctx); err != nil {
			logger.Error("telephony transport start: %v", err)
			return
		}
		if onTransport != nil {
			go onTransport(ctx, tr)
		}
		<-tr.Done()
	})
}

// registerDailyRoutes adds GET / (create room + redirect) and optionally POST /daily-dialin-webhook when RunnerTransport is "daily".
func registerDailyRoutes(mux *http.ServeMux, cfg *config.Config, ctx context.Context, onTransport func(context.Context, transport.Transport)) {
	if cfg.RunnerTransport != "daily" {
		return
	}
	apiKey := cfg.GetAPIKey("daily_api_key", "DAILY_API_KEY")
	getOpts := func() daily.Options {
		return daily.Options{
			APIKey:               apiKey,
			RoomExpDurationHrs:   2,
			TokenExpDurationHrs:  2,
		}
	}

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		if r.Method == http.MethodGet {
			c, err := daily.Configure(ctx, getOpts())
			if err != nil {
				logger.Error("daily configure: %v", err)
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			http.Redirect(w, r, c.RoomURL, http.StatusFound)
			return
		}
		http.NotFound(w, r)
	})

	if cfg.Dialin {
		mux.HandleFunc("/daily-dialin-webhook", func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
				return
			}
			defer r.Body.Close()
			var payload struct {
				Test       bool   `json:"test"`
				From       string `json:"From"`
				To         string `json:"To"`
				CallID     string `json:"callId"`
				CallDomain string `json:"callDomain"`
			}
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				http.Error(w, "invalid JSON payload", http.StatusBadRequest)
				return
			}
			if payload.Test {
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(map[string]string{"status": "OK"})
				return
			}
			if payload.From == "" || payload.To == "" || payload.CallID == "" || payload.CallDomain == "" {
				http.Error(w, "missing required fields: From, To, callId, callDomain", http.StatusBadRequest)
				return
			}
			opts := getOpts()
			opts.SIPCallerPhone = payload.From
			c, err := daily.Configure(ctx, opts)
			if err != nil {
				logger.Error("daily dial-in configure: %v", err)
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"dailyRoom":  c.RoomURL,
				"dailyToken": c.Token,
				"sessionId":  uuid.New().String(),
			})
		})
	}
}

// registerRunnerWebRTCRoutes adds POST /start and POST/PATCH /sessions/{id}/api/offer (Pipecat Cloud-style).
func registerRunnerWebRTCRoutes(mux *http.ServeMux, cfg *config.Config, ctx context.Context, onTransport func(context.Context, transport.Transport), store *runner.SessionStore) {
	mux.HandleFunc("/start", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		defer r.Body.Close()
		var req struct {
			CreateDailyRoom        bool                   `json:"createDailyRoom"`
			EnableDefaultIceServers bool                  `json:"enableDefaultIceServers"`
			Body                   map[string]interface{} `json:"body"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		// Daily: create room + token and return dailyRoom, dailyToken, sessionId
		if req.CreateDailyRoom {
			opts := daily.Options{
				APIKey:               cfg.GetAPIKey("daily_api_key", "DAILY_API_KEY"),
				RoomExpDurationHrs:   2,
				TokenExpDurationHrs:  2,
			}
			c, err := daily.Configure(ctx, opts)
			if err != nil {
				logger.Error("daily configure: %v", err)
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"dailyRoom":  c.RoomURL,
				"dailyToken": c.Token,
				"sessionId":  uuid.New().String(),
			})
			return
		}
		sessionID := uuid.New().String()
		store.Put(sessionID, &runner.Session{
			Body:                    req.Body,
			EnableDefaultIceServers: req.EnableDefaultIceServers,
		})
		resp := map[string]interface{}{"sessionId": sessionID}
		if req.EnableDefaultIceServers {
			resp["iceConfig"] = map[string]interface{}{
				"iceServers": []map[string]interface{}{
					{"urls": []string{"stun:stun.l.google.com:19302"}},
				},
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})

	mux.HandleFunc("/sessions/", func(w http.ResponseWriter, r *http.Request) {
		// Path: /sessions/{sessionId}/api/offer or /sessions/{sessionId}/...
		path := r.URL.Path
		if len(path) <= len("/sessions/") {
			http.NotFound(w, r)
			return
		}
		rest := path[len("/sessions/"):]
		var sessionID string
		for i, c := range rest {
			if c == '/' {
				sessionID = rest[:i]
				break
			}
		}
		if sessionID == "" {
			sessionID = rest
		}
		sess := store.Get(sessionID)
		if sess == nil {
			http.Error(w, "invalid or not-yet-ready session_id", http.StatusNotFound)
			return
		}
		if r.URL.Path != "/sessions/"+sessionID+"/api/offer" {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, PATCH, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if r.Method == http.MethodPatch {
			// ICE candidates: accept and return 200 (trickle ICE; transport may not support adding candidates later)
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "success"})
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		defer r.Body.Close()
		var offerReq struct {
			SDP         string                 `json:"sdp"`
			Type        string                 `json:"type"`
			PCID        string                 `json:"pc_id"`
			RestartPC   bool                   `json:"restart_pc"`
			RequestData map[string]interface{} `json:"request_data"`
			RequestDataAlt map[string]interface{} `json:"requestData"`
		}
		if err := json.NewDecoder(r.Body).Decode(&offerReq); err != nil || offerReq.SDP == "" {
			http.Error(w, "invalid WebRTC request", http.StatusBadRequest)
			return
		}
		if offerReq.RequestData == nil {
			offerReq.RequestData = offerReq.RequestDataAlt
		}
		if offerReq.RequestData == nil {
			offerReq.RequestData = sess.Body
		}
		tr := smallwebrtc.NewTransport(&smallwebrtc.Config{ICEServers: cfg.WebRTCICEServers})
		answer, err := tr.HandleOffer(offerReq.SDP)
		if err != nil {
			logger.Error("sessions api/offer handle: %v", err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		if err := tr.Start(ctx); err != nil {
			logger.Error("sessions api/offer start: %v", err)
			http.Error(w, "failed to start transport", http.StatusInternalServerError)
			return
		}
		if onTransport != nil {
			go onTransport(ctx, tr)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"answer": answer, "type": "answer"})
	})
}

// StartServers starts the HTTP server that serves the WebSocket endpoint at /ws and, when transport is smallwebrtc or both, the WebRTC signaling endpoint at POST /webrtc/offer.
// The onTransport callback is run in a new goroutine for each new connection.
// If cfg is nil, StartServers returns nil without starting anything.
// The server runs until ctx is canceled.
func StartServers(ctx context.Context, cfg *config.Config, onTransport func(ctx context.Context, tr transport.Transport)) error {
	if cfg == nil {
		return nil
	}

	mode := cfg.Transport
	if mode == "" {
		mode = "websocket"
	}

	port := cfg.Port
	if port == 0 {
		port = 8080
	}
	enableWebRTC := mode == "smallwebrtc" || mode == "both"
	runnerMode := enableWebRTC || cfg.RunnerTransport == "daily"
	var sessionStore *runner.SessionStore
	if runnerMode {
		sessionStore = runner.NewSessionStore()
	}

	server := &ws.Server{
		Host:           cfg.Host,
		Port:           port,
		SessionTimeout: ws.DefaultSessionTimeout,
		OnConn: func(c context.Context, tr *ws.ConnTransport) {
			if onTransport != nil {
				onTransport(c, tr)
			}
		},
		RegisterHandlers: func(mux *http.ServeMux) {
			registerHandlers(mux, cfg, ctx, onTransport, sessionStore)
		},
	}

	logger.Info("starting server on %s:%d (transport=%s)", cfg.Host, port, mode)
	return server.ListenAndServe(ctx)
}

// StartServersWithListener starts the same HTTP stack as StartServers but uses the provided listener (e.g. from net.Listen("tcp", ":0")).
// The caller owns the listener and must close it when done.
// Useful for tests that need a dynamic port.
// If cfg is nil, returns nil without starting.
func StartServersWithListener(ctx context.Context, listener net.Listener, cfg *config.Config, onTransport func(ctx context.Context, tr transport.Transport)) error {
	if cfg == nil {
		return nil
	}
	mode := cfg.Transport
	if mode == "" {
		mode = "websocket"
	}
	enableWebRTC := mode == "smallwebrtc" || mode == "both"
	runnerMode := enableWebRTC || cfg.RunnerTransport == "daily"
	var sessionStore *runner.SessionStore
	if runnerMode {
		sessionStore = runner.NewSessionStore()
	}

	server := &ws.Server{
		Host:           cfg.Host,
		Port:           0,
		SessionTimeout: ws.DefaultSessionTimeout,
		OnConn: func(c context.Context, tr *ws.ConnTransport) {
			if onTransport != nil {
				onTransport(c, tr)
			}
		},
		RegisterHandlers: func(mux *http.ServeMux) {
			registerHandlers(mux, cfg, ctx, onTransport, sessionStore)
		},
	}

	return server.ServeWithListener(ctx, listener)
}

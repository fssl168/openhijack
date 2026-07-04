package proxy

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"openhijack/internal/audit"
	"openhijack/internal/config"
)

type ProxyServer struct {
	configMu    sync.RWMutex
	config      *config.Config
	configPath  string
	auth        *ProxyAuth
	transport   *UpstreamTransport
	logger      *slog.Logger
	server      *http.Server
	listenHost  string
	listenPort  int
	debugMode   bool
	useTLS      bool
	tlsCertFile string
	tlsKeyFile  string
	tlsCerts    map[string]*tls.Certificate
	cleanupFn   func()

	audit     *audit.AuditLogger
	auditFile *os.File
	auditPath string

	watcher             *config.Watcher
	watcherMu           sync.Mutex
	watcherStatus       WatcherStatus
	onConfigReloadedHook func(WatcherStatus)
}

const openAIDefaultRoutePrefix = "/v1"

// WatcherStatus records the most recent watcher activity for diagnostic UIs.
type WatcherStatus struct {
	Running   bool   `json:"running"`
	LastReload string `json:"last_reload,omitempty"`
	LastError  string `json:"last_error,omitempty"`
}

type ServeOptions struct {
	ConfigPath       string
	Host             string
	Port             int
	UseTLS           bool
	DebugMode        bool
	DisableSSLStrict bool
	ForceStream      bool
	StreamMode       string
	TLSCertFile      string
	TLSKeyFile       string
	ExtraTLSCerts    map[string]string
	CleanupFn        func()
	LogCallback      func(string)

	// AuditLogPath, when non-empty, enables request audit logging to that
	// file in JSON Lines format. The file is opened in append mode.
	AuditLogPath string

	// WatchConfig, when true, starts an fsnotify watcher on ConfigPath
	// and hot-reloads the in-memory config + transport on change.
	WatchConfig bool

	// OnConfigReloaded, when non-nil, is invoked after every config
	// reload attempt (success or failure) with the latest WatcherStatus.
	// Used by the GUI to emit Wails events to the frontend. The CLI
	// leaves this nil.
	OnConfigReloaded func(WatcherStatus)
}

type callbackHandler struct {
	callback func(string)
}

func (h *callbackHandler) Handle(_ context.Context, r slog.Record) error {
	var sb strings.Builder
	sb.WriteString(r.Message)
	r.Attrs(func(a slog.Attr) bool {
		sb.WriteString(fmt.Sprintf(" %s=%v", a.Key, a.Value))
		return true
	})
	h.callback(sb.String())
	return nil
}

func (h *callbackHandler) Enabled(_ context.Context, _ slog.Level) bool {
	return true
}

func (h *callbackHandler) WithAttrs(_ []slog.Attr) slog.Handler {
	return h
}

func (h *callbackHandler) WithGroup(_ string) slog.Handler {
	return h
}

func NewProxyServer(opts ServeOptions) (*ProxyServer, error) {
	cfg, err := config.Load(opts.ConfigPath)
	if err != nil {
		return nil, err
	}

	var logger *slog.Logger
	if opts.LogCallback != nil {
		logger = slog.New(&callbackHandler{callback: opts.LogCallback})
	} else {
		logger = slog.Default()
	}

	auth := NewProxyAuth(cfg.AuthKey)
	transport := NewUpstreamTransport(cfg, opts.DebugMode, opts.DisableSSLStrict, logger)

	tlsCerts := make(map[string]*tls.Certificate)
	if opts.UseTLS && opts.TLSCertFile != "" && opts.TLSKeyFile != "" {
		defaultCert, err := tls.LoadX509KeyPair(opts.TLSCertFile, opts.TLSKeyFile)
		if err != nil {
			return nil, fmt.Errorf("加载默认 TLS 证书失败: %w", err)
		}
		tlsCerts[""] = &defaultCert

		for domain, certPrefix := range opts.ExtraTLSCerts {
			certFile := certPrefix + ".crt"
			keyFile := certPrefix + ".key"
			extraCert, err := tls.LoadX509KeyPair(certFile, keyFile)
			if err != nil {
				logger.Warn("加载额外 TLS 证书失败，跳过", "domain", domain, "err", err)
				continue
			}
			tlsCerts[domain] = &extraCert
			logger.Info("加载额外 TLS 证书", "domain", domain)
		}
	}

	srv := &ProxyServer{
		config:      cfg,
		configPath:  opts.ConfigPath,
		auth:        auth,
		transport:   transport,
		logger:      logger,
		listenHost:  opts.Host,
		listenPort:  opts.Port,
		debugMode:   opts.DebugMode,
		useTLS:      opts.UseTLS,
		tlsCertFile: opts.TLSCertFile,
		tlsKeyFile:  opts.TLSKeyFile,
		tlsCerts:    tlsCerts,
		cleanupFn:   opts.CleanupFn,

		onConfigReloadedHook: opts.OnConfigReloaded,
	}

	if opts.AuditLogPath != "" {
		f, err := os.OpenFile(opts.AuditLogPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
		if err != nil {
			return nil, fmt.Errorf("打开审计日志文件失败: %w", err)
		}
		srv.auditFile = f
		srv.audit = audit.NewAuditLogger(f)
		srv.auditPath = opts.AuditLogPath
		logger.Info("审计日志已启用", "path", opts.AuditLogPath)
	}

	if opts.WatchConfig {
		w, err := config.NewWatcher(opts.ConfigPath, srv.onConfigReload)
		if err != nil {
			logger.Warn("启动配置监听失败，热重载已禁用", "err", err)
		} else {
			srv.watcher = w
			srv.watcherStatus.Running = true
			logger.Info("配置热重载已启用", "path", opts.ConfigPath)
		}
	}

	return srv, nil
}

// onConfigReload is called by the watcher whenever the config file changes.
// On success it swaps the in-memory config + transport under the config
// mutex. On error it records the failure but keeps serving the old config.
func (s *ProxyServer) onConfigReload(newCfg *config.Config, loadErr error) {
	s.watcherMu.Lock()
	defer s.watcherMu.Unlock()

	if loadErr != nil {
		s.watcherStatus.LastError = loadErr.Error()
		s.logger.Warn("配置热重载失败", "err", loadErr)
		s.notifyReloadedHook()
		return
	}
	if newCfg == nil {
		s.notifyReloadedHook()
		return
	}

	auth := NewProxyAuth(newCfg.AuthKey)
	transport := NewUpstreamTransport(newCfg, s.debugMode, false, s.logger)

	s.configMu.Lock()
	s.config = newCfg
	s.auth = auth
	s.transport = transport
	s.configMu.Unlock()

	s.watcherStatus.LastReload = time.Now().Format(time.RFC3339)
	s.watcherStatus.LastError = ""
	s.logger.Info("配置已热重载", "mapped_model", newCfg.MappedModelID)

	s.notifyReloadedHook()
}

// notifyReloadedHook forwards the current watcher status to the
// registered hook (if any). Called under s.watcherMu.
func (s *ProxyServer) notifyReloadedHook() {
	if s.onConfigReloadedHook == nil {
		return
	}
	// Copy the status so the receiver can't mutate internal state.
	snapshot := s.watcherStatus
	go s.onConfigReloadedHook(snapshot)
}

// GetWatcherStatus returns a snapshot of the watcher state for UI consumers.
func (s *ProxyServer) GetWatcherStatus() WatcherStatus {
	s.watcherMu.Lock()
	defer s.watcherMu.Unlock()
	return s.watcherStatus
}

// ReloadConfigManually forces a config reload regardless of watcher state.
func (s *ProxyServer) ReloadConfigManually() error {
	cfg, err := config.Load(s.configPath)
	if err != nil {
		return err
	}
	if cfg == nil {
		return fmt.Errorf("config load returned nil")
	}
	s.onConfigReload(cfg, nil)
	return nil
}

// CurrentConfig returns a pointer to the current config under the read lock.
// Callers must not mutate the returned config.
func (s *ProxyServer) CurrentConfig() *config.Config {
	s.configMu.RLock()
	defer s.configMu.RUnlock()
	return s.config
}

// CurrentTransport returns the current transport under the read lock.
func (s *ProxyServer) CurrentTransport() *UpstreamTransport {
	s.configMu.RLock()
	defer s.configMu.RUnlock()
	return s.transport
}

// CurrentAuth returns the current auth under the read lock.
func (s *ProxyServer) CurrentAuth() *ProxyAuth {
	s.configMu.RLock()
	defer s.configMu.RUnlock()
	return s.auth
}

// AuditLogger exposes the audit logger for testing and UI bindings.
// May return nil if audit logging is disabled.
func (s *ProxyServer) AuditLogger() *audit.AuditLogger {
	return s.audit
}

// AuditLogPath returns the on-disk path of the audit log file, or an
// empty string if audit logging is disabled. Used by the GUI to
// expose a "show log file" action and to read historical entries.
func (s *ProxyServer) AuditLogPath() string {
	return s.auditPath
}

func (s *ProxyServer) newRequestID() string {
	return fmt.Sprintf("%06x", time.Now().UnixNano()%0xFFFFFF)
}

func (s *ProxyServer) timestampMs() string {
	now := time.Now()
	return now.Format("15:04:05") + fmt.Sprintf(".%03d", now.Nanosecond()/1000000)
}

func (s *ProxyServer) logRequest(requestID, message string) {
	s.logger.Info("request", "time", s.timestampMs(), "id", requestID, "msg", message)
}

// auditLog writes a single audit record if audit logging is enabled.
// It is safe to call when s.audit is nil (no-op).
func (s *ProxyServer) auditLog(requestID string, r *http.Request, status int, upstream, model string, start time.Time, errMsg string) {
	if s.audit == nil {
		return
	}
	_ = s.audit.LogRequest(requestID, r.Method, r.URL.Path, status, upstream, model, clientIPFromRequest(r), time.Since(start), errMsg)
}

// clientIPFromRequest extracts the client IP from an http.Request, falling
// back to the remote address if no forwarding header is present.
func clientIPFromRequest(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if idx := strings.Index(xff, ","); idx > 0 {
			return strings.TrimSpace(xff[:idx])
		}
		return strings.TrimSpace(xff)
	}
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return strings.TrimSpace(xri)
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func buildDefaultOpenAIRoute(suffix string) string {
	return openAIDefaultRoutePrefix + "/" + strings.TrimPrefix(suffix, "/")
}

func (s *ProxyServer) buildModelsRoute() string {
	return s.buildRoute("models")
}

func (s *ProxyServer) buildChatCompletionsRoute() string {
	return s.buildRoute("chat/completions")
}

func (s *ProxyServer) buildRoute(suffix string) string {
	middle := s.CurrentConfig().CurrentGroup().MiddleRoute
	if middle == "" || middle == "/" {
		return "/" + strings.TrimPrefix(suffix, "/")
	}
	return strings.TrimRight(middle, "/") + "/" + strings.TrimPrefix(suffix, "/")
}

func (s *ProxyServer) routeVariants(suffix string) []string {
	candidates := []string{
		s.buildRoute(suffix),
		buildDefaultOpenAIRoute(suffix),
	}
	seen := make(map[string]struct{}, len(candidates))
	routes := make([]string, 0, len(candidates))
	for _, route := range candidates {
		if _, exists := seen[route]; exists {
			continue
		}
		seen[route] = struct{}{}
		routes = append(routes, route)
	}
	return routes
}

func (s *ProxyServer) authorizeRequest(w http.ResponseWriter, r *http.Request, requestID string, logScope string) bool {
	if err := s.CurrentAuth().Verify(r.Header.Get("Authorization"), r.RemoteAddr); err == nil {
		return true
	}

	s.logRequest(requestID, logScope+"鉴权失败")
	WriteAuthenticationError(w)
	return false
}

func (s *ProxyServer) handleModels(w http.ResponseWriter, r *http.Request) {
	requestID := s.newRequestID()
	start := time.Now()
	s.logRequest(requestID, fmt.Sprintf("收到模型列表请求 %s", r.URL.Path))

	if !s.authorizeRequest(w, r, requestID, "模型列表请求") {
		s.auditLog(requestID, r, http.StatusUnauthorized, "", "", start, "auth failed")
		return
	}

	cfg := s.CurrentConfig()
	mappedModel := cfg.MappedModelID

	modelData := map[string]interface{}{
		"object": "list",
		"data": []interface{}{
			map[string]interface{}{
				"id":       mappedModel,
				"object":   "model",
				"owned_by": "openai",
				"created":  time.Now().Unix(),
				"permission": []interface{}{
					map[string]interface{}{
						"id":                   fmt.Sprintf("modelperm-%s", mappedModel),
						"object":               "model_permission",
						"created":              time.Now().Unix(),
						"allow_create_engine":  false,
						"allow_sampling":       true,
						"allow_logprobs":       true,
						"allow_search_indices": false,
						"allow_view":           true,
						"allow_fine_tuning":    false,
						"organization":         "*",
						"group":                nil,
						"is_blocking":          false,
					},
				},
			},
		},
	}

	s.logRequest(requestID, fmt.Sprintf("返回映射模型: %s", mappedModel))
	WriteJSON(w, http.StatusOK, modelData)
	s.auditLog(requestID, r, http.StatusOK, "", mappedModel, start, "")
}

func (s *ProxyServer) handleChatCompletions(w http.ResponseWriter, r *http.Request) {
	requestID := s.newRequestID()
	start := time.Now()

	s.logRequest(requestID, fmt.Sprintf("收到 Chat Completions 请求 %s", s.buildChatCompletionsRoute()))
	s.logRequest(requestID, fmt.Sprintf("请求方法: %s, Content-Length: %s", r.Method, r.Header.Get("Content-Length")))

	if r.Method != http.MethodPost {
		WriteJSON(w, http.StatusMethodNotAllowed, map[string]interface{}{
			"error": map[string]interface{}{
				"message": "Method not allowed",
				"type":    "invalid_request_error",
			},
		})
		s.auditLog(requestID, r, http.StatusMethodNotAllowed, "", "", start, "method not allowed")
		return
	}

	body, err := ReadRequestBody(r)
	if err != nil {
		s.logRequest(requestID, fmt.Sprintf("读取请求体失败: %v", err))
		WriteJSON(w, http.StatusBadRequest, map[string]interface{}{
			"error": "Failed to read request body",
		})
		s.auditLog(requestID, r, http.StatusBadRequest, "", "", start, "read body failed")
		return
	}

	var reqData map[string]interface{}
	if err := json.Unmarshal(body, &reqData); err != nil {
		s.logRequest(requestID, "解析 JSON 失败或请求不是 JSON 格式")
		WriteJSON(w, http.StatusBadRequest, map[string]interface{}{
			"error":   "Invalid JSON or Content-Type",
			"message": "The request body must be valid JSON and the Content-Type header must be 'application/json'.",
		})
		s.auditLog(requestID, r, http.StatusBadRequest, "", "", start, "invalid json")
		return
	}

	if s.debugMode {
		headersStr := ""
		for k, v := range r.Header {
			headersStr += fmt.Sprintf("%s: %s\n", k, strings.Join(v, ", "))
		}
		s.logRequest(requestID, fmt.Sprintf("--- 请求头 (调试模式) ---\n%s---", headersStr))
		s.logRequest(requestID, fmt.Sprintf("--- 请求体 (调试模式) ---\n%s\n---", string(body)))
	}

	if !s.authorizeRequest(w, r, requestID, "Chat Completions 请求") {
		s.auditLog(requestID, r, http.StatusUnauthorized, "", "", start, "auth failed")
		return
	}

	s.logRequest(requestID, "✅ 鉴权通过，准备转发到上游")

	clientRequestedStream := false
	if streamVal, ok := reqData["stream"]; ok {
		clientRequestedStream = ToBool(streamVal)
	}
	s.logRequest(requestID, fmt.Sprintf("客户端请求的流模式: %v", clientRequestedStream))

	isStream := clientRequestedStream
	transport := s.CurrentTransport()
	cfg := s.CurrentConfig()

	upstreamResp, err := transport.ForwardChatCompletions(r.Context(), requestID, body, isStream)
	if err != nil {
		s.logRequest(requestID, fmt.Sprintf("上游请求失败: %v", err))
		WriteOpenAIError(w, http.StatusBadGateway, fmt.Sprintf("Upstream request failed: %v", err), "upstream_error")
		s.auditLog(requestID, r, http.StatusBadGateway, cfg.CurrentGroup().FullAPIURL("chat/completions"), cfg.TargetModelID(), start, err.Error())
		return
	}
	defer upstreamResp.Body.Close()

	s.logRequest(requestID, fmt.Sprintf("上游响应状态: %d", upstreamResp.StatusCode))

	if upstreamResp.StatusCode != http.StatusOK {
		respBody, _ := ReadBody(upstreamResp)
		s.logRequest(requestID, fmt.Sprintf("上游错误响应: %s", string(respBody)))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(upstreamResp.StatusCode)
		w.Write(respBody)
		s.auditLog(requestID, r, upstreamResp.StatusCode, cfg.CurrentGroup().FullAPIURL("chat/completions"), cfg.TargetModelID(), start, "upstream error")
		return
	}

	contentType := upstreamResp.Header.Get("Content-Type")
	isUpstreamStream := strings.Contains(contentType, "text/event-stream")

	if isUpstreamStream {
		s.logRequest(requestID, "返回流式响应")
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("X-Accel-Buffering", "no")

		done := make(chan struct{})
		go func() {
			StreamSSE(upstreamResp, w, done)
		}()
		<-done
		s.auditLog(requestID, r, http.StatusOK, cfg.CurrentGroup().FullAPIURL("chat/completions"), cfg.TargetModelID(), start, "")
		return
	}

	respBody, err := ReadBody(upstreamResp)
	if err != nil {
		s.logRequest(requestID, fmt.Sprintf("读取上游响应失败: %v", err))
		WriteJSON(w, http.StatusBadGateway, map[string]interface{}{
			"error": "Failed to read upstream response",
		})
		s.auditLog(requestID, r, http.StatusBadGateway, cfg.CurrentGroup().FullAPIURL("chat/completions"), cfg.TargetModelID(), start, "read upstream body failed")
		return
	}

	if clientRequestedStream {
		s.logRequest(requestID, "将非流式响应转换为 SSE 流返回给客户端")
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		StreamNonStreamAsSSE(respBody, w, s.logger)
		s.auditLog(requestID, r, http.StatusOK, cfg.CurrentGroup().FullAPIURL("chat/completions"), cfg.TargetModelID(), start, "")
		return
	}

	if s.debugMode {
		s.logRequest(requestID, fmt.Sprintf("--- 完整响应体 (调试模式) ---\n%s\n---", string(respBody)))
	} else {
		s.logRequest(requestID, "返回非流式 JSON 响应")
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(respBody)
	s.auditLog(requestID, r, http.StatusOK, cfg.CurrentGroup().FullAPIURL("chat/completions"), cfg.TargetModelID(), start, "")
}

func (s *ProxyServer) handleOther(w http.ResponseWriter, r *http.Request) {
	requestID := s.newRequestID()
	start := time.Now()
	s.logRequest(requestID, fmt.Sprintf("透传请求: %s %s", r.Method, r.URL.Path))

	if !s.authorizeRequest(w, r, requestID, "透传请求") {
		s.auditLog(requestID, r, http.StatusUnauthorized, "", "", start, "auth failed")
		return
	}

	cfg := s.CurrentConfig()
	group := cfg.CurrentGroup()
	transport := s.CurrentTransport()

	upstreamURL := group.TargetAPIBaseURL() + r.URL.Path
	if r.URL.RawQuery != "" {
		upstreamURL += "?" + r.URL.RawQuery
	}

	body, err := ReadRequestBody(r)
	if err != nil {
		WriteJSON(w, http.StatusBadRequest, map[string]interface{}{"error": "Failed to read request body"})
		s.auditLog(requestID, r, http.StatusBadRequest, upstreamURL, "", start, "read body failed")
		return
	}

	req, err := http.NewRequestWithContext(r.Context(), r.Method, upstreamURL, strings.NewReader(string(body)))
	if err != nil {
		WriteJSON(w, http.StatusInternalServerError, map[string]interface{}{"error": "Failed to create upstream request"})
		s.auditLog(requestID, r, http.StatusInternalServerError, upstreamURL, "", start, "create request failed")
		return
	}

	for k, v := range r.Header {
		if strings.EqualFold(k, "host") || strings.EqualFold(k, "content-length") || strings.EqualFold(k, "authorization") {
			continue
		}
		req.Header[k] = v
	}

	if group.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+group.APIKey)
	}

	for key, value := range group.Headers {
		req.Header.Set(key, value)
	}

	resp, err := transport.client.Do(req)
	if err != nil {
		s.logRequest(requestID, fmt.Sprintf("透传上游请求失败: %v", err))
		WriteJSON(w, http.StatusBadGateway, map[string]interface{}{"error": "Upstream request failed"})
		s.auditLog(requestID, r, http.StatusBadGateway, upstreamURL, group.ModelID, start, err.Error())
		return
	}
	defer resp.Body.Close()

	for k, v := range resp.Header {
		w.Header()[k] = v
	}
	w.WriteHeader(resp.StatusCode)
	readBuf := make([]byte, 32*1024)
	for {
		n, err := resp.Body.Read(readBuf)
		if n > 0 {
			w.Write(readBuf[:n])
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
		}
		if err != nil {
			break
		}
	}
	s.auditLog(requestID, r, resp.StatusCode, upstreamURL, group.ModelID, start, "")
}

func (s *ProxyServer) setupRoutes() http.Handler {
	mux := http.NewServeMux()

	modelsRoutes := s.routeVariants("models")
	chatRoutes := s.routeVariants("chat/completions")

	s.logger.Info("registering routes", "models", modelsRoutes, "chat", chatRoutes)

	for _, route := range modelsRoutes {
		route := route
		mux.HandleFunc(route, func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodGet {
				s.handleModels(w, r)
				return
			}
			http.NotFound(w, r)
		})
	}

	for _, route := range chatRoutes {
		route := route
		mux.HandleFunc(route, func(w http.ResponseWriter, r *http.Request) {
			s.handleChatCompletions(w, r)
		})
	}

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		s.handleOther(w, r)
	})

	return mux
}

func (s *ProxyServer) Start() error {
	handler := s.setupRoutes()

	addr := fmt.Sprintf("%s:%d", s.listenHost, s.listenPort)

	s.server = &http.Server{
		Addr:    addr,
		Handler: handler,
		BaseContext: func(l net.Listener) context.Context {
			return context.Background()
		},
	}

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("监听 %s 失败: %w", addr, err)
	}

	if s.useTLS && s.tlsCertFile != "" && s.tlsKeyFile != "" {
		s.logger.Info("server starting", "addr", addr, "mode", "HTTPS/TLS")
		s.logger.Info("TLS config", "cert", s.tlsCertFile, "key", s.tlsKeyFile)

		defaultCert := s.tlsCerts[""]

		tlsConfig := &tls.Config{
			MinVersion: tls.VersionTLS12,
			MaxVersion: tls.VersionTLS13,
			GetCertificate: func(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
				if hello.ServerName != "" {
					if cert, ok := s.tlsCerts[hello.ServerName]; ok {
						s.logger.Debug("TLS SNI certificate selected", "server_name", hello.ServerName)
						return cert, nil
					}
				}
				return defaultCert, nil
			},
		}

		tlsLn := tls.NewListener(ln, tlsConfig)

		go func() {
			if err := s.server.Serve(tlsLn); err != nil && err != http.ErrServerClosed {
				s.logger.Error("server error", "err", err)
			}
		}()
	} else {
		s.logger.Info("server starting", "addr", addr, "mode", "HTTP")
		go func() {
			if err := s.server.Serve(ln); err != nil && err != http.ErrServerClosed {
				s.logger.Error("server error", "err", err)
			}
		}()
	}

	cfg := s.CurrentConfig()
	s.logger.Info("server started", "addr", addr, "mapped_model", cfg.MappedModelID, "target_model", cfg.TargetModelID())
	s.logger.Info("upstream URL", "url", cfg.CurrentGroup().FullAPIURL("chat/completions"))

	return nil
}

func (s *ProxyServer) Stop() error {
	var firstErr error
	if s.server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := s.server.Shutdown(ctx); err != nil {
			firstErr = err
		}
	}
	if s.watcher != nil {
		if err := s.watcher.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
		s.watcher = nil
	}
	if s.auditFile != nil {
		if err := s.auditFile.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
		s.auditFile = nil
	}
	if s.auth != nil {
		_ = s.auth.Close()
	}
	return firstErr
}

func (s *ProxyServer) Wait() {
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	sig := <-stop
	s.logger.Info("received shutdown signal", "signal", sig.String())
	s.Stop()
	if s.cleanupFn != nil {
		s.cleanupFn()
	}
}

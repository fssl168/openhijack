package proxy

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"openhijack/internal/config"
)

type ProxyServer struct {
	config      *config.Config
	auth        *ProxyAuth
	transport   *UpstreamTransport
	logger      *log.Logger
	server      *http.Server
	listenHost  string
	listenPort  int
	debugMode   bool
	useTLS      bool
	tlsCertFile string
	tlsKeyFile  string
	cleanupFn   func()
}

const openAIDefaultRoutePrefix = "/v1"

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
	CleanupFn        func()
}

func NewProxyServer(opts ServeOptions) (*ProxyServer, error) {
	cfg, err := config.Load(opts.ConfigPath)
	if err != nil {
		return nil, err
	}

	logger := log.New(os.Stderr, "[openhijack] ", log.LstdFlags|log.Lmicroseconds)
	logger.Printf("配置加载成功: mapped_model_id=%s, 当前组=%s", cfg.MappedModelID, cfg.CurrentGroup().Name)

	group := cfg.CurrentGroup()
	logger.Printf("上游配置: provider=%s, api_url=%s, model_id=%s", group.Provider, group.APIURL, group.ModelID)
	if len(group.Headers) > 0 {
		for k, v := range group.Headers {
			logger.Printf("自定义请求头: %s: %s", k, v)
		}
	} else {
		logger.Printf("自定义请求头: 无")
	}

	auth := NewProxyAuth(cfg.AuthKey)
	transport := NewUpstreamTransport(cfg, opts.DebugMode, opts.DisableSSLStrict, logger)

	return &ProxyServer{
		config:      cfg,
		auth:        auth,
		transport:   transport,
		logger:      logger,
		listenHost:  opts.Host,
		listenPort:  opts.Port,
		debugMode:   opts.DebugMode,
		useTLS:      opts.UseTLS,
		tlsCertFile: opts.TLSCertFile,
		tlsKeyFile:  opts.TLSKeyFile,
		cleanupFn:   opts.CleanupFn,
	}, nil
}

func (s *ProxyServer) newRequestID() string {
	return fmt.Sprintf("%06x", time.Now().UnixNano()%0xFFFFFF)
}

func (s *ProxyServer) timestampMs() string {
	now := time.Now()
	return now.Format("15:04:05") + fmt.Sprintf(".%03d", now.Nanosecond()/1000000)
}

func (s *ProxyServer) logRequest(requestID, message string) {
	s.logger.Printf("%s [%s] %s", s.timestampMs(), requestID, message)
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
	middle := s.config.CurrentGroup().MiddleRoute
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
	if s.auth.Verify(r.Header.Get("Authorization")) {
		return true
	}

	s.logRequest(requestID, logScope+"鉴权失败")
	WriteAuthenticationError(w)
	return false
}

func (s *ProxyServer) handleModels(w http.ResponseWriter, r *http.Request) {
	requestID := s.newRequestID()
	s.logRequest(requestID, fmt.Sprintf("收到模型列表请求 %s", r.URL.Path))

	if !s.authorizeRequest(w, r, requestID, "模型列表请求") {
		return
	}

	modelData := map[string]interface{}{
		"object": "list",
		"data": []interface{}{
			map[string]interface{}{
				"id":       s.config.MappedModelID,
				"object":   "model",
				"owned_by": "openai",
				"created":  time.Now().Unix(),
				"permission": []interface{}{
					map[string]interface{}{
						"id":                   fmt.Sprintf("modelperm-%s", s.config.MappedModelID),
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

	s.logRequest(requestID, fmt.Sprintf("返回映射模型: %s", s.config.MappedModelID))
	WriteJSON(w, http.StatusOK, modelData)
}

func (s *ProxyServer) handleChatCompletions(w http.ResponseWriter, r *http.Request) {
	requestID := s.newRequestID()

	s.logRequest(requestID, fmt.Sprintf("收到 Chat Completions 请求 %s", s.buildChatCompletionsRoute()))

	if r.Method != http.MethodPost {
		WriteJSON(w, http.StatusMethodNotAllowed, map[string]interface{}{
			"error": map[string]interface{}{
				"message": "Method not allowed",
				"type":    "invalid_request_error",
			},
		})
		return
	}

	body, err := ReadRequestBody(r)
	if err != nil {
		s.logRequest(requestID, fmt.Sprintf("读取请求体失败: %v", err))
		WriteJSON(w, http.StatusBadRequest, map[string]interface{}{
			"error": "Failed to read request body",
		})
		return
	}

	var reqData map[string]interface{}
	if err := json.Unmarshal(body, &reqData); err != nil {
		s.logRequest(requestID, "解析 JSON 失败或请求不是 JSON 格式")
		WriteJSON(w, http.StatusBadRequest, map[string]interface{}{
			"error":   "Invalid JSON or Content-Type",
			"message": "The request body must be valid JSON and the Content-Type header must be 'application/json'.",
		})
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
		return
	}

	clientRequestedStream := false
	if streamVal, ok := reqData["stream"]; ok {
		clientRequestedStream = ToBool(streamVal)
	}
	s.logRequest(requestID, fmt.Sprintf("客户端请求的流模式: %v", clientRequestedStream))

	isStream := clientRequestedStream

	upstreamResp, err := s.transport.ForwardChatCompletions(r.Context(), requestID, body, isStream)
	if err != nil {
		s.logRequest(requestID, fmt.Sprintf("上游请求失败: %v", err))
		WriteOpenAIError(w, http.StatusBadGateway, fmt.Sprintf("Upstream request failed: %v", err), "upstream_error")
		return
	}

	s.logRequest(requestID, fmt.Sprintf("上游响应状态: %d", upstreamResp.StatusCode))

	if upstreamResp.StatusCode != http.StatusOK {
		respBody, _ := ReadBody(upstreamResp)
		s.logRequest(requestID, fmt.Sprintf("上游错误响应: %s", string(respBody)))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(upstreamResp.StatusCode)
		w.Write(respBody)
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
		return
	}

	respBody, err := ReadBody(upstreamResp)
	if err != nil {
		s.logRequest(requestID, fmt.Sprintf("读取上游响应失败: %v", err))
		WriteJSON(w, http.StatusBadGateway, map[string]interface{}{
			"error": "Failed to read upstream response",
		})
		return
	}

	if clientRequestedStream {
		s.logRequest(requestID, "将非流式响应转换为 SSE 流返回给客户端")
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		StreamNonStreamAsSSE(respBody, w, s.logger)
		return
	}

	if s.debugMode {
		s.logRequest(requestID, fmt.Sprintf("--- 完整响应体 (调试模式) ---\n%s\n---", string(respBody)))
	} else {
		s.logRequest(requestID, "返回非流式 JSON 响应")
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(respBody)
}

func (s *ProxyServer) handleOther(w http.ResponseWriter, r *http.Request) {
	requestID := s.newRequestID()
	s.logRequest(requestID, fmt.Sprintf("透传请求: %s %s", r.Method, r.URL.Path))

	if !s.authorizeRequest(w, r, requestID, "透传请求") {
		return
	}

	upstreamURL := s.config.CurrentGroup().TargetAPIBaseURL() + r.URL.Path
	if r.URL.RawQuery != "" {
		upstreamURL += "?" + r.URL.RawQuery
	}

	body, err := ReadRequestBody(r)
	if err != nil {
		WriteJSON(w, http.StatusBadRequest, map[string]interface{}{"error": "Failed to read request body"})
		return
	}

	req, err := http.NewRequestWithContext(r.Context(), r.Method, upstreamURL, strings.NewReader(string(body)))
	if err != nil {
		WriteJSON(w, http.StatusInternalServerError, map[string]interface{}{"error": "Failed to create upstream request"})
		return
	}

	for k, v := range r.Header {
		if strings.EqualFold(k, "host") || strings.EqualFold(k, "content-length") || strings.EqualFold(k, "authorization") {
			continue
		}
		req.Header[k] = v
	}

	if s.config.CurrentGroup().APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+s.config.CurrentGroup().APIKey)
	}

	for key, value := range s.config.CurrentGroup().Headers {
		req.Header.Set(key, value)
	}

	resp, err := s.transport.client.Do(req)
	if err != nil {
		s.logRequest(requestID, fmt.Sprintf("透传上游请求失败: %v", err))
		WriteJSON(w, http.StatusBadGateway, map[string]interface{}{"error": "Upstream request failed"})
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
}

func (s *ProxyServer) setupRoutes() http.Handler {
	mux := http.NewServeMux()

	modelsRoutes := s.routeVariants("models")
	chatRoutes := s.routeVariants("chat/completions")

	s.logger.Printf("注册模型路由: GET %s", strings.Join(modelsRoutes, ", "))
	s.logger.Printf("注册对话路由: POST %s", strings.Join(chatRoutes, ", "))

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
		s.logger.Printf("代理服务器启动: %s (HTTPS/TLS 模式)", addr)
		go func() {
			if err := s.server.ServeTLS(ln, s.tlsCertFile, s.tlsKeyFile); err != nil && err != http.ErrServerClosed {
				s.logger.Printf("服务器错误: %v", err)
			}
		}()
	} else {
		s.logger.Printf("代理服务器启动: %s (HTTP 模式)", addr)
		go func() {
			if err := s.server.Serve(ln); err != nil && err != http.ErrServerClosed {
				s.logger.Printf("服务器错误: %v", err)
			}
		}()
	}

	s.logger.Printf("代理服务器已启动: %s", addr)
	s.logger.Printf("模型映射: %s -> %s", s.config.MappedModelID, s.config.TargetModelID())
	s.logger.Printf("上游地址: %s", s.config.CurrentGroup().FullAPIURL("chat/completions"))

	return nil
}

func (s *ProxyServer) Stop() error {
	if s.server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return s.server.Shutdown(ctx)
	}
	return nil
}

func (s *ProxyServer) Wait() {
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	sig := <-stop
	s.logger.Printf("收到信号 %v，正在关闭...", sig)
	s.Stop()
	if s.cleanupFn != nil {
		s.cleanupFn()
	}
}

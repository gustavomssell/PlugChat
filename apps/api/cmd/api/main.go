package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"chatfree/api/internal/accounts"
	"chatfree/api/internal/auth"
	"chatfree/api/internal/bootstrap"
	"chatfree/api/internal/chat"
	"chatfree/api/internal/db"
	"chatfree/api/internal/files"
	"chatfree/api/internal/httpapi"
	"chatfree/api/internal/llm"
	"chatfree/api/internal/realtime"
	"chatfree/api/internal/store"
)

func main() {
	port := env("PORT", "8090")
	databaseURL := env("DATABASE_URL", "postgres://chatfree:chatfree@localhost:5437/chatfree?sslmode=disable")
	jwtSecret := env("JWT_SECRET", "chatfree-dev-secret")

	startupCtx, cancelStartup := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancelStartup()

	pool, err := db.Open(startupCtx, databaseURL)
	if err != nil {
		log.Fatalf("banco de dados indisponível: %v", err)
	}
	defer pool.Close()

	if err := db.Migrate(startupCtx, pool); err != nil {
		log.Fatalf("migrações falharam: %v", err)
	}

	st := store.NewPostgres(pool)
	if err := bootstrap.Run(startupCtx, st); err != nil {
		log.Fatalf("bootstrap falhou: %v", err)
	}

	hub := realtime.NewHub()
	tokens := auth.NewManager(jwtSecret, 7*24*time.Hour)
	chatService := chat.New(st, hub, chat.WithLLM(llm.FromEnv()))
	accountsService := accounts.New(st, tokens)

	widgetPolicy := httpapi.DefaultWidgetPolicy()
	widgetPolicy.TrustProxy = envBool("TRUST_PROXY", false)
	widgetPolicy.ConfigPerMinute = envInt("WIDGET_CONFIG_PER_MINUTE", widgetPolicy.ConfigPerMinute)
	widgetPolicy.StartsPerTenMinutesPerIP = envInt("WIDGET_STARTS_PER_10M", widgetPolicy.StartsPerTenMinutesPerIP)
	widgetPolicy.StartsPerHourPerChannel = envInt("WIDGET_STARTS_PER_HOUR", widgetPolicy.StartsPerHourPerChannel)
	widgetPolicy.ReadsPerMinutePerConversation = envInt("WIDGET_READS_PER_MINUTE", widgetPolicy.ReadsPerMinutePerConversation)
	widgetPolicy.MessagesPerMinutePerIP = envInt("WIDGET_MESSAGES_PER_MINUTE", widgetPolicy.MessagesPerMinutePerIP)
	widgetPolicy.MessagesPerMinutePerConversation = envInt("WIDGET_MESSAGES_PER_MINUTE_CONVERSATION", widgetPolicy.MessagesPerMinutePerConversation)

	attachmentStore, err := files.New(env("ATTACHMENTS_DIR", "data/uploads"))
	if err != nil {
		log.Fatalf("diretório de anexos indisponível: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go chatService.StartDelayWorker(ctx)

	server := &http.Server{
		Addr: ":" + port,
		Handler: httpapi.New(
			st,
			chatService,
			accountsService,
			tokens,
			hub,
			httpapi.WithWidgetPolicy(widgetPolicy),
			httpapi.WithAttachments(attachmentStore),
		),
		ReadTimeout:       15 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       75 * time.Second,
		MaxHeaderBytes:    1 << 20,
		// WriteTimeout is intentionally unset: SSE streams stay open until the
		// client disconnects or the server shuts down.
	}

	go func() {
		log.Printf("ChatFree API ouvindo em http://localhost:%s", port)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("erro no servidor: %v", err)
		}
	}()

	<-ctx.Done()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("erro ao encerrar: %v", err)
	}

	log.Println("API encerrada")
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func envBool(key string, fallback bool) bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(key))) {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return fallback
	}
}

func envInt(key string, fallback int) int {
	value, err := strconv.Atoi(strings.TrimSpace(os.Getenv(key)))
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}

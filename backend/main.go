package main

import (
	"context"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"

	"github.com/JoaoCarlosAssis/MS_SDR_FIX/internal/cache"
	"github.com/JoaoCarlosAssis/MS_SDR_FIX/internal/config"
	dbpkg "github.com/JoaoCarlosAssis/MS_SDR_FIX/internal/db"
	"github.com/JoaoCarlosAssis/MS_SDR_FIX/internal/migrations"
	"github.com/JoaoCarlosAssis/MS_SDR_FIX/internal/repository"
	"github.com/JoaoCarlosAssis/MS_SDR_FIX/internal/router"
	"github.com/JoaoCarlosAssis/MS_SDR_FIX/internal/service"
)

func main() {
	// Carrega .env (opcional). Tenta caminhos comuns sem falhar o processo.
	_ = godotenv.Load(".env.local")
	_ = godotenv.Overload(".env")
	_ = godotenv.Load("../.env")

	cfg := config.Load()

	// Logger básico controlado por env
	var infoLogger *log.Logger
	if strings.EqualFold(os.Getenv("LOG_LEVEL"), "error") {
		infoLogger = log.New(io.Discard, "", 0) // silencia INFO
	} else {
		infoLogger = log.New(os.Stdout, "", log.LstdFlags)
	}
	errorLogger := log.New(os.Stderr, "", log.LstdFlags)

	// HTTP client compartilhado
	httpClient := &http.Client{Timeout: 15 * time.Second}

	// DB
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	pool, err := dbpkg.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		errorLogger.Fatalf("erro ao conectar no banco: %v", err)
	}
	defer pool.Close()

	// Migrations mínimas
	if err := migrations.Run(ctx, pool); err != nil {
		errorLogger.Fatalf("erro ao rodar migrations: %v", err)
	}

	// Cache em memória
	memoryCache := cache.NewMemoryCache[string, string](cfg.CacheTTL)
	go memoryCache.StartJanitor()

	// Repos e Services
	clientRepo := repository.NewClientRepository(pool)
	clientService := service.NewClientService(clientRepo)

	// Gin
	r := gin.New()
	r.Use(gin.Recovery())
	if cfg.SlackWebhookURL != "" || (cfg.SlackBotToken != "" && cfg.SlackChannelID != "") {
		r.Use(router.RecoveryWithSlack(cfg.SlackWebhookURL, cfg.SlackBotToken, cfg.SlackChannelID, errorLogger))
	}
	// Registra logging de requests somente se não estivermos em nível 'error'
	if !strings.EqualFold(os.Getenv("LOG_LEVEL"), "error") {
		r.Use(router.RequestLogger())
	}

	// Registrar rotas
	router.Register(
		r,
		router.Dependencies{
			Config:      cfg,
			Cache:       memoryCache,
			HTTPClient:  httpClient,
			ClientSvc:   clientService,
			InfoLogger:  infoLogger,
			ErrorLogger: errorLogger,
		},
	)

	if !strings.EqualFold(os.Getenv("LOG_LEVEL"), "error") {
		infoLogger.Printf("Servidor iniciado. Porta %s", cfg.Port)
	}
	if err := r.Run(":" + cfg.Port); err != nil {
		errorLogger.Fatalf("erro ao iniciar servidor: %v", err)
	}
}

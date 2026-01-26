package cmd

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/labstack/echo-contrib/echoprometheus"
	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/tsingjyujing/vestigo/config"
	"github.com/tsingjyujing/vestigo/controller"
	"github.com/tsingjyujing/vestigo/models"
	"github.com/tsingjyujing/vestigo/utils"
	_ "modernc.org/sqlite"
)

var configFile string

func init() {
	serverCommand.Flags().StringVar(&configFile, "config", "", "Path to config file")
}

func readConfig() (*viper.Viper, *config.Envelope) {
	viperInstance := viper.New()
	viperInstance.SetConfigName("config")
	viperInstance.SetConfigType("yaml")
	viperInstance.AddConfigPath("/etc/vestigo/")
	viperInstance.AddConfigPath("$HOME/.vestigo")
	viperInstance.AddConfigPath("./config")
	viperInstance.SetEnvPrefix("VESTIGO")
	viperInstance.AutomaticEnv()
	err := viperInstance.ReadInConfig()
	if err != nil {
		logger.WithError(err).Fatal("fatal error config file")
	}
	logger.Infof("Using viperInstance file: %s", viperInstance.ConfigFileUsed())
	// Set default values
	viperInstance.SetDefault("server.address", ":8080")
	viperInstance.SetDefault("server.database", "db.sqlite")
	viperInstance.SetDefault("embedding_save_path", "./data/embed/")
	envelope, err := config.LoadConfigFromFile(viperInstance.ConfigFileUsed())
	if err != nil {
		logger.WithError(err).Fatal("Failed to parse configuration")
	}
	return viperInstance, envelope
}

var serverCommand = &cobra.Command{
	Use:   "server",
	Short: "Starting server",
	Run: func(cmd *cobra.Command, args []string) {
		echoServer := echo.New()
		goCtx := cmd.Context()
		viperInstance, config := readConfig()
		// Create controller
		db, err := sql.Open("sqlite", viperInstance.GetString("server.database"))
		if err != nil {
			logger.WithError(err).Fatal("Failed to open database")
		}
		// create tables
		if _, err := db.ExecContext(goCtx, controller.GetDDL()); err != nil {
			logger.WithError(err).Fatal("Failed to create tables")
		}
		embeddingModels := make(map[string]models.BaseEmbeddingModel)
		for _, modelConfig := range config.EmbeddingModels {
			model, err := models.LoadEmbeddingModel(modelConfig.Type, modelConfig.Config)
			if err != nil {
				logger.WithError(err).WithField("config", modelConfig).Fatalf("Failed to load embedding model: %s", modelConfig.ID)
			}
			if embeddingModels[modelConfig.ID] != nil {
				logger.Fatalf("Duplicate embedding model ID: %s", modelConfig.ID)
			}
			embeddingModels[modelConfig.ID] = model
			logger.Infof("Loaded embedding model %s successfully", modelConfig.ID)
		}

		generationModels := make(map[string]models.GenerationModel)
		if len(config.GenerationModels) > 0 {
			for _, modelConfig := range config.GenerationModels {
				model, err := models.NewGenerationModel(modelConfig.Type, modelConfig.Config)
				if err != nil {
					logger.WithError(err).WithField("config", modelConfig).Fatalf("Failed to load generation model: %s", modelConfig.ID)
				}
				if generationModels[modelConfig.ID] != nil {
					logger.Fatalf("Duplicate generation model ID: %s", modelConfig.ID)
				}
				generationModels[modelConfig.ID] = model
				logger.Infof("Loaded generation model %s successfully", modelConfig.ID)
			}
		}
		c, err := controller.NewController(db, embeddingModels, viperInstance.GetString("embedding_save_path"), generationModels)
		if err != nil {
			logger.WithError(err).Fatal("Failed to create controller")
		}

		echoServer.Use(echoprometheus.NewMiddleware("resman"))
		// Set routes
		echoServer.GET("/metrics", echoprometheus.NewHandler())
		echoServer.GET("/health", func(c *echo.Context) error {
			return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
		})
		echoServer.Use(middleware.CORS("*"))
		// RESTful API routes
		apiGroup := echoServer.Group("/api/v1")
		apiGroup.Use(middleware.RequestLogger())

		// Apply Bearer Token authentication if tokens are configured
		tokens := viperInstance.GetStringSlice("server.tokens")
		if len(tokens) > 0 {
			logger.Infof("Bearer token authentication enabled with %d token(s)", len(tokens))
			apiGroup.Use(utils.CreateBearerTokenMiddleware(tokens))
		} else {
			logger.Warn("Bearer token authentication disabled - no tokens configured")
		}

		// Document
		documentGroup := apiGroup.Group("/doc")
		documentGroup.POST("", c.NewDocument)
		documentGroup.GET("/:doc_id", c.GetDocument)
		documentGroup.DELETE("/:doc_id", c.DeleteDocument)
		documentGroup.POST("/:doc_id/text", c.NewTextChunk)

		// Text Chunk
		textGroup := apiGroup.Group("/text")
		textGroup.GET("/:text_id", c.GetTextChunk)
		textGroup.DELETE("/:text_id", c.DeleteTextChunk)

		// Query API
		apiGroup.GET("/models", c.ListEmbeddingModels)
		apiGroup.GET("/search/:model_id", c.Search)

		// Start server in a goroutine
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		httpServer := &http.Server{
			Addr:    viperInstance.GetString("server.address"),
			Handler: echoServer,
		}

		go func() {
			logger.Infof("Starting server at %s", httpServer.Addr)
			if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				logger.WithError(err).Error("failed to start server")
			}
		}()

		// Wait for interrupt signal to gracefully shutdown the server with a timeout
		<-ctx.Done()
		stop()
		logger.Info("Shutting down server gracefully, press Ctrl+C again to force")

		// Graceful shutdown with timeout
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			logger.WithError(err).Error("Server forced to shutdown")
		}

		// Close controller resources
		if err := c.Close(); err != nil {
			logger.WithError(err).Error("Failed to close controller")
		}

		logger.Info("Server stopped gracefully")
	},
}

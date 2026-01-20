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
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
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
	viperInstance.SetDefault("server.db", "db.sqlite")
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
		db, err := sql.Open("sqlite", viperInstance.GetString("server.db"))
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

		var summarizationModel models.SummarizationModel
		if config.SummarizationModel != nil {
			summarizationModel, err = models.NewSummarizationModel(config.SummarizationModel.Type, config.SummarizationModel.Config)
			if err != nil {
				logger.WithError(err).WithField("config", config.SummarizationModel).Fatalf("Failed to load summarization model: %s", config.SummarizationModel.ID)
			}
			logger.Infof("Loaded summarization model %s successfully", config.SummarizationModel.ID)
		} else {
			logger.Info("No summarization model configured")
		}
		c, err := controller.NewController(db, embeddingModels, viperInstance.GetString("embedding_save_path"), summarizationModel)
		if err != nil {
			logger.WithError(err).Fatal("Failed to create controller")
		}

		echoServer.Use(echoprometheus.NewMiddleware("resman"))
		// Set routes
		echoServer.GET("/metrics", echoprometheus.NewHandler())
		echoServer.GET("/health", func(c echo.Context) error {
			return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
		})
		echoServer.Use(middleware.CORS()) // Enable CORS for all origins

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
		documentGroup.POST("/", c.NewDocument)
		documentGroup.GET("/:doc_id", c.GetDocument)
		documentGroup.DELETE("/:doc_id", c.DeleteDocument)
		documentGroup.POST("/:doc_id/text", c.NewTextChunk)

		// Text Chunk
		textGroup := apiGroup.Group("/text")
		textGroup.GET("/:text_id", c.GetTextChunk)
		textGroup.DELETE("/:text_id", c.DeleteTextChunk)

		// Query API
		searchGroup := apiGroup.Group("/search")
		searchGroup.GET("/simple", c.SimpleSearch)
		searchGroup.GET("/ann/:model_id", c.ANNSearch)
		searchGroup.GET("/models", c.ListEmbeddingModels)

		// Start server in a goroutine
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()

		go func() {
			addr := viperInstance.GetString("server.address")
			logger.Infof("Starting server on %s", addr)
			if err := echoServer.Start(addr); err != nil && !errors.Is(err, http.ErrServerClosed) {
				logger.WithError(err).Error("Server start error")
			}
		}()

		// Wait for interrupt signal to gracefully shutdown the server with a timeout
		<-ctx.Done()
		stop()
		logger.Info("Shutting down server gracefully, press Ctrl+C again to force")

		// Graceful shutdown with timeout
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := echoServer.Shutdown(shutdownCtx); err != nil {
			logger.WithError(err).Error("Server forced to shutdown")
		}

		// Close controller resources
		if err := c.Close(); err != nil {
			logger.WithError(err).Error("Failed to close controller")
		}

		logger.Info("Server stopped gracefully")
	},
}

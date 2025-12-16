package cmd

import (
	"database/sql"
	"net/http"

	"github.com/labstack/echo-contrib/echoprometheus"
	"github.com/labstack/echo/v4"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/tsingjyujing/vestigo/controller"
	"github.com/tsingjyujing/vestigo/controller/dao"
	_ "modernc.org/sqlite"
)

var logger = logrus.New()

var configFile string

func init() {
	serverCommand.Flags().StringVar(&configFile, "config", "", "Path to config file")
}

func readConfig() *viper.Viper {
	config := viper.New()
	config.SetConfigName("config")
	config.SetConfigType("yaml")
	config.AddConfigPath("/etc/vestigo/")
	config.AddConfigPath("$HOME/.vestigo")
	config.AddConfigPath("./config")
	config.SetEnvPrefix("VESTIGO")
	config.AutomaticEnv()
	err := config.ReadInConfig()
	if err != nil {
		logger.WithError(err).Fatal("fatal error config file")
	}
	logger.Infof("Using config file: %s", config.ConfigFileUsed())
	// Set default values
	config.SetDefault("server.addr", ":8000")
	config.SetDefault("server.db", "db.sqlite")
	// Check necessary config values
	return config
}

var serverCommand = &cobra.Command{
	Use:   "server",
	Short: "Starting server",
	Run: func(cmd *cobra.Command, args []string) {
		echoServer := echo.New()
		goCtx := cmd.Context()
		config := readConfig()
		db, err := sql.Open("sqlite", config.GetString("server.db"))
		if err != nil {
			logger.WithError(err).Fatal("Failed to open database")
		}
		// create tables
		if _, err := db.ExecContext(goCtx, controller.GetDDL()); err != nil {
			logger.WithError(err).Fatal("Failed to create tables")
		}
		c := controller.New(*dao.New(db))
		echoServer.Use(echoprometheus.NewMiddleware("resman"))
		// Set routes
		echoServer.GET("/metrics", echoprometheus.NewHandler())
		echoServer.GET("/health", func(c echo.Context) error {
			return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
		})
		// RESTful
		apiGroup := echoServer.Group("/api/v1")

		datasourceGroup := apiGroup.Group("/datasource")
		documentGroup := apiGroup.Group("/doc")
		textGroup := apiGroup.Group("/text")

		datasourceGroup.GET("/:datasource_id", c.GetDatasource)
		datasourceGroup.POST("/", c.NewDatasource)
		datasourceGroup.DELETE("/:datasource_id", c.DeleteDatasource)
		datasourceGroup.POST("/:datasource_id/doc", c.NewDocument)
		// Document
		documentGroup.GET("/:doc_id", c.GetDocument)
		documentGroup.DELETE("/:doc_id", c.DeleteDocument)
		documentGroup.POST("/:doc_id/text", c.NewTextChunk)
		// Text Chunk
		textGroup.GET("/:text_id", c.GetTextChunk)
		textGroup.DELETE("/:text_id", c.DeleteTextChunk)

		echoServer.Logger.Fatal(echoServer.Start(config.GetString("server.addr")))
	},
}

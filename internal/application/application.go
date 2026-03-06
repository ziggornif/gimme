package application

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/gimme-cdn/gimme/api"
	"github.com/gimme-cdn/gimme/configs"
	"github.com/gimme-cdn/gimme/internal/auth"
	"github.com/gimme-cdn/gimme/internal/cache"
	"github.com/gimme-cdn/gimme/internal/content"
	gimmeerr "github.com/gimme-cdn/gimme/internal/errors"
	"github.com/gimme-cdn/gimme/internal/metrics"
	"github.com/gimme-cdn/gimme/internal/persistence"
	"github.com/gimme-cdn/gimme/internal/storage"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
)

type Application struct {
	config         *configs.Configuration
	authManager    *auth.AuthManager
	authProvider   auth.AuthProvider
	contentService content.ContentService
	storageManager storage.ObjectStorageManager
	cacheManager   cache.CacheManager
	tokenStore     auth.TokenStore
	redisClient    *persistence.RedisClient
	pgClient       *persistence.PGClient
}

// NewApplication create an application instance
func NewApplication() Application {
	return Application{}
}

// loadConfig load app config
func (app *Application) loadConfig() {
	var err *gimmeerr.GimmeError
	app.config, err = configs.NewConfig()
	if err != nil {
		log.Fatalln(err)
	}
}

// configureTokenStore
// Token store: file, redis or postgres, driven by tokenStore.mode (default: "file").
func (app *Application) configureTokenStore() {
	switch app.config.TokenStore.Mode {
	case "redis":
		app.tokenStore = auth.NewRedisTokenStore(app.redisClient)
		logrus.Info("[Application] loadModules - token store: Redis")
	case "postgres":
		pgClient, pgClientErr := persistence.NewPGClient(app.config.TokenStore.PostgresURL)
		if pgClientErr != nil {
			log.Fatalf("failed to connect to Postgres: %v", pgClientErr)
		}
		app.pgClient = pgClient

		pgStore, pgStoreErr := auth.NewPGTokenStore(app.pgClient.GetPool())
		if pgStoreErr != nil {
			log.Fatalf("failed to initialise PGTokenStore: %v", pgStoreErr)
		}
		app.tokenStore = pgStore
		logrus.Info("[Application] loadModules - token store: PostgreSQL")
	default: // "file"
		fileStore, fileErr := auth.NewFileTokenStore(app.config.Secret, app.config.TokenFile)
		if fileErr != nil {
			log.Fatalf("failed to initialise FileTokenStore at %q: %v", app.config.TokenFile, fileErr)
		}
		app.tokenStore = fileStore
		logrus.Infof("[Application] loadModules - token store: file (%s)", app.config.TokenFile)
	}
}

// configureAuthProvider
// Auth mode is oidc or basic by default
func (app *Application) configureAuthProvider() {
	switch app.config.Auth.Mode {
	case "oidc":
		oidcCtx, oidcCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer oidcCancel()
		// Derive a domain-separated signing secret for OIDC session cookies so that
		// API tokens (signed with app.config.Secret) cannot be replayed as session
		// cookies and vice versa.
		oidcSigningSecret := deriveSecret(app.config.Secret, "oidc-session")
		oidcProvider, oidcErr := auth.NewOIDCProvider(
			oidcCtx,
			app.config.Auth.OIDC.Issuer,
			app.config.Auth.OIDC.ClientID,
			app.config.Auth.OIDC.ClientSecret,
			app.config.Auth.OIDC.RedirectURL,
			oidcSigningSecret,
			app.config.Auth.OIDC.SecureCookies,
		)
		if oidcErr != nil {
			log.Fatalf("failed to initialise OIDC provider: %v", oidcErr)
		}
		app.authProvider = oidcProvider
	default: // "basic"
		app.authProvider = auth.NewBasicAuthProvider(app.config.AdminUser, app.config.AdminPassword)
	}
}

// loadModules load app modules
func (app *Application) loadModules() {
	var err *gimmeerr.GimmeError

	// Build a single shared Redis client if a Redis URL is configured.
	// It is used by any component that needs Redis (token store, cache).
	// validateConfig ensures redis_url is set whenever a Redis-backed
	// component is enabled, so no extra check is needed here.
	if app.config.RedisURL != "" {
		redisClient, redisErr := persistence.NewRedisClient(app.config.RedisURL)
		if redisErr != nil {
			log.Fatalf("failed to connect to Redis: %v", redisErr)
		}
		app.redisClient = redisClient
	}

	app.configureTokenStore()
	app.authManager = auth.NewAuthManager(app.tokenStore)
	app.configureAuthProvider()

	osmClient, err := storage.NewObjectStorageClient(app.config)
	if err != nil {
		log.Fatalln(err)
	}
	app.storageManager = storage.NewObjectStorageManager(osmClient)

	cacheTTL := time.Duration(app.config.Cache.TTL) * time.Second
	if app.config.Cache.Enabled {
		// cache.enabled=true implies cache.redis_url is set (validated in config).
		app.cacheManager = cache.NewRedisCache(app.redisClient)
	}

	app.contentService = content.NewContentService(app.storageManager, app.cacheManager, cacheTTL)

	err = app.storageManager.CreateBucket(context.Background(), app.config.S3BucketName, app.config.S3Location)
	if err != nil {
		log.Fatalln(err)
	}
}

func prometheusHandler() gin.HandlerFunc {
	h := promhttp.Handler()

	return func(c *gin.Context) {
		h.ServeHTTP(c.Writer, c.Request)
	}
}

// metricsMiddleware records HTTP request counts per route, method and status code.
// The /metrics endpoint itself is excluded to avoid a feedback loop where scraping
// inflates the counter, which then changes the metrics output on the next scrape.
func metricsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		route := c.FullPath()
		if route == "" {
			route = "unknown"
		}
		// Exclude the metrics endpoint from the counter.
		if route == "/metrics" {
			return
		}

		status := c.Writer.Status()
		// Status 0 can occur when the Recovery middleware catches a panic before
		// WriteHeader is called. Normalise to 500 to keep the label set finite.
		if status == 0 {
			status = http.StatusInternalServerError
		}

		metrics.HTTPRequestsTotal.WithLabelValues(
			route,
			c.Request.Method,
			strconv.Itoa(status),
		).Inc()
	}
}

// corsConfig returns a CORS configuration for the given list of allowed origins.
// If no origins are configured (empty slice), all cross-origin requests are allowed (*)
// — this is the default behaviour, suitable for a public CDN serving assets cross-origin.
// Set cors.allowed_origins in gimme.yml to a list of trusted origins to restrict access,
// or use ["*"] to explicitly allow all origins.
func corsConfig(allowedOrigins []string) cors.Config {
	cfg := cors.DefaultConfig()
	if len(allowedOrigins) > 0 {
		if len(allowedOrigins) == 1 && allowedOrigins[0] == "*" {
			cfg.AllowAllOrigins = true
		} else {
			cfg.AllowOrigins = allowedOrigins
		}
	} else {
		// No origins configured: default to allowing all origins.
		// Gimme is a public CDN — assets must be consumable cross-origin.
		// Operators can restrict to specific origins via cors.allowed_origins in gimme.yml.
		logrus.Info("[Application] setupServer - cors.allowed_origins is not configured: defaulting to allow all origins (*). Set cors.allowed_origins in gimme.yml to restrict cross-origin access.")
		cfg.AllowAllOrigins = true
	}
	return cfg
}

// deriveSecret produces a domain-separated key from a master secret and a label
// using HMAC-SHA256. This ensures that keys used for different purposes (e.g.
// API token signing vs. OIDC session cookies) are cryptographically independent
// even when they share the same master secret.
func deriveSecret(masterSecret, label string) string {
	mac := hmac.New(sha256.New, []byte(masterSecret))
	mac.Write([]byte(label))
	return hex.EncodeToString(mac.Sum(nil))
}

// loadHttpServer load http (go gin) server
func (app *Application) setupServer() {
	router := gin.Default()
	router.Use(cors.New(corsConfig(app.config.CORSAllowedOrigins)))
	router.Use(metricsMiddleware())
	router.Static("/docs", "./docs")
	router.Static("/assets", "./assets")
	router.SetFuncMap(api.TemplateFuncs())
	router.LoadHTMLGlob("templates/*.tmpl")

	app.authProvider.RegisterRoutes(router)

	api.NewRootController(router)
	api.NewAdminController(router, app.authManager, app.authProvider)
	api.NewPackageController(router, app.authManager, app.contentService)
	api.NewHealthController(router, app.storageManager)

	if app.config.EnableMetrics {
		router.GET("/metrics", prometheusHandler())
	}

	server := &http.Server{
		Addr:              fmt.Sprintf(":%s", app.config.AppPort),
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
	}

	quit := make(chan os.Signal, 1)

	go func() {
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logrus.Errorf("[Application] setupServer - ListenAndServe failed: %v", err)
			quit <- syscall.SIGTERM
		}
	}()
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	logrus.Info("Shutting down server...")

	// The context is used to inform the server it has 60 seconds to finish
	// any request it is currently handling before forcing a shutdown.
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		logrus.Fatal("Server forced to shutdown")
	}

	// In file mode, Close() stops the background purge goroutine.
	// In redis mode, Close() is a no-op — the client is owned by app and
	// closed below via app.redisClient.Close().
	// In postgres mode, Close() stops the background purge goroutine — the
	// pool is owned by app and closed below via app.pgPool.Close().
	if app.tokenStore != nil {
		app.tokenStore.Close()
	}

	// Close the shared Redis client. It is the single owner of the connection;
	// no other component calls Close() on it.
	if app.redisClient != nil {
		app.redisClient.CloseConnection()
	}

	// Close the PostgreSQL connection pool.
	if app.pgClient != nil {
		app.pgClient.CloseConnection()
	}

	logrus.Info("Server exiting.")
}

// Run - run application
func (app *Application) Run() {
	app.loadConfig()
	app.loadModules()
	app.setupServer()
}

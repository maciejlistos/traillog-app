package server

import (
	"context"
	"log/slog"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/maciejlistos/traillog/internal/buildinfo"
	"github.com/maciejlistos/traillog/internal/middleware"
	"github.com/maciejlistos/traillog/internal/webui"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	"go.opentelemetry.io/otel/attribute"
)

// HealthChecker reports whether a dependency is reachable.
type HealthChecker interface {
	PingContext(context.Context) error
}

// RouteRegistrar adds application routes to a Gin engine.
type RouteRegistrar interface {
	RegisterRoutes(*gin.Engine)
}

// NewRouter constructs the application's HTTP handler.
func NewRouter(
	logger *slog.Logger,
	checker HealthChecker,
	registrars ...RouteRegistrar,
) http.Handler {
	router := gin.New()

	router.Use(
		middleware.RequestID(nil),

		otelgin.Middleware(
			"traillog",
			otelgin.WithFilter(
				func(request *http.Request) bool {
					return request.URL.Path != "/health/live"
				},
			),
			otelgin.WithGinMetricAttributeFn(
				func(c *gin.Context) []attribute.KeyValue {
					return []attribute.KeyValue{
						attribute.Bool("traillog.request.htmx", strings.EqualFold(c.GetHeader("HX-Request"), "true")),
					}
				},
			),
		),

		middleware.RequestLogger(logger),
		middleware.Recovery(logger),
	)

	router.SetFuncMap(webui.FuncMap())
	router.LoadHTMLFS(webui.Templates(), "**/*.html")
	router.Static(
		"/static",
		"./web/static",
	)

	router.GET("/", homeHandler)
	router.GET("/version", versionHandler)
	router.GET("/health/live", livenessHandler)
	router.GET("/health/ready", readinessHandler(checker))

	for _, registrar := range registrars {
		registrar.RegisterRoutes(router)
	}

	return router
}

func homeHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"name":    "TrailLog",
		"message": "Hiking journal and route tracker",
	})
}

func livenessHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": "ok",
	})
}

func readinessHandler(checker HealthChecker) gin.HandlerFunc {
	return func(c *gin.Context) {
		if err := checker.PingContext(c.Request.Context()); err != nil {
			_ = c.Error(err)

			c.JSON(
				http.StatusServiceUnavailable,
				gin.H{
					"status": "not_ready",
				},
			)
			return
		}
		c.JSON(
			http.StatusOK,
			gin.H{
				"status": "ok",
			},
		)
	}
}

func versionHandler(c *gin.Context) {
	c.JSON(
		http.StatusOK,
		buildinfo.Current(),
	)
}

package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/juspay/hyperswitch-go-extensions/internal/forex"
	"github.com/juspay/hyperswitch-go-extensions/internal/locale"
	"github.com/juspay/hyperswitch-go-extensions/internal/wallets"
	"github.com/juspay/hyperswitch-go-extensions/pkg/models"
	"go.uber.org/zap"
)

func main() {
	// Initialize logger
	logger, _ := zap.NewProduction()
	defer logger.Sync()

	// Initialize services
	forexSvc := initForexService(logger)
	localeSvc, _ := locale.NewService()
	walletSvc := initWalletService(logger)

	// Initialize HTTP server
	router := setupRouter(logger, forexSvc, localeSvc, walletSvc)

	server := &http.Server{
		Addr:    ":8081",
		Handler: router,
	}

	// Start server in a goroutine
	go func() {
		logger.Info("Starting Hyperswitch Go Extensions server on :8081")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("Failed to start server", zap.Error(err))
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		logger.Fatal("Server forced to shutdown", zap.Error(err))
	}

	logger.Info("Server exited")
}

func initForexService(logger *zap.Logger) *forex.Service {
	provider := forex.NewMockProvider()
	config := forex.Config{
		CacheTTL:      1 * time.Hour,
		FeePercentage: 1.5,
		FallbackRates: map[string]float64{
			"USD-EUR": 0.85,
			"USD-GBP": 0.73,
			"USD-JPY": 110.50,
		},
	}

	return forex.NewService(provider, nil, logger, config)
}

func initWalletService(logger *zap.Logger) *wallets.Service {
	svc := wallets.NewService(logger)

	// Register mock providers
	svc.RegisterProvider(wallets.WalletTypeApplePay, wallets.NewMockProvider(wallets.WalletTypeApplePay, logger))
	svc.RegisterProvider(wallets.WalletTypeGooglePay, wallets.NewMockProvider(wallets.WalletTypeGooglePay, logger))
	svc.RegisterProvider(wallets.WalletTypePayPal, wallets.NewMockProvider(wallets.WalletTypePayPal, logger))

	return svc
}

func setupRouter(logger *zap.Logger, forexSvc *forex.Service, localeSvc *locale.Service, walletSvc *wallets.Service) *gin.Engine {
	router := gin.Default()

	// Health check
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "healthy"})
	})

	api := router.Group("/api/v1")
	{
		// Forex endpoints
		forex := api.Group("/forex")
		{
			forex.POST("/convert", func(c *gin.Context) {
				var req forex.ConversionRequest
				if err := c.ShouldBindJSON(&req); err != nil {
					c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
					return
				}

				resp, err := forexSvc.Convert(c.Request.Context(), req)
				if err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
					return
				}

				c.JSON(http.StatusOK, resp)
			})

			forex.GET("/supported-currencies", func(c *gin.Context) {
				currencies := forexSvc.GetSupportedCurrencies()
				c.JSON(http.StatusOK, gin.H{"currencies": currencies})
			})
		}

		// Locale endpoints
		localeEndpoints := api.Group("/locale")
		{
			localeEndpoints.GET("/supported", func(c *gin.Context) {
				locales := locale.GetSupportedLocales()
				c.JSON(http.StatusOK, gin.H{"locales": locales})
			})

			localeEndpoints.GET("/checkout/:locale", func(c *gin.Context) {
				localeStr := c.Param("locale")
				if !locale.ValidateLocale(localeStr) {
					c.JSON(http.StatusBadRequest, gin.H{"error": "unsupported locale"})
					return
				}

				content := localeSvc.GetCheckoutContent(locale.Locale(localeStr))
				c.JSON(http.StatusOK, content)
			})

			localeEndpoints.GET("/payment-methods/:locale", func(c *gin.Context) {
				localeStr := c.Param("locale")
				if !locale.ValidateLocale(localeStr) {
					c.JSON(http.StatusBadRequest, gin.H{"error": "unsupported locale"})
					return
				}

				methods := localeSvc.GetPreferredPaymentMethods(locale.Locale(localeStr))
				c.JSON(http.StatusOK, gin.H{"payment_methods": methods})
			})
		}

		// Wallet endpoints
		walletEndpoints := api.Group("/wallets")
		{
			walletEndpoints.POST("/pay", func(c *gin.Context) {
				var req wallets.WalletPaymentRequest
				if err := c.ShouldBindJSON(&req); err != nil {
					c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
					return
				}

				resp, err := walletSvc.ProcessPayment(c.Request.Context(), req)
				if err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
					return
				}

				c.JSON(http.StatusOK, resp)
			})

			walletEndpoints.GET("/supported", func(c *gin.Context) {
				region := c.DefaultQuery("region", "NA")
				currency := c.DefaultQuery("currency", "USD")

				supported := walletSvc.GetSupportedWallets(region, models.Currency(currency))
				c.JSON(http.StatusOK, gin.H{"wallets": supported})
			})
		}

		// Demo endpoints
		demo := api.Group("/demo")
		{
			demo.GET("/multi-currency-pricing", func(c *gin.Context) {
				baseAmount := models.NewMoney(100.00, models.USD)
				targetCurrencies := []models.Currency{
					models.EUR, models.GBP, models.JPY, models.INR,
				}

				pricing, err := forexSvc.GetMultiCurrencyPricing(c.Request.Context(), baseAmount, targetCurrencies)
				if err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
					return
				}

				c.JSON(http.StatusOK, pricing)
			})
		}
	}

	return router
}

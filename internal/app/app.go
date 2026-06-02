package app

import (
	"context"
	"order-service/config"
	"order-service/internal/adapter/handler"
	"order-service/internal/adapter/httpclient"
	messageproducer "order-service/internal/adapter/message/producer"
	"order-service/internal/adapter/repository"
	"order-service/internal/core/service"
	"os"
	"os/signal"
	"syscall"
	"time"

	middlewareGateway "order-service/internal/middleware"

	"github.com/gofiber/fiber/v3"
	fiberCors "github.com/gofiber/fiber/v3/middleware/cors"
	fiberRecover "github.com/gofiber/fiber/v3/middleware/recover"

	"github.com/rs/zerolog/log"
)

func RunServer() {
	cfg := config.NewConfig()

	config.NewLogger(cfg.App.AppEnv, cfg.App.LogLevel, cfg.App.AppName)

	db, err := cfg.ConnectionPostgres()
	if err != nil {
		log.Fatal().
			Err(err).
			Str("source", "internal.app.RunServer").
			Msg("failed connect postgres")
	}

	sqlDB, err := db.DB.DB()
	if err != nil {
		log.Fatal().
			Err(err).
			Str("source", "internal.app.RunServer").
			Msg("failed get sql db instance")
	}
	defer sqlDB.Close()

	redis, err := cfg.NewRedisClient()
	if err != nil {
		log.Fatal().
			Err(err).
			Str("source", "internal.app.RunServer").
			Msg("failed connect to redis")
	}
	defer redis.Close()

	opensearch, err := cfg.NewOpenSearch()
	if err != nil {
		log.Fatal().
			Err(err).
			Str("source", "internal.app.RunServer").
			Msg("failed connect to opensearch")
	}

	producer := cfg.NewKafkaProducer()

	var (
		orderDeleteProducer                    *messageproducer.OrderDeleteProducer
		sendEmailUpdateStatusOrderProducer     *messageproducer.EmailUpdateStatusProducer
		sendpushNotifUpdateStatusOrderProducer *messageproducer.NotifUpdateStatusProducer
		updateStatusOrderProducer              *messageproducer.UpdateStatusProducer
		updateProductStockProducer             *messageproducer.UpdateStockProducer
		orderPublishProducer                   *messageproducer.OrderPublishProducer
	)

	if producer != nil {
		orderDeleteProducer = messageproducer.NewOrderDeleteProducer(producer, cfg)
		sendEmailUpdateStatusOrderProducer = messageproducer.NewEmailUpdateStatusProducer(producer, cfg)
		sendpushNotifUpdateStatusOrderProducer = messageproducer.NewNotifUpdateStatusProducer(producer, cfg)
		updateStatusOrderProducer = messageproducer.NewUpdateStatusProducer(producer, cfg)
		updateProductStockProducer = messageproducer.NewUpdateStockProducer(producer, cfg)
		orderPublishProducer = messageproducer.NewOrderPublishProducer(producer, cfg)
	}

	httpClient := httpclient.NewClient(cfg)

	orderRepo := repository.NewOrderRepository(db.DB)
	elasticRepo := repository.NewElasticRepository(opensearch)

	jwtService := service.NewJwtService(cfg)
	orderService := service.NewOrderService(
		orderRepo,
		elasticRepo,
		cfg,
		httpClient,
		orderDeleteProducer,
		sendEmailUpdateStatusOrderProducer,
		sendpushNotifUpdateStatusOrderProducer,
		updateStatusOrderProducer,
		updateProductStockProducer,
		orderPublishProducer,
	)

	app := cfg.NewFiber()

	app.Use(fiberRecover.New())
	app.Use(fiberCors.New())
	app.Use(middlewareGateway.GatewayValidationMiddleware(cfg))

	app.Get("/api/check", func(c fiber.Ctx) error {
		return c.SendString("OK")
	})

	handler.NewOrderHandler(app, orderService, cfg, jwtService, redis)

	go func() {
		if cfg.App.AppPort == "" {
			cfg.App.AppPort = os.Getenv("APP_PORT")
		}

		port := ":" + cfg.App.AppPort

		log.Info().
			Str("port", port).
			Str("source", "internal.app.RunServer").
			Msg("server started")

		err = app.Listen(
			port,
			fiber.ListenConfig{
				EnablePrefork: cfg.App.WebPrefork,
			},
		)

		if err != nil {
			log.Fatal().
				Err(err).
				Str("source", "internal.app.RunServer").
				Msg("failed start server")
		}
	}()

	terminateSignals := make(chan os.Signal, 1)

	signal.Notify(
		terminateSignals,
		os.Interrupt,
		syscall.SIGTERM,
	)

	<-terminateSignals

	log.Info().
		Str("source", "internal.app.RunServer").
		Msg("shutting down server in 5 seconds")

	ctx, cancel := context.WithTimeout(
		context.Background(),
		5*time.Second,
	)
	defer cancel()

	if err := app.ShutdownWithContext(ctx); err != nil {
		log.Error().
			Err(err).
			Str("source", "internal.app.RunServer").
			Msg("failed shutdown server")
	}

	log.Info().
		Str("source", "internal.app.RunServer").
		Msg("server stopped gracefully")

}

package main

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	alertapi "github.com/qiniu/zeroops/internal/alerting/api"
	adb "github.com/qiniu/zeroops/internal/alerting/database"
	"github.com/qiniu/zeroops/internal/alerting/service/healthcheck"
	"github.com/qiniu/zeroops/internal/alerting/service/remediation"
	"github.com/qiniu/zeroops/internal/config"
	"github.com/qiniu/zeroops/internal/middleware"
	servicemanager "github.com/qiniu/zeroops/internal/service_manager"

	// releasesystem "github.com/qiniu/zeroops/internal/release_system/api"
	"strings"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func main() {
	// load config first
	log.Info().Msg("Starting zeroops api server")
	cfg, err := config.Load()
	if err != nil {
		log.Fatal().Err(err).Msg("failed to load config")
	}

	// configure log level from config
	switch strings.ToLower(cfg.Logging.Level) {
	case "trace":
		zerolog.SetGlobalLevel(zerolog.TraceLevel)
	case "debug":
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	case "warn", "warning":
		zerolog.SetGlobalLevel(zerolog.WarnLevel)
	case "error":
		zerolog.SetGlobalLevel(zerolog.ErrorLevel)
	default:
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	}

	serviceManagerSrv, err := servicemanager.NewServiceManagerServer(cfg)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create release system api")
	}
	defer func() {
		serviceManagerSrv.Close()
	}()

	// optional alerting DB for healthcheck and remediation
	var alertDB *adb.Database
	{
		dsn := func() string {
			return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
				cfg.Database.Host, cfg.Database.Port, cfg.Database.User, cfg.Database.Password, cfg.Database.DBName, cfg.Database.SSLMode)
		}()
		if db, derr := adb.New(dsn); derr == nil {
			alertDB = db
		} else {
			log.Error().Err(derr).Msg("healthcheck alerting DB init failed; scheduler/consumer will run without DB")
		}
	}

	// start healthcheck scheduler and remediation consumer
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// bootstrap alert rules from config if provided
	if err := healthcheck.BootstrapRulesFromConfigWithApp(ctx, alertDB, &cfg.Alerting.Ruleset); err != nil {
		log.Error().Err(err).Msg("bootstrap rules from config failed")
	}
	interval := parseDuration(cfg.Alerting.Healthcheck.Interval, 10*time.Second)
	batch := cfg.Alerting.Healthcheck.Batch
	workers := cfg.Alerting.Healthcheck.Workers
	if workers < 1 {
		workers = 1
	}
	alertChSize := cfg.Alerting.Healthcheck.AlertChanSize
	alertCh := make(chan healthcheck.AlertMessage, alertChSize)

	for i := 0; i < workers; i++ {
		go healthcheck.StartScheduler(ctx, healthcheck.Deps{
			DB:       alertDB,
			Redis:    healthcheck.NewRedisClientFromConfig(&cfg.Redis),
			AlertCh:  alertCh,
			Batch:    batch,
			Interval: interval,
		})
	}
	rem := remediation.NewConsumer(alertDB, healthcheck.NewRedisClientFromConfig(&cfg.Redis)).WithConfig(&cfg.Alerting.Remediation)
	go rem.Start(ctx, alertCh)

	// start Prometheus anomaly detection scheduler
	promInterval := parseDuration(cfg.Alerting.Prometheus.SchedulerInterval, 5*time.Minute)
	promStep := parseDuration(cfg.Alerting.Prometheus.QueryStep, time.Minute)
	promRange := parseDuration(cfg.Alerting.Prometheus.QueryRange, 6*time.Hour)
	promCfg := healthcheck.NewPrometheusConfigFromApp(&cfg.Alerting.Prometheus)
	anomalyDetectClient := healthcheck.NewAnomalyDetectClient(promCfg)
	go healthcheck.StartPrometheusScheduler(ctx, healthcheck.PrometheusDeps{
		DB:                  alertDB,
		AnomalyDetectClient: anomalyDetectClient,
		Interval:            promInterval,
		QueryStep:           promStep,
		QueryRange:          promRange,
		RulesetBase:         cfg.Alerting.Ruleset.APIBase,
		RulesetTimeout:      parseDuration(cfg.Alerting.Ruleset.APITimeout, 10*time.Second),
	})

	router := gin.New()
	router.Use(gin.Logger())
	router.Use(gin.Recovery())
	router.Use(middleware.Authentication)
	alertapi.NewApiWithConfig(router, cfg)
	if err := serviceManagerSrv.UseApi(router); err != nil {
		log.Fatal().Err(err).Msg("bind serviceManagerApi failed.")
	}
	log.Info().Msgf("Starting server on %s", cfg.Server.BindAddr)
	if err := router.Run(cfg.Server.BindAddr); err != nil {
		log.Fatal().Err(err).Msg("start zeroops api server failed.")
	}
	log.Info().Msg("zeroops api server exit...")
}

func parseDuration(s string, d time.Duration) time.Duration {
	if s == "" {
		return d
	}
	if v, err := time.ParseDuration(s); err == nil {
		return v
	}
	return d
}

func parseInt(s string, v int) int {
	if s == "" {
		return v
	}
	if n, err := strconv.Atoi(s); err == nil {
		return n
	}
	return v
}

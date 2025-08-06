package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"
	"time"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"

	"github.com/go-logr/logr"
	"github.com/krateoplatformops/git-provider/apis"
	"github.com/krateoplatformops/plumbing/env"
	prettylog "github.com/krateoplatformops/plumbing/slogs/pretty"
	"github.com/krateoplatformops/provider-runtime/pkg/logging"
	"github.com/krateoplatformops/provider-runtime/pkg/ratelimiter"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	github "github.com/krateoplatformops/git-provider/internal/controllers"
	"github.com/krateoplatformops/provider-runtime/pkg/controller"

	"github.com/stoewer/go-strcase"
)

const (
	providerName = "Git"
)

func main() {
	envVarPrefix := fmt.Sprintf("%s_PROVIDER", strcase.UpperSnakeCase(providerName))

	debug := flag.Bool("debug", env.Bool(fmt.Sprintf("%s_DEBUG", envVarPrefix), false), "Run with debug logging.")
	syncPeriod := flag.Duration("sync", env.Duration(fmt.Sprintf("%s_SYNC_PERIOD", envVarPrefix), time.Hour), "Controller manager sync period such as 300ms, 1.5h, or 2h45m")
	pollInterval := flag.Duration("poll", env.Duration(fmt.Sprintf("%s_POLL_INTERVAL", envVarPrefix), 2*time.Minute), "Poll interval controls how often an individual resource should be checked for drift.")
	maxReconcileRate := flag.Int("max-reconcile-rate", env.Int(fmt.Sprintf("%s_MAX_RECONCILE_RATE", envVarPrefix), 5), "The number of concurrent reconciles for each controller. This is the maximum number of resources that can be reconciled at the same time.")
	leaderElection := flag.Bool("leader-election", env.Bool(fmt.Sprintf("%s_LEADER_ELECTION", envVarPrefix), false), "Use leader election for the controller manager.")
	maxErrorRetryInterval := flag.Duration("max-error-retry-interval", env.Duration(fmt.Sprintf("%s_MAX_ERROR_RETRY_INTERVAL", envVarPrefix), 1*time.Minute), "The maximum interval between retries when an error occurs. This should be less than the half of the poll interval.")
	minErrorRetryInterval := flag.Duration("min-error-retry-interval", env.Duration(fmt.Sprintf("%s_MIN_ERROR_RETRY_INTERVAL", envVarPrefix), 1*time.Second), "The minimum interval between retries when an error occurs. This should be less than max-error-retry-interval.")
	flag.Parse()

	// var zapOptions []zap.Opts
	// if *debug {
	// 	// Debug mode: mostra DEBUG, INFO, WARN, ERROR
	// 	zapOptions = []zap.Opts{
	// 		zap.UseDevMode(true),
	// 		zap.Level(zapcore.DebugLevel),
	// 	}
	// } else {
	// 	// Production mode: mostra solo INFO, WARN, ERROR
	// 	zapOptions = []zap.Opts{
	// 		zap.UseDevMode(false),
	// 		zap.Level(zapcore.InfoLevel),
	// 	}
	// }

	// zl := zap.New(zapOptions...)

	// log := logging.NewLogrLogger(zl.WithName(fmt.Sprintf("%s-provider", strcase.KebabCase(providerName))))
	// ctrl.SetLogger(zl)

	logLevel := slog.LevelInfo
	if *debug {
		logLevel = slog.LevelDebug
	}

	lh := prettylog.New(&slog.HandlerOptions{
		Level:     logLevel,
		AddSource: false,
	},
		prettylog.WithDestinationWriter(os.Stderr),
		prettylog.WithColor(),
		prettylog.WithOutputEmptyAttrs(),
	)

	logrlog := logr.FromSlogHandler(slog.New(lh).Handler())
	log := logging.NewLogrLogger(logrlog)

	log.Info("Starting", "sync-period", syncPeriod.String())

	cfg, err := ctrl.GetConfig()
	if err != nil {
		log.Info("Cannot get API server rest config, trying in-cluster config", "error", err)
		os.Exit(1)
	}

	ctrl.SetLogger(logrlog)

	mgr, err := ctrl.NewManager(cfg, ctrl.Options{
		LeaderElection:   *leaderElection,
		LeaderElectionID: fmt.Sprintf("leader-election-%s-provider", strcase.KebabCase(providerName)),
		Cache: cache.Options{
			SyncPeriod: syncPeriod,
		},
		Metrics: metricsserver.Options{
			BindAddress: ":8080",
		},
	})
	if err != nil {
		log.Info("Trying to start metrics server", "error", err)
		os.Exit(1)
	}

	o := controller.Options{
		Logger:                  log,
		MaxConcurrentReconciles: *maxReconcileRate,
		PollInterval:            *pollInterval,
		GlobalRateLimiter:       ratelimiter.NewGlobalExponential(*minErrorRetryInterval, *maxErrorRetryInterval),
	}

	if err := apis.AddToScheme(mgr.GetScheme()); err != nil {
		log.Info("Cannot add APIs to scheme", "error", err)
		os.Exit(1)
	}
	if err := github.Setup(mgr, o); err != nil {
		log.Info("Cannot setup controllers", "error", err)
		os.Exit(1)
	}
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		log.Info("Cannot start controller manager", "error", err)
		os.Exit(1)
	}
}

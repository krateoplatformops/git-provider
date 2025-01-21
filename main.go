package main

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"time"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/krateoplatformops/git-provider/apis"
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

	debug := flag.Bool("debug", getEnvBool(fmt.Sprintf("%s_DEBUG", envVarPrefix), false), "Run with debug logging.")
	syncPeriod := flag.Duration("sync", getEnvDuration(fmt.Sprintf("%s_SYNC_PERIOD", envVarPrefix), time.Hour), "Controller manager sync period such as 300ms, 1.5h, or 2h45m")
	pollInterval := flag.Duration("poll", getEnvDuration(fmt.Sprintf("%s_POLL_INTERVAL", envVarPrefix), 2*time.Minute), "Poll interval controls how often an individual resource should be checked for drift.")
	maxReconcileRate := flag.Int("max-reconcile-rate", getEnvInt(fmt.Sprintf("%s_MAX_RECONCILE_RATE", envVarPrefix), 5), "The global maximum rate per second at which resources may be checked for drift from the desired state.")
	leaderElection := flag.Bool("leader-election", getEnvBool(fmt.Sprintf("%s_LEADER_ELECTION", envVarPrefix), false), "Use leader election for the controller manager.")

	flag.Parse()

	zl := zap.New(zap.UseDevMode(*debug))
	log := logging.NewLogrLogger(zl.WithName(fmt.Sprintf("%s-provider", strcase.KebabCase(providerName))))
	if *debug {
		ctrl.SetLogger(zl)
	}

	log.Debug("Starting", "sync-period", syncPeriod.String())

	cfg, err := ctrl.GetConfig()
	if err != nil {
		log.Info("Cannot get API server rest config, trying in-cluster config", "error", err)
		os.Exit(1)
	}

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
		GlobalRateLimiter:       ratelimiter.NewGlobal(*maxReconcileRate),
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

func getEnvBool(env string, defaultVal bool) bool {
	if val, ok := os.LookupEnv(env); ok {
		return val == "true"
	}
	return defaultVal
}

func getEnvDuration(env string, defaultVal time.Duration) time.Duration {
	if val, ok := os.LookupEnv(env); ok {
		if duration, err := time.ParseDuration(val); err == nil {
			return duration
		}
	}
	return defaultVal
}

func getEnvInt(env string, defaultVal int) int {
	if val, ok := os.LookupEnv(env); ok {
		if intVal, err := strconv.Atoi(val); err == nil {
			return intVal
		}
	}
	return defaultVal
}

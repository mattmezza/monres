package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/mattmezza/monres/internal/alerter"
	"github.com/mattmezza/monres/internal/collector"
	"github.com/mattmezza/monres/internal/config"
	"github.com/mattmezza/monres/internal/history"
	"github.com/mattmezza/monres/internal/notifier"
)

var configFile string

func init() {
	flag.StringVar(&configFile, "config", "config.yaml", "Path to the configuration file.")
	// Set up logger
	log.SetOutput(os.Stdout) // Systemd will capture this
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
}

func main() {
	flag.Parse()
	log.Println("Starting monres...")

	cfg, err := config.LoadConfig(configFile)
	if err != nil {
		log.Fatalf("FATAL: Failed to load configuration from %s: %v", configFile, err)
	}
	log.Printf("Configuration loaded successfully from %s. Interval: %ds, Hostname: %s",
            configFile, cfg.IntervalSeconds, cfg.EffectiveHostname)


	// Initialize Metric History Buffer
	// Determine max history needed based on alert rule durations
	maxHistDuration := history.GetMaxConfiguredDuration(cfg.Alerts, cfg.CollectionInterval)
	if maxHistDuration == 0 && len(cfg.Alerts) > 0 { // No duration specified in any rule, but alerts exist
	    // Need some minimal history for instantaneous alerts if they rely on the buffer
	    // e.g. to hold at least the last 2 samples for any rate calculations or just the last sample.
	    // If GetMaxConfiguredDuration returns 0 because no rule has a duration > 0,
	    // we still need a buffer that can hold at least one, preferably a few, data points.
	    // The NewMetricHistoryBuffer has a minimum size logic.
        log.Printf("No explicit durations in alerts, using default history buffer capacity (based on 2x collection interval).")
	} else {
        log.Printf("Initializing metric history buffer for max duration: %s (collection interval: %s)", maxHistDuration, cfg.CollectionInterval)
    }
	metricHist := history.NewMetricHistoryBuffer(maxHistDuration, cfg.CollectionInterval)


	// Initialize Metric Collectors
	metricCollector := collector.NewGlobalCollector()
	log.Println("Metric collectors initialized.")

	// Initialize Notifiers
	configuredNotifiers, err := notifier.InitializeNotifiers(cfg.NotificationChannels)
	if err != nil {
		log.Fatalf("FATAL: Failed to initialize notifiers: %v", err)
	}
	if len(configuredNotifiers) == 0 && len(cfg.Alerts) > 0 {
        log.Println("Warning: Alerts are configured, but no notification channels were successfully initialized.")
    } else {
        log.Printf("%d notification channel(s) initialized.", len(configuredNotifiers))
    }


	// Initialize Alerter (loads initial state itself)
	alertProcessor, err := alerter.NewAlerter(cfg, metricHist, configuredNotifiers)
	if err != nil {
		log.Fatalf("FATAL: Failed to initialize alerter: %v", err)
	}
	log.Println("Alerter initialized. Loaded initial alert states.")

	// Setup Graceful Shutdown
	shutdownSignal := make(chan os.Signal, 1)
	signal.Notify(shutdownSignal, syscall.SIGINT, syscall.SIGTERM)

	// Main Application Loop
	ticker := time.NewTicker(cfg.CollectionInterval)
	defer ticker.Stop()

	log.Println("monres started. Monitoring resources...")

	// Initial collection to populate previous values for rate calculations
	// This will mean the first set of rates might be 0 or based on a very short interval if run immediately.
	// The GlobalCollector handles this by returning 0 for rates on the first pass.
	log.Println("Performing initial metric collection...")
	initialMetrics, err := metricCollector.CollectAll()
	if err != nil {
		log.Printf("Warning: Error during initial metric collection: %v", err)
	} else {
		now := time.Now()
		for name, val := range initialMetrics {
			metricHist.AddDataPoint(name, val, now)
		}
		log.Printf("Initial metrics collected. %d data points added to history.", len(initialMetrics))
		// Run alerter once after initial collection to catch immediate state changes for non-duration alerts.
        // This is important if an alert condition is met by the very first data sample.
		log.Println("Performing initial alert evaluation pass...")
		alertProcessor.CheckAndNotify(now, initialMetrics)
        log.Println("Initial alert evaluation complete.")
	}


	for {
		select {
		case <-ticker.C:
			currentTime := time.Now()
			log.Println("Collection cycle triggered.")

			collectedData, err := metricCollector.CollectAll()
			if err != nil {
				log.Printf("Error during metric collection cycle: %v", err)
				// Continue, try next cycle. Some metrics might have been collected.
			}
			if len(collectedData) == 0 && err == nil {
				log.Println("No metrics collected in this cycle.")
			} else {
                 log.Printf("Collected %d metrics. Adding to history.", len(collectedData))
            }


			for name, value := range collectedData {
				metricHist.AddDataPoint(name, value, currentTime)
				// if you want to debug or log each metric value:
				// log.Printf("Metric %s: %v", name, value)
			}

			alertProcessor.CheckAndNotify(currentTime, collectedData)

		case sig := <-shutdownSignal:
			log.Printf("Received signal: %s. Shutting down gracefully...", sig)
			// Perform any necessary cleanup here
			log.Println("monres shut down.")
			return
		}
	}
}

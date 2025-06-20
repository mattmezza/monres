package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"strings"
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

func testNotification(configPath, channelName string) {
	log.Println("Testing notification channels...")
	
	// Load configuration
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		log.Fatalf("FATAL: Failed to load configuration from %s: %v", configPath, err)
	}
	
	// Check if specific channel exists in config
	if channelName != "" {
		found := false
		for _, channel := range cfg.NotificationChannels {
			if channel.Name == channelName {
				found = true
				break
			}
		}
		if !found {
			// List available channels for user guidance
			var availableChannels []string
			for _, channel := range cfg.NotificationChannels {
				availableChannels = append(availableChannels, channel.Name)
			}
			if len(availableChannels) > 0 {
				log.Fatalf("ERROR: Channel '%s' not found in configuration. Available channels: %s", 
					channelName, strings.Join(availableChannels, ", "))
			} else {
				log.Fatalf("ERROR: Channel '%s' not found and no notification channels configured", channelName)
			}
		}
	}
	
	// Initialize notifiers
	configuredNotifiers, err := notifier.InitializeNotifiers(cfg.NotificationChannels)
	if err != nil {
		log.Fatalf("FATAL: Failed to initialize notifiers: %v", err)
	}
	
	if len(configuredNotifiers) == 0 {
		log.Fatalf("ERROR: No notification channels were successfully initialized")
	}
	
	// Create test notification data
	testData := notifier.NotificationData{
		AlertName:      "Test Alert",
		MetricName:     "test_metric",
		MetricValue:    42.5,
		ThresholdValue: 40.0,
		Condition:      ">",
		State:          "FIRED",
		Hostname:       cfg.EffectiveHostname,
		Time:           time.Now(),
		DurationString: "1m",
		Aggregation:    "average",
	}
	
	templates := notifier.NotificationTemplates{
		FiredTemplate:    cfg.Templates.AlertFired,
		ResolvedTemplate: cfg.Templates.AlertResolved,
	}
	
	// Test specific channel or all channels
	if channelName != "" {
		// Test specific channel
		if notifierInstance, exists := configuredNotifiers[channelName]; exists {
			log.Printf("Testing notification channel: %s", channelName)
			err := notifierInstance.Send(testData, templates)
			if err != nil {
				log.Fatalf("ERROR: Failed to send test notification to channel '%s': %v", channelName, err)
			}
			log.Printf("✅ Test notification sent successfully to channel: %s", channelName)
		} else {
			log.Fatalf("ERROR: Channel '%s' was not successfully initialized", channelName)
		}
	} else {
		// Test all channels
		log.Printf("Testing all %d configured notification channels...", len(configuredNotifiers))
		successCount := 0
		for name, notifierInstance := range configuredNotifiers {
			log.Printf("Testing channel: %s", name)
			err := notifierInstance.Send(testData, templates)
			if err != nil {
				log.Printf("❌ Failed to send test notification to channel '%s': %v", name, err)
			} else {
				log.Printf("✅ Test notification sent successfully to channel: %s", name)
				successCount++
			}
		}
		log.Printf("Test completed: %d/%d channels successful", successCount, len(configuredNotifiers))
		if successCount == 0 {
			log.Fatalf("ERROR: All notification channels failed")
		}
	}
}

func main() {
	flag.Parse()
	
	// Check if test-notification subcommand is provided
	args := flag.Args()
	if len(args) > 0 && args[0] == "test-notification" {
		var channelName string
		if len(args) > 1 {
			channelName = args[1]
		}
		testNotification(configFile, channelName)
		return
	}
	
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
			collectedData, err := metricCollector.CollectAll()
			if err != nil {
				log.Printf("Error during metric collection cycle: %v", err)
				// Continue, try next cycle. Some metrics might have been collected.
			}
			if len(collectedData) == 0 && err == nil {
				log.Println("No metrics collected in this cycle.")
			} else {
                 log.Printf("%d metrics added to history.", len(collectedData))
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

package main

import (
	"os"
	"runtime"
	"testing"
)

// TestLoadConfig_Defaults: with no env set, every knob falls back to its
// documented default. This catches the case where someone renames a const
// without updating loadConfig.
func TestLoadConfig_Defaults(t *testing.T) {
	for _, k := range []string{
		"IMS_HTTP_ADDR", "IMS_QUEUE_CAPACITY", "IMS_WORKER_COUNT",
		"IMS_RATE_LIMIT_RPS", "IMS_RATE_LIMIT_BURST",
		"IMS_METRICS_INTERVAL", "IMS_SHUTDOWN_TIMEOUT",
	} {
		t.Setenv(k, "")
	}
	c := loadConfig()
	if c.httpAddr != defaultHTTPAddr {
		t.Errorf("httpAddr: want %q, got %q", defaultHTTPAddr, c.httpAddr)
	}
	if c.queueCapacity != defaultQueueCapacity {
		t.Errorf("queueCapacity: want %d, got %d", defaultQueueCapacity, c.queueCapacity)
	}
	if c.workerCount != runtime.NumCPU()*2 {
		t.Errorf("workerCount: want %d, got %d", runtime.NumCPU()*2, c.workerCount)
	}
}

// TestLoadConfig_Overrides: env vars win over defaults; bad values fall
// back to defaults with a log line (not tested) rather than crashing.
func TestLoadConfig_Overrides(t *testing.T) {
	t.Setenv("IMS_HTTP_ADDR", ":9090")
	t.Setenv("IMS_QUEUE_CAPACITY", "1000")
	t.Setenv("IMS_WORKER_COUNT", "4")
	t.Setenv("IMS_RATE_LIMIT_RPS", "500.5")
	t.Setenv("IMS_METRICS_INTERVAL", "1s")
	c := loadConfig()
	if c.httpAddr != ":9090" {
		t.Errorf("httpAddr override broken: %q", c.httpAddr)
	}
	if c.queueCapacity != 1000 {
		t.Errorf("queueCapacity override broken: %d", c.queueCapacity)
	}
	if c.workerCount != 4 {
		t.Errorf("workerCount override broken: %d", c.workerCount)
	}
	if c.rateLimitRPS != 500.5 {
		t.Errorf("rateLimitRPS override broken: %v", c.rateLimitRPS)
	}
	if c.metricsInterval.String() != "1s" {
		t.Errorf("metricsInterval override broken: %v", c.metricsInterval)
	}
}

// TestLoadConfig_BadValueFallsBack: a non-numeric env var doesn't crash;
// the documented default kicks in.
func TestLoadConfig_BadValueFallsBack(t *testing.T) {
	t.Setenv("IMS_QUEUE_CAPACITY", "not-a-number")
	c := loadConfig()
	if c.queueCapacity != defaultQueueCapacity {
		t.Errorf("bad value should fall back to default, got %d", c.queueCapacity)
	}
}

func TestMain(m *testing.M) {
	// Defensive: prevent tests from inheriting a wonky env from the dev
	// machine. t.Setenv inside each test scopes correctly anyway, but
	// this guards against forgotten resets.
	for _, k := range []string{
		"IMS_HTTP_ADDR", "IMS_QUEUE_CAPACITY", "IMS_WORKER_COUNT",
		"IMS_RATE_LIMIT_RPS", "IMS_RATE_LIMIT_BURST",
		"IMS_METRICS_INTERVAL", "IMS_SHUTDOWN_TIMEOUT",
	} {
		_ = os.Unsetenv(k)
	}
	os.Exit(m.Run())
}

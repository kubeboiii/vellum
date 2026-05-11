package main

import (
	"os"
	"runtime"
	"testing"
)

func TestLoadConfig_Defaults(t *testing.T) {
	for _, k := range []string{
		"VELLUM_HTTP_ADDR", "VELLUM_QUEUE_CAPACITY", "VELLUM_WORKER_COUNT",
		"VELLUM_RATE_LIMIT_RPS", "VELLUM_RATE_LIMIT_BURST",
		"VELLUM_METRICS_INTERVAL", "VELLUM_SHUTDOWN_TIMEOUT",
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

func TestLoadConfig_Overrides(t *testing.T) {
	t.Setenv("VELLUM_HTTP_ADDR", ":9090")
	t.Setenv("VELLUM_QUEUE_CAPACITY", "1000")
	t.Setenv("VELLUM_WORKER_COUNT", "4")
	t.Setenv("VELLUM_RATE_LIMIT_RPS", "500.5")
	t.Setenv("VELLUM_METRICS_INTERVAL", "1s")
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

func TestLoadConfig_BadValueFallsBack(t *testing.T) {
	t.Setenv("VELLUM_QUEUE_CAPACITY", "not-a-number")
	c := loadConfig()
	if c.queueCapacity != defaultQueueCapacity {
		t.Errorf("bad value should fall back to default, got %d", c.queueCapacity)
	}
}

func TestMain(m *testing.M) {

	for _, k := range []string{
		"VELLUM_HTTP_ADDR", "VELLUM_QUEUE_CAPACITY", "VELLUM_WORKER_COUNT",
		"VELLUM_RATE_LIMIT_RPS", "VELLUM_RATE_LIMIT_BURST",
		"VELLUM_METRICS_INTERVAL", "VELLUM_SHUTDOWN_TIMEOUT",
	} {
		_ = os.Unsetenv(k)
	}
	os.Exit(m.Run())
}

package main

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/push"
)

var (
	gpuUsagePercentage = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "gpu_usage_percentage",
		Help: "Current GPU usage percentage",
	}, []string{"index", "model", "project_id", "project_name"})

	gpuMemoryUsage = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "gpu_memory_usage",
		Help: "Current GPU Memory usage",
	}, []string{"index", "model", "project_id", "project_name"})

	gpuMemoryTotal = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "gpu_memory_total",
		Help: "Current GPU Memory total",
	}, []string{"index", "model", "project_id", "project_name"})

	gpuTemperature = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "gpu_temperature",
		Help: "Current GPU temperature",
	}, []string{"index", "model", "project_id", "project_name"})

	gpuPowerUsage = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "gpu_power_usage",
		Help: "Current GPU Power usage",
	}, []string{"index", "model", "project_id", "project_name"})

	gpuPowerLimit = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "gpu_power_limit",
		Help: "Current GPU Power limit",
	}, []string{"index", "model", "project_id", "project_name"})
)

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

func gpuSmiHandler() {
	projectID := getEnv("project_id", "unknown")
	projectName := getEnv("project_name", "unknown")
	instanceName := getEnv("instance_name", "unknown")
	pushgatewayURL := getEnv("pushgateway_url", "http://150.183.252.204:9091")

	out, err := exec.Command("nvidia-smi", "--format=csv,noheader,nounits",
		"--query-gpu=index,name,utilization.gpu,memory.used,memory.total,temperature.gpu,power.draw,power.limit").Output()
	if err != nil {
		fmt.Fprintf(os.Stderr, "nvidia-smi error: %v\n", err)
		return
	}

	registry := prometheus.NewRegistry()
	registry.MustRegister(gpuUsagePercentage, gpuMemoryUsage, gpuMemoryTotal, gpuTemperature, gpuPowerUsage, gpuPowerLimit)

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	for _, line := range lines {
		fields := strings.Split(line, ", ")
		if len(fields) < 8 {
			continue
		}
		index := strings.TrimSpace(fields[0])
		model := strings.TrimSpace(fields[1])
		usage, _ := strconv.ParseFloat(strings.TrimSpace(fields[2]), 64)
		memUsed, _ := strconv.ParseFloat(strings.TrimSpace(fields[3]), 64)
		memTotal, _ := strconv.ParseFloat(strings.TrimSpace(fields[4]), 64)
		temp, _ := strconv.ParseFloat(strings.TrimSpace(fields[5]), 64)
		powerDraw, _ := strconv.ParseFloat(strings.TrimSpace(fields[6]), 64)
		powerLimit, _ := strconv.ParseFloat(strings.TrimSpace(fields[7]), 64)

		labels := prometheus.Labels{
			"index": index, "model": model,
			"project_id": projectID, "project_name": projectName,
		}
		gpuUsagePercentage.With(labels).Set(usage)
		gpuMemoryUsage.With(labels).Set(memUsed)
		gpuMemoryTotal.With(labels).Set(memTotal)
		gpuTemperature.With(labels).Set(temp)
		gpuPowerUsage.With(labels).Set(powerDraw)
		gpuPowerLimit.With(labels).Set(powerLimit)
	}

	pusher := push.New(pushgatewayURL, "gpu_usage_job").
		Grouping("instance", instanceName).
		Gatherer(registry)

	if err := pusher.Push(); err != nil {
		fmt.Fprintf(os.Stderr, "Could not push completion time to Pushgateway: %v\n", err)
	}
}

func scheduleEveryTwoMinutes(f func()) {
	for {
		f()
		time.Sleep(2 * time.Minute)
	}
}

func main() {
	scheduleEveryTwoMinutes(gpuSmiHandler)
}

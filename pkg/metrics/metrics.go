package metrics

import "github.com/penglongli/gin-metrics/ginmetrics"

func GetMonitor(path string) *ginmetrics.Monitor {
	m := ginmetrics.GetMonitor()
	// +optional set path
	m.SetMetricPath(path)
	// +optional set slow time
	m.SetSlowTime(1)

	// +optional set request duration, default {0.1, 0.3, 1.2, 5, 10}
	// used to p95, p99

	m.SetDuration([]float64{0.05, 0.1, 0.2, 0.3, 0.5, 1, 2, 5})

	// customize metrics
	// gaugeMetric := &ginmetrics.Metric{
	// 	Type:        ginmetrics.Gauge,
	// 	Name:        "example_gauge_metric",
	// 	Description: "an example of gauge type metric",
	// 	Labels:      []string{"label1"},
	// }

	// m.AddMetric(gaugeMetric)

	return m
}

package awsds

import (
	"github.com/prometheus/client_golang/prometheus"
)

var AWSSessionCreatedHistogram = prometheus.NewHistogramVec(prometheus.HistogramOpts{
	Namespace: "grafana",
	Subsystem: "aws_datasources",
	Name:      "aws_session_created_duration_seconds",
	Help:      "histogram of grafana aws datasource session creation duration in seconds",
	Buckets:   []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10, 25, 50, 100},
}, []string{"auth_type"})

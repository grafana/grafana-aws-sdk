package cloudWatchConsts

import (
	"fmt"
	"slices"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
)

// test to check NamespaceMetricsMap is sorted alphabetically
func TestNamespaceMetricsMap(t *testing.T) {
	unsortedMetricNamespaces := namespacesWithUnsortedMetrics(NamespaceMetricsMap)
	if len(unsortedMetricNamespaces) != 0 {
		assert.Fail(t, "NamespaceMetricsMap is not sorted alphabetically. Please replace the printed services")
		printNamespacesThatNeedSorted(unsortedMetricNamespaces)
	}
}

func printNamespacesThatNeedSorted(unsortedMetricNamespaces []string) {
	slices.Sort(unsortedMetricNamespaces)

	for _, namespace := range unsortedMetricNamespaces {
		metrics := NamespaceMetricsMap[namespace]
		slices.Sort(metrics)
		uniqMetrics := slices.Compact(metrics)
		fmt.Printf("    \"%s\": {\n", namespace)
		for _, metric := range uniqMetrics {
			fmt.Printf("        \"%s\",\n", metric)
		}
		fmt.Println("    },")
	}
	fmt.Println("}")
}

// namespacesWithUnsortedMetrics returns which namespaces have unsorted metrics
func namespacesWithUnsortedMetrics(NamespaceMetricsMap map[string][]string) []string {
	// Extract keys from the map and sort them
	keys := make([]string, 0, len(NamespaceMetricsMap))
	for k := range NamespaceMetricsMap {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var unsortedNamespace []string
	// Check if metrics are sorted for each key
	for _, namespace := range keys {
		metrics := NamespaceMetricsMap[namespace]
		sortedMetrics := make([]string, len(metrics))
		copy(sortedMetrics, metrics)
		sort.Strings(sortedMetrics)

		// Compare the sorted metrics with the original to find unsorted metrics
		for i, metric := range metrics {
			if metric != sortedMetrics[i] {
				unsortedNamespace = append(unsortedNamespace, namespace)
				break // Only report the first unsorted metric per key
			}
		}
	}

	return unsortedNamespace
}

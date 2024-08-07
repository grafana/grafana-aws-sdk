package cloudWatchConsts

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"slices"
	"sort"
	"testing"
)

// test to check NamespaceMetricsMap is sorted alphabetically
func TestNamespaceMetricsMap(t *testing.T) {
	var ok bool
	var val string
	if ok, val = isNamespaceMetricsMapSorted(NamespaceMetricsMap); !ok {
		fmt.Println(val)
		fmt.Println("NamespaceMetricsMap is not sorted alphabetically. Please use sorted version:")
		printSortedNamespaceMetricsMapAsGoMap()
	}
	assert.True(t, ok)
}

func printSortedNamespaceMetricsMapAsGoMap() {
	keys := make([]string, 0, len(NamespaceMetricsMap))
	for k := range NamespaceMetricsMap {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	fmt.Println("var NamespaceMetricsMap = map[string][]string{")
	for _, k := range keys {
		metrics := NamespaceMetricsMap[k]
		sort.Strings(metrics)
		uniqMetrics := slices.Compact(metrics)
		fmt.Printf("    \"%s\": {\n", k)
		for _, metric := range uniqMetrics {
			fmt.Printf("        \"%s\",\n", metric)
		}
		fmt.Println("    },")
	}
	fmt.Println("}")
}

// isNamespaceMetricsMapSorted checks if NamespaceMetricsMap is sorted alphabetically by keys and their metrics.
func isNamespaceMetricsMapSorted(NamespaceMetricsMap map[string][]string) (bool, string) {
	var report string
	isSorted := true

	// Extract keys from the map and sort them
	keys := make([]string, 0, len(NamespaceMetricsMap))
	for k := range NamespaceMetricsMap {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// Check if metrics are sorted for each key
	for _, key := range keys {
		metrics := NamespaceMetricsMap[key]
		sortedMetrics := make([]string, len(metrics))
		copy(sortedMetrics, metrics)
		sort.Strings(sortedMetrics)

		// Compare the sorted metrics with the original to find unsorted metrics
		for i, metric := range metrics {
			if metric != sortedMetrics[i] {
				isSorted = false
				report += fmt.Sprintf("Metric '%s' in key '%s' is not sorted correctly.\n", metric, key)
				break // Only report the first unsorted metric per key
			}
		}
	}

	return isSorted, report
}

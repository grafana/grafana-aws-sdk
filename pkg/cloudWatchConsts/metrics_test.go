package cloudWatchConsts

import (
	"fmt"
	"maps"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// test to check NamespaceMetricsMap metrics are sorted alphabetically
func TestNamespaceMetricsAlphabetized(t *testing.T) {
	unsortedMetricNamespaces := namespacesWithUnsortedMetrics(NamespaceMetricsMap)
	if len(unsortedMetricNamespaces) != 0 {
		assert.Fail(t, "NamespaceMetricsMap is not sorted alphabetically. Please replace the printed services")
		printNamespacesThatNeedSorted(unsortedMetricNamespaces)
	}
}

func TestNamespaceMetricKeysAllHaveDimensions(t *testing.T) {
	namespaceMetricsKeys := slices.Collect(maps.Keys(NamespaceMetricsMap))
	namespaceDimensionKeys := slices.Collect(maps.Keys(NamespaceDimensionKeysMap))

	namespaceMetricsMissingKeys := findMetricKeysFromAMissingInB(namespaceDimensionKeys, namespaceMetricsKeys)

	if len(namespaceMetricsMissingKeys) != 0 {
		assert.Fail(t, "NamespaceMetricsMap is missing key(s) from NamespaceDimensionKeysMap.")
		fmt.Println(strings.Join(namespaceMetricsMissingKeys, "\n"))
	}
}

func TestNamespaceDimensionKeysAllHaveMetrics(t *testing.T) {
	namespaceMetricsKeys := slices.Collect(maps.Keys(NamespaceMetricsMap))
	namespaceDimensionKeys := slices.Collect(maps.Keys(NamespaceDimensionKeysMap))

	namespaceDimensionMissingKeys := findMetricKeysFromAMissingInB(namespaceMetricsKeys, namespaceDimensionKeys)

	if len(namespaceDimensionMissingKeys) != 0 {
		assert.Fail(t, "NamespaceDimensionKeysMap is missing key(s) from NamespaceMetricsMap.")
		fmt.Println(strings.Join(namespaceDimensionMissingKeys, "\n"))
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
	keys := slices.Collect(maps.Keys(NamespaceMetricsMap))

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

func findMetricKeysFromAMissingInB(a []string, b []string) []string {
	var missingKeys []string

	for i := range a {
		if !slices.Contains(b, a[i]) {
			missingKeys = append(missingKeys, a[i])
		}
	}

	return missingKeys
}

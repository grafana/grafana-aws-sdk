//go:build mage
// +build mage

package main

import (
	"github.com/magefile/mage/sh"
)

// Build builds the binaries.
func Build() error {
	return sh.RunV("go", "build", "./...")
}

// Test runs the test suite.
func Test() error {
	return sh.RunV("go", "test", "./...")
}

func Lint() error {
	if err := sh.RunV("golangci-lint", "run", "./..."); err != nil {
		return err
	}
	return nil
}

// Drone signs the Drone configuration file
// This needs to be run everytime the drone.yml file is modified
// See https://github.com/grafana/deployment_tools/blob/master/docs/infrastructure/drone/signing.md for more info
func Drone() error {
	if err := sh.RunV("drone", "lint"); err != nil {
		return err
	}

	if err := sh.RunV("drone", "--server", "https://drone.grafana.net", "sign", "--save", "grafana/grafana-plugin-sdk-go"); err != nil {
		return err
	}

	return nil
}

var Default = Build

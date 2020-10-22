//+build mage

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
	if err := sh.RunV("revive", "-formatter", "stylish", "-config", "scripts/configs/revive.toml", "./..."); err != nil {
		return err
	}

	return nil
}

var Default = Build

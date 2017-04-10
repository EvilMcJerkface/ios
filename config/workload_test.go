package config

import (
	"os"
	"testing"
)

// TestParseAuto calls ParseAuto for the example configuration file
func TestParseAuto(t *testing.T) {
	ParseWorkloadConfig(os.Getenv("GOPATH") + "/src/github.com/heidi-ann/ios/test/workloads/example.conf")
	ParseWorkloadConfig(os.Getenv("GOPATH") + "/src/github.com/heidi-ann/ios/test/workloads/balanced.conf")
	ParseWorkloadConfig(os.Getenv("GOPATH") + "/src/github.com/heidi-ann/ios/test/workloads/read-heavy.conf")
	ParseWorkloadConfig(os.Getenv("GOPATH") + "/src/github.com/heidi-ann/ios/test/workloads/write-heavy.conf")
}

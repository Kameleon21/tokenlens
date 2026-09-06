package app

import (
	"regexp"
	"testing"
	"time"
)

func TestVersionFlagDoesNotRequireRuntimeConfiguration(t *testing.T) {
	t.Setenv("TZ", "invalid/timezone")
	t.Setenv("TOKENLENS_CURRENCY", "invalid")
	o, err := options([]string{"--version"}, time.Now())
	if err != nil || !o.ShowVersion {
		t.Fatalf("version lookup required runtime configuration: %v", err)
	}
}

func TestReleaseVersionFormat(t *testing.T) {
	if !regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`).MatchString(Version) {
		t.Fatalf("Version must be a semantic release number without a v prefix: %q", Version)
	}
}

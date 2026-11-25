package main

import (
	"runtime/debug"
	"testing"
)

func TestResolvedVersion(t *testing.T) {
	original := version
	t.Cleanup(func() { version = original })
	for _, tc := range []struct {
		name    string
		version string
		module  string
		ok      bool
		want    string
	}{
		{"release", "0.5.0-rc.1", "v0.4.1", true, "0.5.0-rc.1"},
		{"installed module", "dev", "v0.4.1", true, "0.4.1"},
		{"installed prerelease", "dev", "v0.5.0-beta.1", true, "0.5.0-beta.1"},
		{"source build", "dev", "(devel)", true, "dev"},
		{"empty module version", "dev", "", true, "dev"},
		{"missing build info", "dev", "", false, "dev"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			version = tc.version
			read := func() (*debug.BuildInfo, bool) {
				if tc.version != "dev" {
					t.Fatal("release version must take precedence over build info")
				}
				if !tc.ok {
					return nil, false
				}
				return &debug.BuildInfo{Main: debug.Module{Version: tc.module}}, true
			}
			if got := resolvedVersion(read); got != tc.want {
				t.Errorf("resolvedVersion() = %q, want %q", got, tc.want)
			}
		})
	}
}

package tdl

import "testing"

func TestManagedDownloadVersionSupport(t *testing.T) {
	tests := []struct {
		name    string
		version string
		want    bool
	}{
		{name: "release output with minimum version", version: "Version: 0.20.4\nCommit: abc123", want: true},
		{name: "prefixed version", version: "tdl v0.20.4", want: true},
		{name: "newer release", version: "Version: 0.21.0", want: true},
		{name: "same minor newer patch", version: "Version: 0.20.5", want: true},
		{name: "older patch", version: "Version: 0.20.3", want: false},
		{name: "older minor", version: "Version: 0.19.9", want: false},
		{name: "prerelease of minimum", version: "Version: 0.20.4-rc.1", want: false},
		{name: "prerelease newer than minimum", version: "Version: 0.21.0-rc.1", want: false},
		{name: "development tag newer than minimum", version: "tdl v0.22.0-dev.2", want: false},
		{name: "unknown dev build", version: "tdl development build", want: false},
		{name: "empty output", version: "", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := SupportsManagedDownloadVersion(tt.version); got != tt.want {
				t.Fatalf("SupportsManagedDownloadVersion(%q) = %v, want %v", tt.version, got, tt.want)
			}
		})
	}
}

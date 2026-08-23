package system

import (
	"strings"
	"testing"
)

func TestBanner(t *testing.T) {
	banner := Banner(":8080", false)

	if !strings.Contains(banner, "ZEN") {
		t.Fatal("banner should contain 'ZEN'")
	}
	if !strings.Contains(banner, ":8080") {
		t.Fatal("banner should contain address")
	}
}

func TestBanner_Version(t *testing.T) {
	origVersion := Version
	Version = "1.4.0"
	defer func() { Version = origVersion }()

	banner := Banner(":8080", false)
	if !strings.Contains(banner, "v1.4.0") {
		t.Fatalf("banner should contain version; got: %s", banner)
	}
}

func TestBanner_DefaultVersion(t *testing.T) {
	if Version == "" {
		t.Fatal("version must not be empty")
	}
}

func TestBanner_TLS(t *testing.T) {
	banner := Banner(":8080", true)
	if !strings.Contains(banner, "https://localhost:8080") {
		t.Fatalf("TLS banner should show https URL; got: %s", banner)
	}
	if strings.Contains(Banner(":8080", false), "https://") {
		t.Fatal("plain banner should not show https URL")
	}
}

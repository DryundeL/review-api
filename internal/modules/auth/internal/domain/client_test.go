package domain

import "testing"

func TestClientLabel(t *testing.T) {
	t.Parallel()
	browser, device := ClientLabel("Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	if browser != "Chrome" || device != "macOS" {
		t.Fatalf("chrome mac = %s %s", browser, device)
	}
	browser, device = ClientLabel("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36 Edg/120.0.0.0")
	if browser != "Edge" || device != "Windows" {
		t.Fatalf("edge windows = %s %s", browser, device)
	}
	browser, device = ClientLabel("Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Mobile/15E148 Safari/604.1")
	if browser != "Safari" || device != "iPhone" {
		t.Fatalf("safari iphone = %s %s", browser, device)
	}
	browser, device = ClientLabel("  ")
	if browser != "Unknown" || device != "Unknown" {
		t.Fatalf("empty = %s %s", browser, device)
	}
}

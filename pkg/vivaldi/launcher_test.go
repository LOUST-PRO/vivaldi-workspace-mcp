package vivaldi

import "testing"

func TestSplitValidURLs_AllValid(t *testing.T) {
	urls := []string{
		"https://example.com",
		"https://github.com/foo/bar",
		"http://localhost:8080/",
	}
	accepted, rejected := splitValidURLs(urls)
	if len(accepted) != 3 || len(rejected) != 0 {
		t.Errorf("expected 3/0, got %d/%d", len(accepted), len(rejected))
	}
}

func TestSplitValidURLs_RejectsNonHTTP(t *testing.T) {
	urls := []string{
		"https://example.com", // ok
		"file:///etc/passwd",
		"javascript:alert(1)",
		"data:text/plain,hello",
		"ftp://server/file",
		"",
		"   ",
		"https://ok.com/path?q=v",
	}
	accepted, rejected := splitValidURLs(urls)
	if len(accepted) != 2 {
		t.Errorf("expected 2 accepted, got %d (%v)", len(accepted), accepted)
	}
	// 4 non-http schemes + 1 summary entry for the empty x 2
	if len(rejected) != 5 {
		t.Errorf("expected 5 rejected, got %d (%v)", len(rejected), rejected)
	}
}

func TestSplitValidURLs_TooShort(t *testing.T) {
	urls := []string{
		"https://x", // 9 chars, below 12-char minimum
		"http://a.b",
		"https://short.com",
	}
	accepted, rejected := splitValidURLs(urls)
	if len(accepted) != 1 {
		t.Errorf("expected 1 accepted, got %d", len(accepted))
	}
	if len(rejected) != 2 {
		t.Errorf("expected 2 rejected, got %d", len(rejected))
	}
}

func TestSplitValidURLs_PreservesOrder(t *testing.T) {
	urls := []string{
		"https://a.com",
		"file:///x",
		"https://b.com",
		"ftp://x",
		"https://c.com",
	}
	accepted, rejected := splitValidURLs(urls)
	if len(accepted) != 3 || accepted[0] != "https://a.com" || accepted[1] != "https://b.com" || accepted[2] != "https://c.com" {
		t.Errorf("order broken: %v", accepted)
	}
	if len(rejected) != 2 {
		t.Errorf("expected 2 rejected, got %d (%v)", len(rejected), rejected)
	}
}

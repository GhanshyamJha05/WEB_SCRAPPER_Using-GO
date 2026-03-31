package scraper

import (
	"reflect"
	"testing"
)

func TestParseURLs(t *testing.T) {
	raw := "https://a.com, https://b.com\nhttps://c.com"
	got := ParseURLs(raw)
	want := []string{"https://a.com", "https://b.com", "https://c.com"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ParseURLs() = %v, want %v", got, want)
	}
}

package server

import (
	"reflect"
	"testing"

	"main/src/config"
	"main/src/packet"
	"main/src/text"
)

func TestServerLinks(t *testing.T) {
	links, err := serverLinks([]config.ServerLink{
		{Label: "report_bug", URL: "https://example.com/issues"},
		{Label: "Live map", URL: "http://example.com:8100"},
	})

	want := []packet.ServerLink{
		{Type: packet.LinkReportBug, URL: "https://example.com/issues"},
		{Type: packet.LinkCustom, Label: text.Text("Live map"), URL: "http://example.com:8100"},
	}

	if err != nil || !reflect.DeepEqual(links, want) {
		t.Fatalf("serverLinks = %+v, %v, want %+v", links, err, want)
	}

	for _, bad := range []string{"example.com", "ftp://example.com", "javascript:alert(1)", "https://"} {
		if _, err := serverLinks([]config.ServerLink{{Label: "website", URL: bad}}); err == nil {
			t.Errorf("link %q was accepted", bad)
		}
	}
}

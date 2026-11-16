package config

import (
	"reflect"
	"testing"
)

func TestServerLinksFromEnvironment(t *testing.T) {
	t.Setenv("SERVER_LINKS", "website=https://example.com/?a=b, Live map=http://example.com:8100")

	c := New()
	c.readEnvVars()

	want := []ServerLink{
		{Label: "website", URL: "https://example.com/?a=b"},
		{Label: "Live map", URL: "http://example.com:8100"},
	}

	if !reflect.DeepEqual(c.ServerLinks, want) {
		t.Fatalf("SERVER_LINKS parsed as %+v, want %+v", c.ServerLinks, want)
	}
}

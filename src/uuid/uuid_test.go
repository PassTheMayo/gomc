package uuid

import (
	"strings"
	"testing"
)

func TestOfflinePlayer(t *testing.T) {
	// Reference value computed independently from MD5("OfflinePlayer:Notch").
	if got := OfflinePlayer("Notch").String(); got != "b50ad385-829d-3141-a216-7e7d7539ba7f" {
		t.Errorf("OfflinePlayer(Notch) = %s", got)
	}
}

func TestParse(t *testing.T) {
	u := New()

	for _, s := range []string{u.String(), strings.ReplaceAll(u.String(), "-", "")} {
		parsed, err := Parse(s)

		if err != nil || parsed != u {
			t.Errorf("Parse(%q) = %v, %v", s, parsed, err)
		}
	}

	if _, err := Parse("not-a-uuid"); err == nil {
		t.Error("expected an error for an invalid UUID")
	}

	if u[6]>>4 != 4 {
		t.Error("New must produce version 4 UUIDs")
	}
}

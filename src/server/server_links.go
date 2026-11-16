package server

import (
	"fmt"
	"net/url"

	"main/src/config"
	"main/src/packet"
	"main/src/text"
)

// serverLinkTypes maps the vanilla names of built-in link types, used as
// labels in the configuration, to their IDs.
var serverLinkTypes = map[string]int32{
	"report_bug":           packet.LinkReportBug,
	"community_guidelines": packet.LinkCommunityGuidelines,
	"support":              packet.LinkSupport,
	"status":               packet.LinkStatus,
	"feedback":             packet.LinkFeedback,
	"community":            packet.LinkCommunity,
	"website":              packet.LinkWebsite,
	"forums":               packet.LinkForums,
	"news":                 packet.LinkNews,
	"announcements":        packet.LinkAnnouncements,
}

// serverLinks converts the configured links for the client. Clients only open
// absolute http and https links, so anything else is a configuration error.
func serverLinks(links []config.ServerLink) ([]packet.ServerLink, error) {
	out := make([]packet.ServerLink, 0, len(links))

	for _, l := range links {
		if u, err := url.Parse(l.URL); err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			return nil, fmt.Errorf("server: server link %q must be an http or https URL, got %q", l.Label, l.URL)
		}

		link := packet.ServerLink{Type: packet.LinkCustom, Label: text.Text(l.Label), URL: l.URL}

		if t, ok := serverLinkTypes[l.Label]; ok {
			link = packet.ServerLink{Type: t, URL: l.URL}
		}

		out = append(out, link)
	}

	return out, nil
}

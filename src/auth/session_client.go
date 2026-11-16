package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"main/src/profile"
	"main/src/uuid"
)

// DefaultSessionServer is Mojang's session server.
const DefaultSessionServer = "https://sessionserver.mojang.com"

// ErrNotAuthenticated is returned when the session server does not recognise
// the player's join request.
var ErrNotAuthenticated = errors.New("auth: player failed to authenticate with the session server")

// SessionClient verifies players with a session server.
type SessionClient struct {
	BaseURL string
	HTTP    *http.Client
}

// NewSessionClient returns a client for Mojang's session server.
func NewSessionClient() *SessionClient {
	return &SessionClient{BaseURL: DefaultSessionServer, HTTP: &http.Client{Timeout: 10 * time.Second}}
}

// HasJoined asks the session server whether username joined with serverHash
// and returns the authenticated profile, including its skin properties.
func (c *SessionClient) HasJoined(ctx context.Context, username, serverHash string) (profile.GameProfile, error) {
	query := url.Values{"username": {username}, "serverId": {serverHash}}
	endpoint := c.BaseURL + "/session/minecraft/hasJoined?" + query.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)

	if err != nil {
		return profile.GameProfile{}, err
	}

	res, err := c.HTTP.Do(req)

	if err != nil {
		return profile.GameProfile{}, err
	}

	defer res.Body.Close()

	switch res.StatusCode {
	case http.StatusOK:
	case http.StatusNoContent:
		return profile.GameProfile{}, ErrNotAuthenticated
	default:
		return profile.GameProfile{}, fmt.Errorf("auth: session server returned %s", res.Status)
	}

	var body struct {
		ID         string             `json:"id"`
		Name       string             `json:"name"`
		Properties []profile.Property `json:"properties"`
	}

	if err = json.NewDecoder(res.Body).Decode(&body); err != nil {
		return profile.GameProfile{}, err
	}

	id, err := uuid.Parse(body.ID)

	if err != nil {
		return profile.GameProfile{}, err
	}

	return profile.GameProfile{ID: id, Name: body.Name, Properties: body.Properties}, nil
}

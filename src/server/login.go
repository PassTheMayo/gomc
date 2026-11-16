package server

import (
	"bytes"
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"regexp"
	"time"

	"main/src/auth"
	"main/src/packet"
	"main/src/profile"
	"main/src/text"
	"main/src/uuid"
)

// validName matches the player names the vanilla server accepts offline.
var validName = regexp.MustCompile(`^[a-zA-Z0-9_]{1,16}$`)

// login authenticates the player. It returns false if the connection was
// rejected or lost.
func (s *session) login() bool {
	prof, err := s.authenticate()

	if err != nil {
		s.log.Info("Login failed", "error", err)
		return false
	}

	s.profile = prof
	s.log = s.log.With("player", prof.Name)

	if s.srv.playerCount() >= s.srv.conf.MaxPlayers {
		s.disconnect(text.Translatable("multiplayer.disconnect.server_full"))
		return false
	}

	if threshold := s.srv.conf.CompressionThreshold; threshold >= 0 {
		s.send(&packet.LoginCompression{Threshold: int32(threshold)})
		s.conn.SetCompression(threshold)
	}

	s.send(&packet.LoginFinished{Profile: prof, SessionID: s.srv.sessionID})

	p, err := s.read(false)

	if err != nil {
		return false
	}

	if _, ok := p.(*packet.LoginAcknowledged); !ok {
		s.log.Info("Expected login acknowledgement", "packet", fmt.Sprintf("%T", p))
		return false
	}

	s.setState(stateConfiguration, packet.ConfigurationServerbound, packet.ConfigurationClientbound)

	return true
}

// authenticate reads the login start packet and returns the player's
// profile, verified with the session server in online mode.
func (s *session) authenticate() (profile.GameProfile, error) {
	p, err := s.read(false)

	if err != nil {
		return profile.GameProfile{}, err
	}

	hello, ok := p.(*packet.LoginHello)

	if !ok {
		return profile.GameProfile{}, fmt.Errorf("expected login start, got %T", p)
	}

	if !s.srv.conf.OnlineMode {
		if !validName.MatchString(hello.Name) {
			s.disconnect(text.Translatable("multiplayer.disconnect.invalid_player_data"))
			return profile.GameProfile{}, fmt.Errorf("invalid player name %q", hello.Name)
		}

		return profile.GameProfile{ID: uuid.OfflinePlayer(hello.Name), Name: hello.Name}, nil
	}

	return s.verify(hello.Name)
}

// verify performs the encryption handshake and asks the session server
// whether the player really joined this server.
func (s *session) verify(name string) (profile.GameProfile, error) {
	challenge := make([]byte, 4)
	_, _ = rand.Read(challenge)

	s.send(&packet.EncryptionRequest{PublicKey: s.srv.keys.PublicDER, Challenge: challenge, ShouldAuthenticate: true})

	p, err := s.read(false)

	if err != nil {
		return profile.GameProfile{}, err
	}

	res, ok := p.(*packet.EncryptionResponse)

	if !ok {
		return profile.GameProfile{}, fmt.Errorf("expected encryption response, got %T", p)
	}

	secret, err := s.srv.keys.Decrypt(res.SharedSecret)

	if err != nil || len(secret) != 16 {
		return profile.GameProfile{}, errors.New("invalid shared secret")
	}

	answer, err := s.srv.keys.Decrypt(res.Challenge)

	if err != nil || !bytes.Equal(answer, challenge) {
		return profile.GameProfile{}, errors.New("invalid challenge response")
	}

	if err = s.conn.EnableEncryption(secret); err != nil {
		return profile.GameProfile{}, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	prof, err := s.srv.sessions.HasJoined(ctx, name, auth.ServerHash("", secret, s.srv.keys.PublicDER))

	switch {
	case errors.Is(err, auth.ErrNotAuthenticated):
		s.disconnect(text.Translatable("multiplayer.disconnect.unverified_username"))
		return profile.GameProfile{}, err
	case err != nil:
		s.disconnect(text.Translatable("multiplayer.disconnect.authservers_down"))
		return profile.GameProfile{}, err
	}

	return prof, nil
}

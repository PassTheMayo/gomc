// Package profile describes player identities.
package profile

import "main/src/uuid"

// Property is a signed profile attribute, such as the player's skin textures.
type Property struct {
	Name      string `json:"name"`
	Value     string `json:"value"`
	Signature string `json:"signature,omitempty"`
}

// GameProfile identifies a player.
type GameProfile struct {
	ID         uuid.UUID
	Name       string
	Properties []Property
}

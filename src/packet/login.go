package packet

import (
	"encoding/json"

	"main/src/profile"
	"main/src/protocol"
	"main/src/text"
	"main/src/uuid"
)

// LoginHello starts the login with the player's name and claimed UUID.
type LoginHello struct {
	Name string
	UUID uuid.UUID
}

func (p *LoginHello) Encode(w *protocol.Writer) {
	w.String(p.Name)
	w.UUID(p.UUID)
}

func (p *LoginHello) Decode(r *protocol.Reader) {
	p.Name = r.String(16)
	p.UUID = r.UUID()
}

// EncryptionRequest sends the server's public key and a challenge.
type EncryptionRequest struct {
	ServerID           string
	PublicKey          []byte
	Challenge          []byte
	ShouldAuthenticate bool
}

func (p *EncryptionRequest) Encode(w *protocol.Writer) {
	w.String(p.ServerID)
	w.ByteArray(p.PublicKey)
	w.ByteArray(p.Challenge)
	w.Bool(p.ShouldAuthenticate)
}

func (p *EncryptionRequest) Decode(r *protocol.Reader) {
	p.ServerID = r.String(20)
	p.PublicKey = r.ByteArray(r.Remaining())
	p.Challenge = r.ByteArray(r.Remaining())
	p.ShouldAuthenticate = r.Bool()
}

// EncryptionResponse returns the shared secret and challenge, both encrypted
// with the server's public key.
type EncryptionResponse struct {
	SharedSecret []byte
	Challenge    []byte
}

func (p *EncryptionResponse) Encode(w *protocol.Writer) {
	w.ByteArray(p.SharedSecret)
	w.ByteArray(p.Challenge)
}

func (p *EncryptionResponse) Decode(r *protocol.Reader) {
	p.SharedSecret = r.ByteArray(256)
	p.Challenge = r.ByteArray(256)
}

// LoginCompression enables compression for packets of at least Threshold bytes.
type LoginCompression struct {
	Threshold int32
}

func (p *LoginCompression) Encode(w *protocol.Writer) { w.VarInt(p.Threshold) }
func (p *LoginCompression) Decode(r *protocol.Reader) { p.Threshold = r.VarInt() }

// LoginFinished completes the login with the player's final profile and the
// server's session ID.
type LoginFinished struct {
	Profile   profile.GameProfile
	SessionID uuid.UUID
}

func (p *LoginFinished) Encode(w *protocol.Writer) {
	w.UUID(p.Profile.ID)
	w.String(p.Profile.Name)
	writeProperties(w, p.Profile.Properties)
	w.UUID(p.SessionID)
}

func (p *LoginFinished) Decode(r *protocol.Reader) {
	p.Profile.ID = r.UUID()
	p.Profile.Name = r.String(16)
	p.Profile.Properties = readProperties(r)
	p.SessionID = r.UUID()
}

// LoginAcknowledged confirms LoginFinished and switches to configuration.
type LoginAcknowledged struct{}

func (p *LoginAcknowledged) Encode(*protocol.Writer) {}
func (p *LoginAcknowledged) Decode(*protocol.Reader) {}

// LoginDisconnect rejects the login. The reason is encoded as JSON.
type LoginDisconnect struct {
	Reason text.Component
}

func (p *LoginDisconnect) Encode(w *protocol.Writer) {
	data, _ := json.Marshal(p.Reason)
	w.String(string(data))
}

func (p *LoginDisconnect) Decode(r *protocol.Reader) {
	data := r.String(262144)

	if r.Err() == nil {
		if err := json.Unmarshal([]byte(data), &p.Reason); err != nil {
			r.Fail(err)
		}
	}
}

// writeProperties writes game profile properties such as skin textures.
func writeProperties(w *protocol.Writer, props []profile.Property) {
	writeList(w, props, func(prop profile.Property) {
		w.String(prop.Name)
		w.String(prop.Value)
		w.Bool(prop.Signature != "")

		if prop.Signature != "" {
			w.String(prop.Signature)
		}
	})
}

func readProperties(r *protocol.Reader) []profile.Property {
	return readList(r, 16, func() profile.Property {
		prop := profile.Property{Name: r.String(64), Value: r.String(32767)}

		if r.Bool() {
			prop.Signature = r.String(1024)
		}

		return prop
	})
}

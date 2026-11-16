package packet

import (
	"main/src/item"
	"main/src/protocol"
	"main/src/uuid"
)

// AddEntity spawns an entity on the client.
type AddEntity struct {
	EntityID   int32
	UUID       uuid.UUID
	Type       int32
	X, Y, Z    float64
	VX, VY, VZ float64
	Pitch      float32
	Yaw        float32
	HeadYaw    float32
	Data       int32
}

func (p *AddEntity) Encode(w *protocol.Writer) {
	w.VarInt(p.EntityID)
	w.UUID(p.UUID)
	w.VarInt(p.Type)
	w.Double(p.X)
	w.Double(p.Y)
	w.Double(p.Z)
	w.LpVec3(p.VX, p.VY, p.VZ)
	w.Angle(p.Pitch)
	w.Angle(p.Yaw)
	w.Angle(p.HeadYaw)
	w.VarInt(p.Data)
}

func (p *AddEntity) Decode(r *protocol.Reader) {
	p.EntityID = r.VarInt()
	p.UUID = r.UUID()
	p.Type = r.VarInt()
	p.X = r.Double()
	p.Y = r.Double()
	p.Z = r.Double()
	p.VX, p.VY, p.VZ = r.LpVec3()
	p.Pitch = r.Angle()
	p.Yaw = r.Angle()
	p.HeadYaw = r.Angle()
	p.Data = r.VarInt()
}

// moveFlags packs the on-ground flag and the number of interpolation steps of
// a relative move. The server always sends linear moves, which have no steps.
func moveFlags(onGround bool) int32 {
	if onGround {
		return 1
	}

	return 0
}

// decodeMoveFlags reads relative move flags and rejects stepped moves, which
// the server never sends.
func decodeMoveFlags(r *protocol.Reader) bool {
	flags := r.VarInt()

	if flags>>1 != 0 {
		r.Fail(errSteppedMove)
	}

	return flags&1 != 0
}

// MoveEntityPos moves an entity by less than 8 blocks on each axis. Deltas are
// in 1/4096ths of a block.
type MoveEntityPos struct {
	EntityID   int32
	DX, DY, DZ int16
	OnGround   bool
}

func (p *MoveEntityPos) Encode(w *protocol.Writer) {
	w.VarInt(p.EntityID)
	w.VarInt(moveFlags(p.OnGround))
	w.Short(p.DX)
	w.Short(p.DY)
	w.Short(p.DZ)
}

func (p *MoveEntityPos) Decode(r *protocol.Reader) {
	p.EntityID = r.VarInt()
	p.OnGround = decodeMoveFlags(r)
	p.DX = r.Short()
	p.DY = r.Short()
	p.DZ = r.Short()
}

// MoveEntityPosRot moves and rotates an entity.
type MoveEntityPosRot struct {
	EntityID   int32
	DX, DY, DZ int16
	Yaw        float32
	Pitch      float32
	OnGround   bool
}

func (p *MoveEntityPosRot) Encode(w *protocol.Writer) {
	w.VarInt(p.EntityID)
	w.VarInt(moveFlags(p.OnGround))
	w.Short(p.DX)
	w.Short(p.DY)
	w.Short(p.DZ)
	w.Angle(p.Yaw)
	w.Angle(p.Pitch)
}

func (p *MoveEntityPosRot) Decode(r *protocol.Reader) {
	p.EntityID = r.VarInt()
	p.OnGround = decodeMoveFlags(r)
	p.DX = r.Short()
	p.DY = r.Short()
	p.DZ = r.Short()
	p.Yaw = r.Angle()
	p.Pitch = r.Angle()
}

// MoveEntityRot rotates an entity.
type MoveEntityRot struct {
	EntityID int32
	Yaw      float32
	Pitch    float32
	OnGround bool
}

func (p *MoveEntityRot) Encode(w *protocol.Writer) {
	w.VarInt(p.EntityID)
	w.Bool(p.OnGround)
	w.Angle(p.Yaw)
	w.Angle(p.Pitch)
}

func (p *MoveEntityRot) Decode(r *protocol.Reader) {
	p.EntityID = r.VarInt()
	p.OnGround = r.Bool()
	p.Yaw = r.Angle()
	p.Pitch = r.Angle()
}

// RotateHead sets the head yaw of an entity.
type RotateHead struct {
	EntityID int32
	HeadYaw  float32
}

func (p *RotateHead) Encode(w *protocol.Writer) {
	w.VarInt(p.EntityID)
	w.Angle(p.HeadYaw)
}

func (p *RotateHead) Decode(r *protocol.Reader) {
	p.EntityID = r.VarInt()
	p.HeadYaw = r.Angle()
}

// EntityPositionSync teleports an entity to an absolute position. It is used
// for moves too large for MoveEntityPos and to correct accumulated drift.
type EntityPositionSync struct {
	EntityID int32
	X, Y, Z  float64
	Yaw      float32
	Pitch    float32
	OnGround bool
}

// positionPathLinear is the PositionPath type of a plain target position.
const positionPathLinear int32 = 0

func (p *EntityPositionSync) Encode(w *protocol.Writer) {
	w.VarInt(p.EntityID)
	w.VarInt(positionPathLinear)
	w.Double(p.X)
	w.Double(p.Y)
	w.Double(p.Z)
	w.Float(p.Yaw)
	w.Float(p.Pitch)
	w.Bool(p.OnGround)
}

func (p *EntityPositionSync) Decode(r *protocol.Reader) {
	p.EntityID = r.VarInt()

	if r.VarInt() != positionPathLinear {
		r.Fail(errSteppedMove)
	}

	p.X = r.Double()
	p.Y = r.Double()
	p.Z = r.Double()
	p.Yaw = r.Float()
	p.Pitch = r.Float()
	p.OnGround = r.Bool()
}

// RemoveEntities despawns entities on the client.
type RemoveEntities struct {
	EntityIDs []int32
}

func (p *RemoveEntities) Encode(w *protocol.Writer) {
	protocol.WriteList(w, p.EntityIDs, w.VarInt)
}

func (p *RemoveEntities) Decode(r *protocol.Reader) {
	p.EntityIDs = protocol.ReadList(r, r.Remaining(), r.VarInt)
}

// Entity data serializer IDs.
const (
	DataTypeByte      int32 = 0
	DataTypeItemStack int32 = 7
	DataTypePose      int32 = 20
)

// Entity data indices.
const (
	DataSharedFlags             uint8 = 0
	DataPose                    uint8 = 6
	DataItem                    uint8 = 8
	DataPlayerModeCustomisation uint8 = 16
)

// Shared entity flags stored at DataSharedFlags.
const (
	FlagCrouching int8 = 0x02
	FlagSprinting int8 = 0x08
)

// Poses stored at DataPose.
const (
	PoseStanding  int32 = 0
	PoseCrouching int32 = 5
)

// EntityDataValue is one synchronized entity data entry. The byte, pose and
// item stack serializers are supported, which is what players and dropped
// items use.
type EntityDataValue struct {
	Index uint8
	Type  int32
	Byte  int8
	Pose  int32
	Item  item.Stack
}

// Equal reports whether two entries hold the same value.
func (v EntityDataValue) Equal(o EntityDataValue) bool {
	if v.Index != o.Index || v.Type != o.Type || v.Byte != o.Byte || v.Pose != o.Pose {
		return false
	}

	return item.Equal(v.Item, o.Item)
}

// SetEntityData updates synchronized entity data.
type SetEntityData struct {
	EntityID int32
	Values   []EntityDataValue
}

func (p *SetEntityData) Encode(w *protocol.Writer) {
	w.VarInt(p.EntityID)

	for _, v := range p.Values {
		w.UByte(v.Index)
		w.VarInt(v.Type)

		switch v.Type {
		case DataTypeByte:
			w.Byte(v.Byte)
		case DataTypePose:
			w.VarInt(v.Pose)
		case DataTypeItemStack:
			writeStack(w, v.Item)
		}
	}

	w.UByte(0xFF)
}

func (p *SetEntityData) Decode(r *protocol.Reader) {
	p.EntityID = r.VarInt()

	for r.Err() == nil {
		index := r.UByte()

		if index == 0xFF {
			return
		}

		v := EntityDataValue{Index: index, Type: r.VarInt()}

		switch v.Type {
		case DataTypeByte:
			v.Byte = r.Byte()
		case DataTypePose:
			v.Pose = r.VarInt()
		case DataTypeItemStack:
			v.Item = readStack(r)
		default:
			r.Fail(errUnsupportedDataType)
		}

		p.Values = append(p.Values, v)
	}
}

// SetEntityMotion sets an entity's velocity, in blocks per tick.
type SetEntityMotion struct {
	EntityID   int32
	VX, VY, VZ float64
}

func (p *SetEntityMotion) Encode(w *protocol.Writer) {
	w.VarInt(p.EntityID)
	w.LpVec3(p.VX, p.VY, p.VZ)
}

func (p *SetEntityMotion) Decode(r *protocol.Reader) {
	p.EntityID = r.VarInt()
	p.VX, p.VY, p.VZ = r.LpVec3()
}

// TakeItemEntity shows an entity picking up a dropped item. Amount is the
// item's count before the pickup, even when only part of it fit.
type TakeItemEntity struct {
	ItemID      int32
	CollectorID int32
	Amount      int32
}

func (p *TakeItemEntity) Encode(w *protocol.Writer) {
	w.VarInt(p.ItemID)
	w.VarInt(p.CollectorID)
	w.VarInt(p.Amount)
}

func (p *TakeItemEntity) Decode(r *protocol.Reader) {
	p.ItemID = r.VarInt()
	p.CollectorID = r.VarInt()
	p.Amount = r.VarInt()
}

// Package nbt implements Minecraft's Named Binary Tag format in its network
// form, where the root tag carries no name.
package nbt

// Tag type identifiers.
const (
	TypeEnd       byte = 0
	TypeByte      byte = 1
	TypeShort     byte = 2
	TypeInt       byte = 3
	TypeLong      byte = 4
	TypeFloat     byte = 5
	TypeDouble    byte = 6
	TypeByteArray byte = 7
	TypeString    byte = 8
	TypeList      byte = 9
	TypeCompound  byte = 10
	TypeIntArray  byte = 11
	TypeLongArray byte = 12
)

// Tag is any NBT value.
type Tag interface {
	Type() byte
}

type (
	// Byte is a signed 8-bit integer tag.
	Byte int8

	// Short is a signed 16-bit integer tag.
	Short int16

	// Int is a signed 32-bit integer tag.
	Int int32

	// Long is a signed 64-bit integer tag.
	Long int64

	// Float is a single precision float tag.
	Float float32

	// Double is a double precision float tag.
	Double float64

	// ByteArray is an array of signed bytes.
	ByteArray []byte

	// String is a modified UTF-8 string tag.
	String string

	// IntArray is an array of signed 32-bit integers.
	IntArray []int32

	// LongArray is an array of signed 64-bit integers.
	LongArray []int64
)

// List is a homogeneous list of tags.
type List struct {
	ElemType byte
	Items    []Tag
}

// Entry is a named tag inside a Compound.
type Entry struct {
	Name  string
	Value Tag
}

// Compound is an ordered collection of named tags.
type Compound []Entry

func (Byte) Type() byte      { return TypeByte }
func (Short) Type() byte     { return TypeShort }
func (Int) Type() byte       { return TypeInt }
func (Long) Type() byte      { return TypeLong }
func (Float) Type() byte     { return TypeFloat }
func (Double) Type() byte    { return TypeDouble }
func (ByteArray) Type() byte { return TypeByteArray }
func (String) Type() byte    { return TypeString }
func (IntArray) Type() byte  { return TypeIntArray }
func (LongArray) Type() byte { return TypeLongArray }
func (List) Type() byte      { return TypeList }
func (Compound) Type() byte  { return TypeCompound }

// NewList returns a list holding items, which must all share one type.
func NewList(items ...Tag) List {
	if len(items) == 0 {
		return List{ElemType: TypeEnd}
	}

	return List{ElemType: items[0].Type(), Items: items}
}

// Set stores value under name, replacing an existing entry with that name.
func (c *Compound) Set(name string, value Tag) {
	for i := range *c {
		if (*c)[i].Name == name {
			(*c)[i].Value = value
			return
		}
	}

	*c = append(*c, Entry{Name: name, Value: value})
}

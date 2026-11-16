package protocol

// BitSet is a growable set of bits with the byte layout of Java's
// BitSet.toByteArray: bit i is stored in byte i/8 at position i%8.
type BitSet []byte

// BitSetFromBytes wraps b as a BitSet.
func BitSetFromBytes(b []byte) BitSet {
	return BitSet(b)
}

// Set sets bit i.
func (s *BitSet) Set(i int) {
	for len(*s) <= i/8 {
		*s = append(*s, 0)
	}

	(*s)[i/8] |= 1 << (i % 8)
}

// Get reports whether bit i is set.
func (s BitSet) Get(i int) bool {
	return i/8 < len(s) && s[i/8]&(1<<(i%8)) != 0
}

// Bytes returns the bit set without trailing zero bytes, as Java does.
func (s BitSet) Bytes() []byte {
	n := len(s)

	for n > 0 && s[n-1] == 0 {
		n--
	}

	return s[:n]
}

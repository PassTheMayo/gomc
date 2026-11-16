package command

import "strings"

// Reader reads a command line word by word. Arguments read as much input as
// they need from it.
type Reader struct {
	input string
	pos   int
}

// NewReader returns a reader at the start of input.
func NewReader(input string) *Reader {
	return &Reader{input: input}
}

// AtEnd reports whether only whitespace is left.
func (r *Reader) AtEnd() bool {
	return strings.TrimSpace(r.input[r.pos:]) == ""
}

// PeekWord returns the next space-separated word without consuming it.
func (r *Reader) PeekWord() string {
	rest := strings.TrimLeft(r.input[r.pos:], " ")
	word, _, _ := strings.Cut(rest, " ")

	return word
}

// ReadWord consumes and returns the next space-separated word.
func (r *Reader) ReadWord() string {
	rest := r.input[r.pos:]
	trimmed := strings.TrimLeft(rest, " ")
	word, _, _ := strings.Cut(trimmed, " ")
	r.pos += len(rest) - len(trimmed) + len(word)

	return word
}

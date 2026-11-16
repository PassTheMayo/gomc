package protocol

// WriteList writes a VarInt count followed by each item.
func WriteList[T any](w *Writer, items []T, write func(T)) {
	w.VarInt(int32(len(items)))

	for _, item := range items {
		write(item)
	}
}

// ReadList reads a VarInt count of at most max items followed by each item.
// An empty list decodes as nil.
func ReadList[T any](r *Reader, max int, read func() T) []T {
	n := r.Length(max)

	if n == 0 {
		return nil
	}

	items := make([]T, 0, n)

	for i := 0; i < n && r.Err() == nil; i++ {
		items = append(items, read())
	}

	return items
}

package command

import "main/src/text"

// Source is whoever runs a command.
type Source interface {
	// Reply shows a message to the source.
	Reply(msg text.Component)

	// IsOperator reports whether the source may use restricted commands.
	IsOperator() bool
}

// Context is a parsed command being executed.
type Context struct {
	Source Source
	values map[string]any
}

// Int returns the value of the integer argument called name.
func (c *Context) Int(name string) int32 {
	return c.values[name].(int32)
}

// GameMode returns the ID of the game mode argument called name.
func (c *Context) GameMode(name string) int32 {
	return c.values[name].(int32)
}

// String returns the value of the text argument called name, such as a
// player's name.
func (c *Context) String(name string) string {
	return c.values[name].(string)
}

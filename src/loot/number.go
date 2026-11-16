package loot

import (
	"bytes"
	"encoding/json"
	"math"
)

// number is a number provider: a constant or a random distribution.
type number interface {
	float(ctx *Context) float32
	int(ctx *Context) int32
}

var numberTypes = decoders[number]{}

func init() {
	numberTypes["minecraft:constant"] = func(raw json.RawMessage) (number, error) {
		var v struct {
			Type  string  `json:"type"`
			Value float32 `json:"value"`
		}

		err := decodeStrict(raw, &v)

		return constant(v.Value), err
	}

	numberTypes["minecraft:uniform"] = func(raw json.RawMessage) (number, error) {
		var v struct {
			Type string          `json:"type"`
			Min  json.RawMessage `json:"min"`
			Max  json.RawMessage `json:"max"`
		}

		if err := decodeStrict(raw, &v); err != nil {
			return nil, err
		}

		lo, err := decodeNumber(v.Min)

		if err != nil {
			return nil, err
		}

		hi, err := decodeNumber(v.Max)

		return uniform{min: lo, max: hi}, err
	}

	numberTypes["minecraft:binomial"] = func(raw json.RawMessage) (number, error) {
		var v struct {
			Type string          `json:"type"`
			N    json.RawMessage `json:"n"`
			P    json.RawMessage `json:"p"`
		}

		if err := decodeStrict(raw, &v); err != nil {
			return nil, err
		}

		n, err := decodeNumber(v.N)

		if err != nil {
			return nil, err
		}

		p, err := decodeNumber(v.P)

		return binomial{n: n, p: p}, err
	}
}

// decodeNumber decodes a number provider, where a bare JSON number is a
// constant.
func decodeNumber(raw json.RawMessage) (number, error) {
	if !bytes.HasPrefix(bytes.TrimSpace(raw), []byte("{")) {
		var v float32
		err := json.Unmarshal(raw, &v)

		return constant(v), err
	}

	return numberTypes.decode("number provider", raw)
}

// constant is always the same number.
type constant float32

func (c constant) float(*Context) float32 { return float32(c) }
func (c constant) int(*Context) int32     { return int32(math.Round(float64(c))) }

// uniform is a number picked evenly between two bounds, inclusive for whole
// numbers.
type uniform struct {
	min, max number
}

func (u uniform) float(ctx *Context) float32 {
	lo, hi := u.min.float(ctx), u.max.float(ctx)

	if lo >= hi {
		return lo
	}

	return lo + ctx.Rand.Float32()*(hi-lo)
}

func (u uniform) int(ctx *Context) int32 {
	lo, hi := u.min.int(ctx), u.max.int(ctx)

	if lo >= hi {
		return lo
	}

	return lo + ctx.Rand.Int32N(hi-lo+1)
}

// binomial counts the successes of n trials that each succeed with
// probability p.
type binomial struct {
	n, p number
}

func (b binomial) float(ctx *Context) float32 {
	return float32(b.int(ctx))
}

func (b binomial) int(ctx *Context) int32 {
	n, p := b.n.int(ctx), b.p.float(ctx)

	var successes int32

	for range n {
		if ctx.Rand.Float32() < p {
			successes++
		}
	}

	return successes
}

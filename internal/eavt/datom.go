package eavt

// Op represents an assert or retract operation on a datom.
type Op int

const (
	Assert  Op = 1
	Retract Op = 0
)

// Datom is a single fact in the EAVT store.
type Datom struct {
	E  int64  // Entity ID
	A  string // Attribute
	V  Value  // Value
	Tx int64  // Transaction ID
	Op Op     // Assert or Retract
}

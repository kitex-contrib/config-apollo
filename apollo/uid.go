package apollo

import "go.uber.org/atomic"

var uniqueID atomic.Int64

func init() {
	uniqueID.Store(0)
}

// GetUniqueID get the unique id
func GetUniqueID() int64 {
	return uniqueID.Add(1)
}

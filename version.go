package untie

import "strconv"

const (
	libraryVersion = "0.1.0-dev"
	layoutRevision = 1
)

// Version returns the library and layout revision used in cache contracts.
func Version() string {
	return libraryVersion + "+layout." + strconv.Itoa(layoutRevision)
}

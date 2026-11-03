// Package render provides a small, framework-neutral drawing protocol over
// display.Box.
//
// Walk streams visible path and rule commands without copying transformed path
// geometry. The path template in each command borrows storage from the box and
// is valid only for the duration of its callback invocation. ResolvePath is the
// explicit allocating escape hatch for hosts that need standalone geometry.
//
// Fit computes a uniform transform that places all certified box bounds,
// including overflow, inside a host viewport.
package render

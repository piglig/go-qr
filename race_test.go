//go:build race

package qr

// raceEnabled reports whether the tests run with the race detector, which
// slows them too much for timing checks.
const raceEnabled = true

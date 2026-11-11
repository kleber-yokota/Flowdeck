package status

import "testing"

// Terminal reports the states that end the run (the 2s badge poll stops):
// success, failed and cancelled are terminal; queued and running are not.
func TestTerminalStates(t *testing.T) {
	for _, s := range []State{Success, Error, Cancelled} {
		if !s.Terminal() {
			t.Errorf("%s: want terminal, got false", s)
		}
	}
	for _, s := range []State{Queued, Running} {
		if s.Terminal() {
			t.Errorf("%s: want not terminal, got true", s)
		}
	}
}

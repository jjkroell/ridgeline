package auth

import (
	"testing"
	"time"
)

// With every Argon2 slot taken, a new hash waits instead of allocating
// another 64 MiB, and proceeds once a slot frees.
func TestArgonConcurrencyBounded(t *testing.T) {
	for i := 0; i < cap(argonSlots); i++ {
		argonSlots <- struct{}{}
	}
	done := make(chan struct{})
	go func() {
		HashPassword("hunter2hunter2")
		close(done)
	}()
	select {
	case <-done:
		t.Fatal("hash ran with every slot taken")
	case <-time.After(200 * time.Millisecond):
	}
	<-argonSlots
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("hash did not proceed after a slot freed")
	}
	for i := 1; i < cap(argonSlots); i++ {
		<-argonSlots
	}
}

package store

import (
	"fmt"
	"sync"
	"testing"
)

// Concurrent first registrations must produce exactly one protected owner.
func TestConcurrentFirstRegistrationsMakeOneOwner(t *testing.T) {
	st := testStore(t)
	var wg sync.WaitGroup
	owners := make(chan bool, 20)
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			u, err := st.CreateUser(fmt.Sprintf("u%d@example.com", i), "x", "")
			if err != nil {
				t.Error(err)
				return
			}
			owners <- u.IsOwner
		}(i)
	}
	wg.Wait()
	close(owners)
	n := 0
	for o := range owners {
		if o {
			n++
		}
	}
	var stored int
	st.db.QueryRow(`SELECT COUNT(*) FROM users WHERE protected = 1`).Scan(&stored)
	if n != 1 || stored != 1 {
		t.Fatalf("want exactly one owner, got %d reported / %d stored", n, stored)
	}
}

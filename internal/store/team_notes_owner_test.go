package store

import (
	"strings"
	"testing"
)

// Team notes from one ownership must not pass to the node's next owner; their
// authors keep them as private notes.
func TestNextOwnerCannotReadPreviousTeamNotes(t *testing.T) {
	st := testStore(t)
	node := strings.Repeat("AB", 32)
	first, _ := st.CreateUser("first@example.com", "x", "")
	author, _ := st.CreateUser("author@example.com", "x", "")
	next, _ := st.CreateUser("next@example.com", "x", "")

	if _, err := st.CreateVerifiedClaim(node, first.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateNote(node, author.ID, "team", "gate code 1234"); err != nil {
		t.Fatal(err)
	}
	if _, err := st.DeleteClaim(node, first.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateVerifiedClaim(node, next.ID); err != nil {
		t.Fatal(err)
	}

	if notes, _ := st.NotesForNode(node, next.ID, true); len(notes) != 0 {
		t.Fatalf("next owner sees the previous circle's notes: %+v", notes)
	}
	if notes, _ := st.NotesForNode(node, author.ID, false); len(notes) != 1 || notes[0].Visibility != "private" {
		t.Fatalf("author should keep the note as private, got %+v", notes)
	}
}

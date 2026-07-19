package core

import "testing"

func TestParseManualSongs(t *testing.T) {
	songs := ParseManualSongs(" First - Artist A\r\nSecond\n\n")
	if len(songs) != 2 || songs[0].Name != "First" || songs[0].Artist != "Artist A" || songs[1].Name != "Second" {
		t.Fatalf("songs = %#v", songs)
	}
	if songs[0].SourceID == "" || songs[1].SourceID == "" {
		t.Fatal("manual songs lack stable source IDs")
	}
}

func TestDefaultOptionsValidate(t *testing.T) {
	if err := DefaultOptions().Validate(); err != nil {
		t.Fatal(err)
	}
}

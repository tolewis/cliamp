package model

import (
	"testing"

	"github.com/bjarneo/cliamp/history"
	"github.com/bjarneo/cliamp/playlist"
	"github.com/bjarneo/cliamp/provider"
)

func playlistTrack(path string) playlist.Track {
	return playlist.Track{Path: path, Stream: true}
}

func TestSelectedHistoryEntry(t *testing.T) {
	entries := []history.Entry{
		{Track: playlistTrack("http://one/stream")},
		{Track: playlistTrack("http://two/stream")},
	}

	if got, ok := selectedHistoryEntry(entries, 1, "", ""); !ok || got.Track.Path != "http://two/stream" {
		t.Fatalf("index pick: got %v ok=%v", got, ok)
	}
	if got, ok := selectedHistoryEntry(entries, 0, "http://one/stream", ""); !ok || got.Track.Path != "http://one/stream" {
		t.Fatalf("path pick: got %v ok=%v", got, ok)
	}
	if _, ok := selectedHistoryEntry(entries, 5, "", ""); ok {
		t.Fatal("out-of-range index must not match")
	}
	if _, ok := selectedHistoryEntry(entries, -1, "", ""); ok {
		t.Fatal("no selector must not match")
	}
	if _, ok := selectedHistoryEntry(nil, 0, "", ""); ok {
		t.Fatal("empty history must not match")
	}
}

func TestProviderStatusesShape(t *testing.T) {
	m := &Model{
		providers: []provider.Entry{
			{Key: "radio", Name: "Radio"},
			{Key: "local", Name: "Local", Provider: commandsTestProvider{name: "Local"}},
		},
	}
	// nil provider entries stay unconfigured and unauthed; the shape holds
	// regardless of the spotify credential store on the machine.
	statuses := m.providerStatuses()
	if len(statuses) != 2 {
		t.Fatalf("want 2 rows, got %d", len(statuses))
	}
	if statuses[0].Key != "radio" || statuses[0].Configured || statuses[0].Authed {
		t.Fatalf("radio row wrong: %+v", statuses[0])
	}
	if statuses[1].Key != "local" || !statuses[1].Configured || !statuses[1].Authed || statuses[1].Active {
		t.Fatalf("local row wrong: %+v", statuses[1])
	}
}

func TestProviderAuthedNilEntry(t *testing.T) {
	if providerAuthed(provider.Entry{}) {
		t.Fatal("nil provider must not count as authed")
	}
}

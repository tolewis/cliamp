package model

import (
	tea "charm.land/bubbletea/v2"

	"github.com/bjarneo/cliamp/ipc"
	"github.com/bjarneo/cliamp/playlist"
)

// Bridge helpers for the station ops. The registry lives in stations.go;
// these adapt it to the v2 response types and to the player, the same way
// the TUI plays a station row (external/radio provider track shape).

// stationByLookup matches a registry row by id first, then by URL. Empty
// fields never match.
func stationByLookup(stations []StationInfo, id, rawURL string) (StationInfo, bool) {
	id, rawURL = trimStationKey(id), trimStationKey(rawURL)
	for _, s := range stations {
		if id != "" && s.ID == id {
			return s, true
		}
	}
	for _, s := range stations {
		if rawURL != "" && s.URL == rawURL {
			return s, true
		}
	}
	return StationInfo{}, false
}

func trimStationKey(v string) string {
	out := make([]rune, 0, len(v))
	for _, r := range v {
		if r != ' ' && r != '\t' && r != '\n' && r != '\r' {
			out = append(out, r)
		}
	}
	return string(out)
}

// stationsToIPC converts registry rows to their wire type.
func stationsToIPC(stations []StationInfo) []ipc.StationInfo {
	if len(stations) == 0 {
		return nil
	}
	out := make([]ipc.StationInfo, len(stations))
	for i, s := range stations {
		out[i] = ipc.StationInfo{ID: s.ID, Name: s.Name, URL: s.URL, Provider: s.Provider}
	}
	return out
}

// playStation switches the live player to a registry row. The track mirrors
// the radio provider's own row: stream, realtime, provider meta for the
// favorites and rename paths.
func (m *Model) playStation(s StationInfo) tea.Cmd {
	track := playlist.Track{
		Path:     s.URL,
		Title:    s.Name,
		Stream:   true,
		Realtime: true,
		ProviderMeta: map[string]string{
			"radio.name": s.Name,
			"radio.url":  s.URL,
		},
	}
	return m.playTrackImmediate(track)
}

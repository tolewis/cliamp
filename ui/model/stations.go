package model

import (
	"net/url"
	"strings"

	"github.com/bjarneo/cliamp/external/radio"
	"github.com/bjarneo/cliamp/playlist"
)

// Station registry for the station.list and station.play V2 operations and
// the stream branch of next/prev, per CONTRACT sections 1-3.
//
// A station is one row of the merged, deduped list: radio favorites first,
// then the cached radio catalog, then the stream URLs of the playback
// history. The merge is ordered by source and deduped by URL, first wins.

// StationInfo is one row of the station registry.
type StationInfo struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	URL      string `json:"url"`
	Provider string `json:"provider"`
}

// stationID builds the registry id of provider and name. A name that is not
// already a safe slug gets lowercased and stripped of unsafe runes.
func stationID(provider, name string) string {
	if name == "" {
		return provider + ":"
	}
	if strings.IndexFunc(name, func(r rune) bool {
		return !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-' || r == '_' || r == '.')
	}) < 0 {
		return provider + ":" + name
	}
	var b strings.Builder
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z' || r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == ' ' || r == '/' || r == '&' || r == ',' || r == '(' || r == ')' || r == '[' || r == ']' || r == '·' || r == '|' || r == '-':
			b.WriteByte('-')
		case r == '-' || r == '_' || r == '.':
			b.WriteRune(r)
		}
	}
	return provider + ":" + strings.Trim(b.String(), "-")
}

// stationNameForURL derives a display name from a stream URL: the host plus
// the last path segment.
func stationNameForURL(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil || u.Host == "" {
		return rawURL
	}
	name := u.Host
	if seg := strings.TrimRight(u.Path, "/"); seg != "" {
		if name, err = url.PathUnescape(seg); err != nil {
			name = seg
		}
	}
	return name
}

// isStreamTrack reports whether the current track is a stream or a station
// row of the registry.
func isStreamTrack(track playlist.Track) bool {
	return track.Stream || track.Meta("radio.url") != ""
}

// stationAtURL returns the registry row whose URL matches the current
// track, or -1.
func stationAtURL(stations []StationInfo, track playlist.Track) int {
	for i, s := range stations {
		if s.URL == track.Path {
			return i
		}
	}
	return -1
}

// prevStreamStation / nextStreamStation pick the wrapped neighbor of the
// current station. An unknown URL starts at 0: prev is the last station,
// next is the first. An empty registry has no station, so the normal queue
// behavior applies.
func prevStreamStation(stations []StationInfo, track playlist.Track) (StationInfo, bool) {
	if len(stations) == 0 {
		return StationInfo{}, false
	}
	idx := stationAtURL(stations, track)
	if idx < 0 {
		return stations[len(stations)-1], true
	}
	return stations[(idx-1+len(stations))%len(stations)], true
}

func nextStreamStation(stations []StationInfo, track playlist.Track) (StationInfo, bool) {
	if len(stations) == 0 {
		return StationInfo{}, false
	}
	idx := stationAtURL(stations, track)
	if idx < 0 {
		return stations[0], true
	}
	return stations[(idx+1)%len(stations)], true
}

// stationList builds the registry in the contract order: radio favorites,
// then the cached radio catalog of the active Radio provider, then the
// stream URLs of the playback history. It is deduped by URL, first wins.
func (m Model) stationList() []StationInfo {
	var out []StationInfo
	seen := map[string]bool{}
	add := func(name, rawURL, provider string) {
		if rawURL == "" || seen[rawURL] {
			return
		}
		seen[rawURL] = true
		out = append(out, StationInfo{ID: stationID(provider, name), Name: name, URL: rawURL, Provider: provider})
	}
	if m.radioFavorites != nil {
		for _, s := range m.radioFavorites.Stations() {
			add(s.Name, s.URL, "radio")
		}
	}
	if p, ok := m.provider.(*radio.Provider); ok {
		for _, s := range p.StationCatalog() {
			add(s.Name, s.URL, "radio")
		}
	}
	// Built-in cliamp radio channels, even when that provider is not the
	// active one and history is empty. A cold daemon still has stations.
	for _, entry := range m.providers {
		cp, ok := entry.Provider.(*radio.ChannelProvider)
		if !ok || cp == nil {
			continue
		}
		pls, err := cp.Playlists()
		if err != nil {
			continue
		}
		for _, pl := range pls {
			tracks, err := cp.Tracks(pl.ID)
			if err != nil || len(tracks) == 0 {
				continue
			}
			add(tracks[0].Title, tracks[0].Path, "cliamp")
		}
	}
	if m.historyStore != nil {
		if entries, err := m.historyStore.Recent(0); err == nil {
			for _, e := range entries {
				t := e.Track
				if !t.Stream {
					continue
				}
				name := t.Title
				if name == "" || name == t.Path {
					if t.Artist != "" {
						name = t.Artist + " — " + stationNameForURL(t.Path)
					} else {
						name = stationNameForURL(t.Path)
					}
				}
				add(name, t.Path, "history")
			}
		}
	}
	return out
}

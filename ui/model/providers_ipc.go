package model

import (
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/bjarneo/cliamp/external/spotify"
	"github.com/bjarneo/cliamp/history"
	"github.com/bjarneo/cliamp/ipc"
	"github.com/bjarneo/cliamp/playlist"
	"github.com/bjarneo/cliamp/provider"
)

// This file backs the provider.* and history.play IPC operations. See
// CONTRACT.md in the fork root: the shapes there are frozen.

// providerAuthed reports whether the credentials of a provider are usable.
// Providers without an OAuth flow are authed when configured: their
// credentials live in config and are read per call.
func providerAuthed(entry provider.Entry) bool {
	if entry.Provider == nil {
		return false
	}
	if _, ok := entry.Provider.(*spotify.SpotifyProvider); ok {
		return spotify.HasStoredCredentials()
	}
	return true
}

// providerStatuses reflects every known provider: config, activation and
// auth state. Read-only; no network calls, safe on the IPC path.
func (m *Model) providerStatuses() []ipc.ProviderStatus {
	out := make([]ipc.ProviderStatus, 0, len(m.providers))
	for i, entry := range m.providers {
		st := ipc.ProviderStatus{
			Key:        entry.Key,
			Name:       entry.Name,
			Configured: entry.Provider != nil,
			// Identity via the pill index: provider values are not always
			// comparable, so == on the interface can panic.
			Active: m.provider != nil && i == m.provPillIdx,
		}
		st.Authed = providerAuthed(entry)
		out = append(out, st)
	}
	return out
}

// selectedHistoryEntry resolves the history.play target: index into the
// order of the history op (most recent first), else the first entry whose
// track path equals the given path or URL. Index wins only when alone.
func selectedHistoryEntry(entries []history.Entry, index int, path, url string) (history.Entry, bool) {
	if len(entries) == 0 {
		return history.Entry{}, false
	}
	if index >= 0 && index < len(entries) && path == "" && url == "" {
		return entries[index], true
	}
	want := path
	if want == "" {
		want = url
	}
	if want == "" {
		return history.Entry{}, false
	}
	for _, entry := range entries {
		if entry.Track.Path == want {
			return entry, true
		}
	}
	return history.Entry{}, false
}

// ipcProviderAuth starts the interactive sign-in flow of a provider without
// a TUI. The flow opens the browser itself and waits on its loopback
// callback; the token lands wherever the TUI flow stores it. Idempotent
// while credentials are valid.
func (m *Model) ipcProviderAuth(request ipcLibraryRequest) tea.Cmd {
	entry, ok := m.providerEntryForRequest(request)
	if !ok {
		request.Reply <- ipc.Response{OK: false, Error: "unknown or unconfigured provider"}
		return nil
	}
	if providerAuthed(entry) {
		request.Reply <- ipc.Response{OK: true, State: "authed", ProviderStatuses: m.providerStatuses()}
		return nil
	}
	auth, ok := entry.Provider.(playlist.Authenticator)
	if !ok {
		request.Reply <- ipc.Response{OK: true, State: "none_required", ProviderStatuses: m.providerStatuses()}
		return nil
	}
	request.Reply <- ipc.Response{OK: true, State: "awaiting_browser", ProviderStatuses: m.providerStatuses()}
	return authenticateProviderCmd(auth, entry.Name, nextRequest(&m.requests.auth))
}

// ipcProviderSwitch makes the keyed provider active, stopping playback
// first so nothing keeps playing from the old one.
func (m *Model) ipcProviderSwitch(request ipcLibraryRequest) tea.Cmd {
	key := request.Key
	if key == "" {
		key = request.Provider
	}
	idx := -1
	for i, entry := range m.providers {
		if strings.EqualFold(entry.Key, key) && entry.Provider != nil {
			idx = i
		}
	}
	if idx < 0 {
		request.Reply <- ipc.Response{OK: false, Error: "unknown or unconfigured provider"}
		return nil
	}
	if m.player != nil && m.player.IsPlaying() {
		m.stopByUser()
	}
	cmd := m.switchProvider(idx)
	request.Reply <- ipc.Response{OK: true, State: m.providers[idx].Key, ProviderStatuses: m.providerStatuses()}
	return cmd
}

func (m *Model) providerEntryForRequest(request ipcLibraryRequest) (provider.Entry, bool) {
	key := request.Key
	if key == "" {
		key = request.Provider
	}
	entry, ok := m.ipcProvider(key)
	if !ok || entry.Provider == nil {
		return provider.Entry{}, false
	}
	return entry, true
}

// handleV2HistoryPlay replays one entry of the playback history: by index in
// the order of the history op, or by path/url match. Streams play as a
// one-entry queue, local paths play normally.
func (m *Model) handleV2HistoryPlay(jobs *ipc.JobStore, jobID string, request ipc.Request) tea.Cmd {
	if m.historyStore == nil {
		m.failV2Job(jobs, jobID, v2NotFoundError())
		return nil
	}
	entries, err := m.historyStore.Recent(0)
	if err != nil {
		m.failV2Job(jobs, jobID, v2InternalError())
		return nil
	}
	entry, ok := selectedHistoryEntry(entries, request.Index, request.Path, request.URL)
	if !ok {
		m.failV2Job(jobs, jobID, v2NotFoundError())
		return nil
	}
	m.completeV2Job(jobs, jobID, ipc.Response{OK: true})
	return m.playTrackImmediate(entry.Track)
}

package acoustid_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	// Packages
	client "github.com/mutablelogic/go-client"
	acoustid "github.com/mutablelogic/go-metadata/pkg/acoustid"
	httpresponse "github.com/mutablelogic/go-server/pkg/httpresponse"
)

// stub answers every request with the status and body, and records the
// requests
type stub struct {
	status   int
	body     string
	requests []*http.Request
}

func (s *stub) RoundTrip(req *http.Request) (*http.Response, error) {
	s.requests = append(s.requests, req)
	return &http.Response{
		StatusCode: s.status,
		Status:     http.StatusText(s.status),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(s.body)),
		Request:    req,
	}, nil
}

// newClient returns a client whose requests the stub answers with the
// status and body
func newClient(t *testing.T, status int, body string) (*acoustid.Client, *stub) {
	t.Helper()
	s := &stub{status: status, body: body}
	c, err := acoustid.New("key", client.OptTransport(func(http.RoundTripper) http.RoundTripper { return s }))
	if err != nil {
		t.Fatal(err)
	}
	return c, s
}

// lookup is a response of a lookup, with a track, its recording, release
// group, release and medium
const lookup = `{"status": "ok", "results": [{
	"id": "9ff43b6a-4f16-427c-93c2-92307ca505e0",
	"score": 0.97,
	"recordings": [{
		"id": "cd2e7c47-16f5-46c6-a37c-a1eb7bf599ff",
		"title": "Dream Is Collapsing",
		"duration": 143.5,
		"artists": [{"id": "e0c97a4d", "name": "Hans Zimmer"}],
		"releasegroups": [{
			"id": "8ee1a1a6", "type": "Album", "secondarytypes": ["Soundtrack"],
			"title": "Inception",
			"releases": [{
				"id": "6e6c4a5b", "country": "US", "date": {"year": 2010, "month": 7, "day": 13},
				"track_count": 12, "medium_count": 1,
				"mediums": [{"format": "CD", "position": 1, "track_count": 12,
					"tracks": [{"id": "1a2b", "position": 2, "title": "Dream Is Collapsing"}]}]
			}]
		}]
	}]
}]}`

func Test_acoustid_000(t *testing.T) {
	// A lookup sends the key, fingerprint, duration in whole seconds, and
	// metadata, and returns the tracks which match, with their metadata
	c, s := newClient(t, http.StatusOK, lookup)
	results, err := c.Lookup(context.Background(), "AQAAT0mU", 143900*time.Millisecond, acoustid.MetaRecordings|acoustid.MetaReleaseGroups|acoustid.MetaReleases|acoustid.MetaTracks)
	if err != nil {
		t.Fatal(err)
	}
	req := s.requests[0]
	query := req.URL.Query()
	if req.URL.Host != "api.acoustid.org" || req.URL.Path != "/v2/lookup" || query.Get("client") != "key" || query.Get("fingerprint") != "AQAAT0mU" || query.Get("duration") != "143" || query.Get("meta") != "recordings releases releasegroups tracks" {
		t.Errorf("unexpected request %s", req.URL)
	}
	if len(results) != 1 || results[0].Score != 0.97 || len(results[0].Recordings) != 1 {
		t.Fatalf("unexpected results %+v", results)
	}
	recording := results[0].Recordings[0]
	if recording.Title != "Dream Is Collapsing" || recording.Duration != 143.5 || recording.Artists[0].Name != "Hans Zimmer" {
		t.Errorf("unexpected recording %+v", recording)
	}
	group := recording.ReleaseGroups[0]
	release := group.Releases[0]
	if group.Type != "Album" || group.SecondaryTypes[0] != "Soundtrack" || release.Date.Year != 2010 || release.Mediums[0].Tracks[0].Position != 2 {
		t.Errorf("unexpected release group %+v", group)
	}
}

func Test_acoustid_001(t *testing.T) {
	// A fingerprint which matches nothing has no tracks, and without
	// metadata, there's no meta parameter
	c, s := newClient(t, http.StatusOK, `{"status": "ok", "results": []}`)
	results, err := c.Lookup(context.Background(), "AQAAT0mU", 5*time.Second, acoustid.MetaNone)
	if err != nil {
		t.Fatal(err)
	} else if len(results) != 0 {
		t.Errorf("expected no results, got %v", results)
	}
	if s.requests[0].URL.Query().Has("meta") {
		t.Errorf("unexpected meta parameter in %s", s.requests[0].URL)
	}
}

func Test_acoustid_002(t *testing.T) {
	// AcoustID's errors, with a status which isn't ok, or an HTTP error,
	// are errors with their messages
	c, _ := newClient(t, http.StatusOK, `{"status": "error", "error": {"code": 4, "message": "invalid API key"}}`)
	if _, err := c.Lookup(context.Background(), "AQAAT0mU", 5*time.Second, acoustid.MetaRecordings); err == nil || !strings.Contains(err.Error(), "invalid API key") {
		t.Errorf("expected the error's message, got %v", err)
	}
	c, _ = newClient(t, http.StatusBadRequest, `{"status": "error", "error": {"code": 3, "message": "invalid fingerprint"}}`)
	if _, err := c.Lookup(context.Background(), "AQAAT0mU", 5*time.Second, acoustid.MetaRecordings); err == nil || !strings.Contains(err.Error(), "invalid fingerprint") {
		t.Errorf("expected the error's message, got %v", err)
	}
}

func Test_acoustid_003(t *testing.T) {
	// A client without a key, or a lookup without a fingerprint, or of less
	// than a second of audio, is a bad request, without a request
	if _, err := acoustid.New(" "); !errors.Is(err, httpresponse.ErrBadRequest) {
		t.Errorf("expected ErrBadRequest, got %v", err)
	}
	c, s := newClient(t, http.StatusOK, lookup)
	for _, test := range []struct {
		fingerprint string
		duration    time.Duration
	}{
		{"", 5 * time.Second},
		{"AQAAT0mU", 500 * time.Millisecond},
	} {
		if _, err := c.Lookup(context.Background(), test.fingerprint, test.duration, acoustid.MetaAll); !errors.Is(err, httpresponse.ErrBadRequest) {
			t.Errorf("%q, %v: expected ErrBadRequest, got %v", test.fingerprint, test.duration, err)
		}
	}
	if len(s.requests) != 0 {
		t.Errorf("unexpected requests %d", len(s.requests))
	}
}

func Test_acoustid_004(t *testing.T) {
	// The names of the metadata, as AcoustID's meta parameter has them
	for meta, want := range map[acoustid.Meta]string{
		acoustid.MetaNone:                                "",
		acoustid.MetaRecordings:                          "recordings",
		acoustid.MetaRecordingIDs | acoustid.MetaSources: "recordingids sources",
		acoustid.MetaAll:                                 "recordings recordingids releases releaseids releasegroups releasegroupids tracks compress usermeta sources",
	} {
		if got := meta.String(); got != want {
			t.Errorf("%d: got %q, want %q", meta, got, want)
		}
	}
}

func Test_acoustid_005(t *testing.T) {
	// A lookup of a track sends its identifier, rather than a fingerprint,
	// and returns it, and without an identifier, it's a bad request
	c, s := newClient(t, http.StatusOK, lookup)
	results, err := c.LookupTrack(context.Background(), "9ff43b6a-4f16-427c-93c2-92307ca505e0", acoustid.MetaRecordings)
	if err != nil {
		t.Fatal(err)
	}
	query := s.requests[0].URL.Query()
	if query.Get("trackid") != "9ff43b6a-4f16-427c-93c2-92307ca505e0" || query.Get("client") != "key" || query.Has("fingerprint") || query.Get("meta") != "recordings" {
		t.Errorf("unexpected request %s", s.requests[0].URL)
	}
	if len(results) != 1 || results[0].Recordings[0].ID != "cd2e7c47-16f5-46c6-a37c-a1eb7bf599ff" {
		t.Errorf("unexpected results %+v", results)
	}
	if _, err := c.LookupTrack(context.Background(), " ", acoustid.MetaRecordings); !errors.Is(err, httpresponse.ErrBadRequest) {
		t.Errorf("expected ErrBadRequest, got %v", err)
	}
}

package tmdb_test

import (
	"bytes"
	"context"
	"image"
	"image/png"
	"io"
	"maps"
	"net/http"
	"regexp"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	// Packages
	client "github.com/mutablelogic/go-client"
	metadata "github.com/mutablelogic/go-metadata"
	meta "github.com/mutablelogic/go-metadata/internal/meta"
	tmdb "github.com/mutablelogic/go-metadata/pkg/tmdb"
	_ "github.com/mutablelogic/go-metadata/video/tmdb"
)

// namedReader is a reader with a file name
type namedReader struct {
	*strings.Reader
	name string
}

func (r namedReader) Name() string { return r.name }

// noResults is a search which has no results
const noResults = `{"page":1,"results":[],"total_pages":1,"total_results":0}`

// stub answers TMDB's endpoints with the responses, by path, or that the
// resource isn't found, and records the requests, as their paths and
// queries
type stub struct {
	sync.Mutex
	responses map[string]string
	requests  []string
}

func (s *stub) RoundTrip(req *http.Request) (*http.Response, error) {
	s.Lock()
	defer s.Unlock()
	request := req.URL.Path
	if query := req.URL.Query(); len(query) > 0 {
		request += "?" + query.Encode()
	}
	s.requests = append(s.requests, request)
	status, body, contentType := http.StatusOK, s.responses[req.URL.Path], "application/json"
	if body == "" {
		status, body = http.StatusNotFound, `{"status_code":34,"status_message":"The resource you requested could not be found."}`
	} else if strings.HasPrefix(req.URL.Path, "/t/p/") {
		contentType = "image/png"
	}
	return &http.Response{
		StatusCode: status,
		Header:     http.Header{"Content-Type": []string{contentType}},
		Body:       io.NopCloser(strings.NewReader(body)),
		Request:    req,
	}, nil
}

// Requests returns the requests, and forgets them
func (s *stub) Requests() []string {
	s.Lock()
	defer s.Unlock()
	requests := s.requests
	s.requests = nil
	return requests
}

// newClient returns a TMDB client whose requests the stub answers with the
// responses, by path
func newClient(t *testing.T, responses map[string]string) (*tmdb.Client, *stub) {
	t.Helper()
	s := &stub{responses: responses}
	c, err := tmdb.New("token", client.OptTransport(func(http.RoundTripper) http.RoundTripper { return s }))
	if err != nil {
		t.Fatal(err)
	}
	return c, s
}

// lookup returns the tmdb metadata of an empty video with the name, by key,
// with the options
func lookup(t *testing.T, name string, opts ...metadata.Option) map[string]string {
	t.Helper()
	opts = append([]metadata.Option{metadata.WithContentType("video/mp4"), metadata.WithNamespace("tmdb")}, opts...)
	_, meta, err := metadata.Extract(context.Background(), namedReader{strings.NewReader(""), name}, opts...)
	if err != nil {
		t.Fatal(err)
	}
	result := make(map[string]string, len(meta))
	for _, m := range meta {
		if m.NS() == "tmdb" {
			result[m.Key()] = m.Value()
		}
	}
	return result
}

func Test_tmdb_000(t *testing.T) {
	// A movie is searched for by its title and year, and extras aren't
	// looked up
	c, s := newClient(t, map[string]string{"/3/search/movie": noResults, "/3/search/tv": noResults})
	tests := []struct {
		name string
		want []string
	}{
		// A movie, with its year from its folder, which is searched for in
		// any year, when none was released in the year
		{"Movies/Close Encounters of the Third Kind (1977)/Close.Encounters.of.the.Third.Kind.mp4", []string{
			"/3/search/movie?primary_release_year=1977&query=Close+Encounters+of+the+Third+Kind",
			"/3/search/movie?query=Close+Encounters+of+the+Third+Kind",
		}},

		// An episode, with its season from its folder
		{"TV/Seinfeld/Season 7/12 The Caddy.m4v", []string{"/3/search/tv?query=Seinfeld"}},

		// Extras, by their name and their folder, aren't looked up
		{"TV/Silicon Valley/Season 4/Season 4 Trailer.mp4", nil},
		{"The.Matrix.1999.Sample.mkv", nil},
		{"Movies/Round Midnight/Featurettes/Dexter Gordon - Fried Bananas (1969).mkv", nil},
		{"TV/Hit & Miss/Extras/Extra 10.mp4", nil},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := lookup(t, test.name, metadata.WithTMDB(c)); len(got) != 0 {
				t.Errorf("got %v, want nothing", got)
			}
			if got := s.Requests(); !slices.Equal(got, test.want) {
				t.Errorf("requests %v, want %v", got, test.want)
			}
		})
	}

	t.Run("WithoutClient", func(t *testing.T) {
		if got := lookup(t, "The Matrix (1999).mp4"); len(got) != 0 {
			t.Errorf("got %v, want nothing", got)
		}
	})

	t.Run("NilClient", func(t *testing.T) {
		if _, _, err := metadata.Extract(context.Background(), strings.NewReader(""), metadata.WithTMDB(nil)); err == nil {
			t.Error("expected an error")
		}
	})
}

// tags are the tags of the test videos, by name, which the test handler
// returns in the video namespace, as the handler of videos does
var tags = map[string]map[string]any{
	"Episode.mp4":    {"show": "The West Wing", "season-number": "1", "episode-sort": "11", "episode-id": "111", "date": "2000"},
	"S03E05.mp4":     {"show": "Seinfeld"},
	"Untagged.mp4":   {"show": "Seinfeld", "season-number": "2"},
	"Movie.mp4":      {"date": "2000-05-01T00:00:00Z"},
	"Short.mp4":      {"duration": 4 * time.Minute},
	"Long.mp4":       {"duration": 90 * time.Minute},
	"Not A Year.mp4": {"date": "unknown"},

	// Movies whose titles are the same, by their durations
	"100/Ambition.mp4": {"duration": 100 * time.Minute},
	"120/Ambition.mp4": {"duration": 120 * time.Minute},
	"60/Ambition.mp4":  {"duration": 60 * time.Minute},

	// An episode, by its duration
	"22/The Office S02E01.mp4": {"duration": 22 * time.Minute},

	// A show's video, without an episode, by its duration
	"The Office/Season 2/Pilot.mp4": {"duration": 22 * time.Minute},

	// Episodes whose runtimes on TMDB are far from their durations
	"60/The Office S02E01.mp4": {"duration": 60 * time.Minute},
	"35/The Office S02E01.mp4": {"duration": 35 * time.Minute},

	// Episodes tagged with their season before them, and in the whole show
	"Season Prefixed.mp4":              {"show": "Show", "season-number": "2", "episode-sort": "206"},
	"Show/Season 5/12 The Episode.m4v": {"show": "Show", "season-number": "5", "episode-sort": "76"},
}

// addTagsHandler registers a handler of test videos which returns their
// tags, and a title, as the handler of videos does
var addTagsHandler = sync.OnceFunc(func() {
	metadata.AddHandler(regexp.MustCompile(`^video/x-test-tags$`), "tags", func(_ context.Context, r io.Reader, _ *metadata.Opts) ([]metadata.Metadata, error) {
		result := append(metadata.KindMetadata("video"), meta.New("dc", "title", "Title From Tags"))
		for key, value := range tags[r.(metadata.NamedReader).Name()] {
			result = append(result, meta.New("video", key, value))
		}
		return result, nil
	}, "video", "dc", "metadata")
})

func Test_tmdb_001(t *testing.T) {
	// The tags of a video are in preference to its name
	addTagsHandler()
	c, s := newClient(t, map[string]string{"/3/search/movie": noResults, "/3/search/tv": noResults})
	opts := []metadata.Option{metadata.WithTMDB(c), metadata.WithContentType("video/x-test-tags")}

	tests := []struct {
		name string
		want []string
	}{
		// An episode, by its show's tags, but not its date, which isn't the
		// year the show was first aired
		{"Episode.mp4", []string{"/3/search/tv?query=The+West+Wing"}},
		{"S03E05.mp4", []string{"/3/search/tv?query=Seinfeld"}},

		// A show without an episode is searched for as a show
		{"Untagged.mp4", []string{"/3/search/tv?query=Seinfeld"}},

		// A movie's year, from its date
		{"Movie.mp4", []string{"/3/search/movie?primary_release_year=2000&query=Movie", "/3/search/movie?query=Movie"}},
		{"Not A Year.mp4", []string{"/3/search/movie?query=Not+A+Year"}},

		// A video shorter than five minutes isn't looked up
		{"Short.mp4", nil},
		{"Long.mp4", []string{"/3/search/movie?query=Long"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			lookup(t, test.name, opts...)
			if got := s.Requests(); !slices.Equal(got, test.want) {
				t.Errorf("requests %v, want %v", got, test.want)
			}
		})
	}
}

func Test_tmdb_002(t *testing.T) {
	// A movie or an episode by its identifier, without a search, whatever
	// the video's name
	c, s := newClient(t, map[string]string{
		"/3/movie/603":                  `{"id":603,"title":"The Matrix","release_date":"1999-03-30","overview":"Neo","runtime":136,"imdb_id":"tt0133093"}`,
		"/3/tv/1396":                    `{"id":1396,"name":"Breaking Bad","episode_run_time":[47]}`,
		"/3/tv/1396/season/1/episode/2": `{"id":62086,"name":"Cat's in the Bag...","air_date":"2008-01-27","season_number":1,"episode_number":2,"external_ids":{"imdb_id":"tt1054724"}}`,
	})

	tests := []struct {
		name string
		opt  metadata.Option
		want map[string]string
	}{
		{"Movie", metadata.WithTMDBMovie(603), map[string]string{
			"id": "603", "title": "The Matrix", "release_date": "1999-03-30", "overview": "Neo", "runtime": "2h16m0s", "imdb_id": "tt0133093",
		}},

		// The episode's runtime is the show's typical one, when it isn't
		// known
		{"Episode", metadata.WithTMDBEpisode(1396, 1, 2), map[string]string{
			"id": "62086", "show_id": "1396", "show": "Breaking Bad", "season": "1", "episode": "2",
			"title": "Cat's in the Bag...", "release_date": "2008-01-27", "runtime": "47m0s", "imdb_id": "tt1054724",
		}},

		// A movie or an episode which TMDB doesn't have has no metadata,
		// and isn't an error
		{"MovieNotFound", metadata.WithTMDBMovie(1), map[string]string{}},
		{"EpisodeNotFound", metadata.WithTMDBEpisode(1396, 9, 99), map[string]string{}},
		{"ShowNotFound", metadata.WithTMDBEpisode(1, 1, 1), map[string]string{}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := lookup(t, "Something Else (1970).mp4", metadata.WithTMDB(c), test.opt); !maps.Equal(got, test.want) {
				t.Errorf("got %v, want %v", got, test.want)
			}
		})
	}

	// No search was made
	for _, request := range s.Requests() {
		if strings.HasPrefix(request, "/3/search/") {
			t.Errorf("searched %s", request)
		}
	}
}

func Test_tmdb_003(t *testing.T) {
	// The movie is the first of the search's top results whose title is the
	// video's, and whose runtime is its duration, within 10%
	addTagsHandler()
	search := `{"page":1,"results":[
		{"id":1,"title":"Ambition Road"},
		{"id":2,"title":"Ambition"},
		{"id":3,"title":"Ambition","original_title":"Ambición"},
		{"id":4,"title":"AMBITION!"},
		{"id":5,"title":"Ambition"},
		{"id":6,"title":"Ambition"}
	],"total_pages":1,"total_results":6}`
	details := map[string]string{
		"/3/search/movie": search,
		"/3/movie/1":      `{"id":1,"title":"Ambition Road","runtime":100}`,
		"/3/movie/2":      `{"id":2,"title":"Ambition","runtime":30}`,
		"/3/movie/3":      `{"id":3,"title":"Ambition","runtime":95}`,
		"/3/movie/4":      `{"id":4,"title":"AMBITION!","runtime":120}`,
		"/3/movie/6":      `{"id":6,"title":"Ambition","runtime":60}`,
	}

	tests := []struct {
		name string
		want string
		gets []string
	}{
		// The first whose title is the name, within 10%, of the results
		// whose titles are the name: 2 is too short, and 3 is within 10%
		{"100/Ambition.mp4", "3", []string{"/3/movie/2", "/3/movie/3"}},

		// Punctuation and case don't matter
		{"120/Ambition.mp4", "4", []string{"/3/movie/2", "/3/movie/3", "/3/movie/4"}},

		// Without the duration, the first whose title is the name
		{"Ambition.mp4", "2", []string{"/3/movie/2"}},

		// None within 10% of the top results, which aren't found, or which
		// are beyond them, such as 6
		{"60/Ambition.mp4", "", []string{"/3/movie/2", "/3/movie/3", "/3/movie/4", "/3/movie/5"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			c, s := newClient(t, details)
			got := lookup(t, test.name, metadata.WithTMDB(c), metadata.WithContentType("video/x-test-tags"))
			if got["id"] != test.want {
				t.Errorf("got %q, want %q", got["id"], test.want)
			}
			var gets []string
			for _, request := range s.Requests() {
				if strings.HasPrefix(request, "/3/movie/") {
					gets = append(gets, request)
				}
			}
			if !slices.Equal(gets, test.gets) {
				t.Errorf("got %v, want %v", gets, test.gets)
			}
		})
	}
}

// yearStub answers a search for a movie in a year with no results, and in
// any year with the results, which are the movie's details too
type yearStub struct {
	*stub
}

func (s yearStub) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.URL.Query().Has("primary_release_year") {
		saved := s.responses
		s.responses = map[string]string{"/3/search/movie": noResults}
		defer func() { s.responses = saved }()
	}
	return s.stub.RoundTrip(req)
}

func Test_tmdb_004(t *testing.T) {
	// A movie which wasn't released in the year of the video is searched
	// for in any year, but is one released within a year of it, such as
	// Sexy Beast, which premiered in 2000, and was released in 2001
	s := yearStub{&stub{responses: map[string]string{
		"/3/search/movie": `{"page":1,"results":[
			{"id":1,"title":"Sexy Beast","release_date":"2010-01-01"},
			{"id":2,"title":"Sexy Beast","release_date":"2001-01-12"}
		],"total_pages":1,"total_results":2}`,
		"/3/movie/1": `{"id":1,"title":"Sexy Beast","release_date":"2010-01-01"}`,
		"/3/movie/2": `{"id":2,"title":"Sexy Beast","release_date":"2001-01-12"}`,
	}}}
	c, err := tmdb.New("token", client.OptTransport(func(http.RoundTripper) http.RoundTripper { return s }))
	if err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]string{
		"Sexy Beast (2000).mp4": "2",
		"Sexy Beast (1990).mp4": "",
		"Sexy Beast.mp4":        "1",
	} {
		if got := lookup(t, name, metadata.WithTMDB(c)); got["id"] != want {
			t.Errorf("%s: got %q, want %q", name, got["id"], want)
		}
	}
}

func Test_tmdb_005(t *testing.T) {
	// The episode is of the first of the search's top shows whose name is
	// the video's, which has the episode, and whose episode's runtime is the
	// video's duration, within 10%, or else the show's typical runtime
	addTagsHandler()
	responses := map[string]string{
		"/3/search/tv": `{"page":1,"results":[
			{"id":1,"name":"The Office (Extended)"},
			{"id":2,"name":"The Office","original_name":"The Office","first_air_date":"2001-07-09"},
			{"id":3,"name":"The Office","first_air_date":"2005-03-24"},
			{"id":4,"name":"The Office","first_air_date":"2024-01-01"}
		],"total_pages":1,"total_results":4}`,
		"/3/tv/2":                     `{"id":2,"name":"The Office","first_air_date":"2001-07-09","episode_run_time":[30]}`,
		"/3/tv/2/season/2/episode/1":  `{"id":21,"name":"Merger","season_number":2,"episode_number":1,"runtime":30}`,
		"/3/tv/3":                     `{"id":3,"name":"The Office","first_air_date":"2005-03-24","episode_run_time":[22]}`,
		"/3/tv/3/season/2/episode/1":  `{"id":31,"name":"The Dundies","season_number":2,"episode_number":1}`,
		"/3/tv/3/season/5/episode/20": `{"id":32,"name":"New Boss","season_number":5,"episode_number":20,"runtime":22}`,
		"/3/tv/4":                     `{"id":4,"name":"The Office"}`,
		"/3/tv/4/season/5/episode/20": `{"id":41,"name":"Something","season_number":5,"episode_number":20}`,
	}

	tests := []struct {
		name string
		want map[string]string
	}{
		// The first show with the episode, within 10%: the second show's
		// episode is too long, and the third's runtime is its show's
		{"22/The Office S02E01.mp4", map[string]string{
			"id": "31", "show_id": "3", "show": "The Office", "season": "2", "episode": "1", "title": "The Dundies", "runtime": "22m0s",
		}},

		// Without the duration, the first show with the episode
		{"The Office S02E01.mp4", map[string]string{
			"id": "21", "show_id": "2", "show": "The Office", "season": "2", "episode": "1", "title": "Merger", "runtime": "30m0s",
		}},

		// The first show which has the episode
		{"The Office S05E20.mp4", map[string]string{
			"id": "32", "show_id": "3", "show": "The Office", "season": "5", "episode": "20", "title": "New Boss", "runtime": "22m0s",
		}},

		// No show has the episode
		{"The Office S09E99.mp4", map[string]string{}},

		// No show's episode is within 10%, when it's the first show whose
		// episode is within 25%, as TMDB's runtimes of episodes are often
		// rounded, or else none
		{"35/The Office S02E01.mp4", map[string]string{
			"id": "21", "show_id": "2", "show": "The Office", "season": "2", "episode": "1", "title": "Merger", "runtime": "30m0s",
		}},
		{"60/The Office S02E01.mp4", map[string]string{}},

		// A show's video, without an episode, is the first show, whose
		// typical runtime is the video's duration, within 10%
		{"The Office/Season 1/Pilot.mp4", map[string]string{"show_id": "2", "show": "The Office", "release_date": "2001-07-09"}},
		{"The Office/Season 2/Pilot.mp4", map[string]string{"show_id": "3", "show": "The Office", "release_date": "2005-03-24"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			c, _ := newClient(t, responses)
			if got := lookup(t, test.name, metadata.WithTMDB(c), metadata.WithContentType("video/x-test-tags")); !maps.Equal(got, test.want) {
				t.Errorf("got %v, want %v", got, test.want)
			}
		})
	}
}

func Test_tmdb_006(t *testing.T) {
	// A tagged episode with its season before it is the season's, and an
	// episode tagged in the whole show is the one its name has, when the
	// show doesn't have the tagged one
	addTagsHandler()
	c, s := newClient(t, map[string]string{
		"/3/search/tv":                `{"page":1,"results":[{"id":7,"name":"Show"}],"total_pages":1,"total_results":1}`,
		"/3/tv/7":                     `{"id":7,"name":"Show"}`,
		"/3/tv/7/season/2/episode/6":  `{"id":76,"name":"Six","season_number":2,"episode_number":6}`,
		"/3/tv/7/season/5/episode/12": `{"id":512,"name":"Twelve","season_number":5,"episode_number":12}`,
	})
	opts := []metadata.Option{metadata.WithTMDB(c), metadata.WithContentType("video/x-test-tags")}

	tests := []struct {
		name     string
		id       string
		episodes []string
	}{
		{"Season Prefixed.mp4", "76", []string{"/3/tv/7/season/2/episode/6"}},
		{"Show/Season 5/12 The Episode.m4v", "512", []string{"/3/tv/7/season/5/episode/76", "/3/tv/7/season/5/episode/12"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := lookup(t, test.name, opts...); got["id"] != test.id {
				t.Errorf("got %q, want %q", got["id"], test.id)
			}
			var episodes []string
			for _, request := range s.Requests() {
				if strings.Contains(request, "/episode/") {
					episodes = append(episodes, strings.SplitN(request, "?", 2)[0])
				}
			}
			if !slices.Equal(episodes, test.episodes) {
				t.Errorf("got %v, want %v", episodes, test.episodes)
			}
		})
	}
}

func Test_tmdb_007(t *testing.T) {
	// The title, date, description and identifier of a movie, show or
	// episode from TMDB are in place of the video's own, from its tags
	addTagsHandler()
	c, _ := newClient(t, map[string]string{
		"/3/movie/603":                  `{"id":603,"title":"The Matrix","release_date":"1999-03-30","overview":"Neo","imdb_id":"tt0133093"}`,
		"/3/tv/1396":                    `{"id":1396,"name":"Breaking Bad","first_air_date":"2008-01-20","overview":"Walter","external_ids":{"imdb_id":"tt0903747"}}`,
		"/3/tv/1396/season/1/episode/2": `{"id":62086,"name":"Cat's in the Bag...","season_number":1,"episode_number":2,"air_date":"2008-01-27","overview":"Jesse","external_ids":{"imdb_id":"tt1054724"}}`,
		"/3/search/tv":                  `{"page":1,"results":[{"id":1396,"name":"Breaking Bad"}],"total_pages":1,"total_results":1}`,
		"/3/search/movie":               noResults,
	})
	tests := []struct {
		name string
		opt  metadata.Option
		want []string
	}{
		{"Movie.mp4", metadata.WithTMDBMovie(603), []string{"The Matrix", "1999-03-30", "Neo", "tt0133093"}},
		{"Movie.mp4", metadata.WithTMDBEpisode(1396, 1, 2), []string{"Breaking Bad – Season 1 – Cat's in the Bag...", "2008-01-27", "Jesse", "tt1054724"}},
		{"Breaking Bad/Season 1/Pilot.mp4", nil, []string{"Breaking Bad", "2008-01-20", "Walter", "tt0903747"}},

		// Without a match, the video's own title
		{"Unknown Movie.mp4", nil, []string{"Title From Tags"}},
	}
	for _, test := range tests {
		opts := []metadata.Option{metadata.WithTMDB(c), metadata.WithContentType("video/x-test-tags"), metadata.WithNamespace("dc")}
		if test.opt != nil {
			opts = append(opts, test.opt)
		}
		_, meta, err := metadata.Extract(context.Background(), namedReader{strings.NewReader(""), test.name}, opts...)
		if err != nil {
			t.Fatal(err)
		}
		var titles []string
		for _, key := range []string{"title", "date", "description", "identifier"} {
			for _, m := range meta {
				if m.NS() == "dc" && m.Key() == key {
					titles = append(titles, m.Value())
				}
			}
		}
		if !slices.Equal(titles, test.want) {
			t.Errorf("%s: got %q, want %q", test.name, titles, test.want)
		}
	}
}

func Test_tmdb_008(t *testing.T) {
	// The kind of a movie is a film, and of a show's video an episode, in
	// place of the video's own kind, from its tags
	addTagsHandler()
	c, _ := newClient(t, map[string]string{
		"/3/movie/603":                  `{"id":603,"title":"The Matrix"}`,
		"/3/tv/1396":                    `{"id":1396,"name":"Breaking Bad"}`,
		"/3/tv/1396/season/1/episode/2": `{"id":62086,"name":"Cat's in the Bag...","season_number":1,"episode_number":2}`,
		"/3/search/tv":                  `{"page":1,"results":[{"id":1396,"name":"Breaking Bad"}],"total_pages":1,"total_results":1}`,
		"/3/search/movie":               noResults,
	})
	tests := []struct {
		name string
		opt  metadata.Option
		want []string
	}{
		{"Movie.mp4", metadata.WithTMDBMovie(603), []string{"film"}},
		{"Movie.mp4", metadata.WithTMDBEpisode(1396, 1, 2), []string{"episode"}},
		{"Breaking Bad/Season 1/Pilot.mp4", nil, []string{"episode"}},

		// Without a match, the video's own kind
		{"Unknown Movie.mp4", nil, []string{"video"}},
	}
	for _, test := range tests {
		opts := []metadata.Option{metadata.WithTMDB(c), metadata.WithContentType("video/x-test-tags"), metadata.WithNamespace("metadata", "dc")}
		if test.opt != nil {
			opts = append(opts, test.opt)
		}
		_, meta, err := metadata.Extract(context.Background(), namedReader{strings.NewReader(""), test.name}, opts...)
		if err != nil {
			t.Fatal(err)
		}
		var kinds, types []string
		for _, m := range meta {
			switch m.NS() + ":" + m.Key() {
			case "metadata:kind":
				kinds = append(kinds, m.Value())
			case "dc:type":
				types = append(types, m.Value())
			}
		}
		if !slices.Equal(kinds, test.want) || !slices.Equal(types, []string{"MovingImage"}) {
			t.Errorf("%s: got %q %q, want %q", test.name, kinds, types, test.want)
		}
	}
}

func Test_tmdb_009(t *testing.T) {
	// A movie, episode or show as Markdown, only when the markdown
	// namespace is requested
	c, _ := newClient(t, map[string]string{
		"/3/movie/603":                  `{"id":603,"title":"The Matrix","original_title":"The Matrix","release_date":"1999-03-30","overview":"Neo.\n\nThe *One*.","runtime":136,"imdb_id":"tt0133093"}`,
		"/3/tv/1396":                    `{"id":1396,"name":"Breaking Bad","first_air_date":"2008-01-20","overview":"Walter","number_of_seasons":5,"episode_run_time":[47]}`,
		"/3/tv/1396/season/1/episode/2": `{"id":62086,"name":"Cat's in the Bag...","season_number":1,"episode_number":2,"air_date":"2008-01-27","overview":"Jesse","external_ids":{"imdb_id":"tt1054724"}}`,
		"/3/search/tv":                  `{"page":1,"results":[{"id":1396,"name":"Breaking Bad"}],"total_pages":1,"total_results":1}`,
	})
	tests := []struct {
		name string
		opt  metadata.Option
		want string
	}{
		{"Movie.mp4", metadata.WithTMDBMovie(603), "# The Matrix\n\n" +
			"- **Released:** 1999-03-30\n" +
			"- **Runtime:** 2h 16m\n" +
			"- **TMDB:** [603](https://www.themoviedb.org/movie/603)\n" +
			"- **IMDb:** [tt0133093](https://www.imdb.com/title/tt0133093/)\n" +
			"\nNeo.\n\nThe \\*One\\*.\n"},
		{"Movie.mp4", metadata.WithTMDBEpisode(1396, 1, 2), "# Breaking Bad – Season 1 – Cat's in the Bag...\n\n" +
			"- **Show:** Breaking Bad\n" +
			"- **Season:** 1\n" +
			"- **Episode:** 2\n" +
			"- **Title:** Cat's in the Bag...\n" +
			"- **Aired:** 2008-01-27\n" +
			"- **Runtime:** 47m\n" +
			"- **TMDB:** [62086](https://www.themoviedb.org/tv/1396/season/1/episode/2)\n" +
			"- **IMDb:** [tt1054724](https://www.imdb.com/title/tt1054724/)\n" +
			"\nJesse\n"},
		{"Breaking Bad/Season 1/Pilot.mp4", nil, "# Breaking Bad\n\n" +
			"- **First aired:** 2008-01-20\n" +
			"- **Seasons:** 5\n" +
			"- **TMDB:** [1396](https://www.themoviedb.org/tv/1396)\n" +
			"\nWalter\n"},
	}
	for _, test := range tests {
		opts := []metadata.Option{metadata.WithTMDB(c), metadata.WithContentType("video/mp4"), metadata.WithNamespace("markdown"), metadata.WithRange(":")}
		if test.opt != nil {
			opts = append(opts, test.opt)
		}
		_, meta, err := metadata.Extract(context.Background(), namedReader{strings.NewReader(""), test.name}, opts...)
		if err != nil {
			t.Fatal(err)
		}
		var body string
		for _, m := range meta {
			if m.NS() == "markdown" && m.Key() == "body" {
				body = m.Value()
			}
		}
		if body != test.want {
			t.Errorf("%s: got\n%s\nwant\n%s", test.name, body, test.want)
		}
	}

	// Without the markdown namespace requested, there's no Markdown
	_, meta, err := metadata.Extract(context.Background(), namedReader{strings.NewReader(""), "Movie.mp4"}, metadata.WithTMDB(c), metadata.WithContentType("video/mp4"), metadata.WithTMDBMovie(603))
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range meta {
		if m.NS() == "markdown" {
			t.Errorf("unexpected %s:%s", m.NS(), m.Key())
		}
	}
}

// pngImage returns a PNG image of the width and height
func pngImage(t *testing.T, width, height int) string {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewGray(image.Rect(0, 0, width, height))); err != nil {
		t.Fatal(err)
	}
	return buf.String()
}

func Test_tmdb_010(t *testing.T) {
	// The images of a movie, or of an episode and its show, as artwork, only
	// when the artwork namespace is requested, in the smallest sizes whose
	// largest dimension is at least the largest image dimension, downscaled
	// to fit it. An image which can't be downloaded isn't returned.
	c, s := newClient(t, map[string]string{
		"/3/movie/603":                  `{"id":603,"title":"The Matrix","poster_path":"/poster.png","backdrop_path":"/missing.png"}`,
		"/3/tv/1396":                    `{"id":1396,"name":"Breaking Bad","poster_path":"/poster.png","backdrop_path":"/backdrop.png"}`,
		"/3/tv/1396/season/1/episode/2": `{"id":62086,"name":"Cat's in the Bag...","season_number":1,"episode_number":2,"still_path":"/still.png"}`,
		"/t/p/w185/poster.png":          pngImage(t, 185, 278),
		"/t/p/w300/backdrop.png":        pngImage(t, 300, 169),
		"/t/p/w300/still.png":           pngImage(t, 300, 169),
	})
	tests := []struct {
		name     string
		opt      metadata.Option
		want     map[string][2]int
		requests []string
	}{
		{"Movie", metadata.WithTMDBMovie(603), map[string][2]int{"poster": {170, 256}}, []string{"/t/p/w185/poster.png", "/t/p/w300/missing.png"}},
		{"Episode", metadata.WithTMDBEpisode(1396, 1, 2), map[string][2]int{"still": {256, 144}, "poster": {170, 256}, "backdrop": {256, 144}}, []string{"/t/p/w300/still.png", "/t/p/w185/poster.png", "/t/p/w300/backdrop.png"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			s.Requests()
			_, meta, err := metadata.Extract(context.Background(), namedReader{strings.NewReader(""), "Video.mp4"},
				metadata.WithTMDB(c), metadata.WithContentType("video/mp4"), metadata.WithNamespace("artwork"), metadata.WithMaxImageDimension(256), test.opt)
			if err != nil {
				t.Fatal(err)
			}
			got := make(map[string][2]int)
			for _, m := range meta {
				if m.NS() == "artwork" && m.Image() != nil {
					got[m.Key()] = [2]int{m.Image().Bounds().Dx(), m.Image().Bounds().Dy()}
				}
			}
			if !maps.Equal(got, test.want) {
				t.Errorf("got %v, want %v", got, test.want)
			}
			var images []string
			for _, request := range s.Requests() {
				if strings.HasPrefix(request, "/t/p/") {
					images = append(images, request)
				}
			}
			if !slices.Equal(images, test.requests) {
				t.Errorf("requests %v, want %v", images, test.requests)
			}
		})
	}

	// Without the artwork namespace requested, no image is downloaded
	s.Requests()
	lookup(t, "Video.mp4", metadata.WithTMDB(c), metadata.WithTMDBMovie(603))
	for _, request := range s.Requests() {
		if strings.HasPrefix(request, "/t/p/") {
			t.Errorf("unexpected request %s", request)
		}
	}
}

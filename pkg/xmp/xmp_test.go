package xmp_test

import (
	"bytes"
	"encoding/json"
	"image"
	"math"
	"os"
	"strings"
	"testing"
	"time"

	metadata "github.com/mutablelogic/go-metadata"
	geo "github.com/mutablelogic/go-metadata/pkg/geo"
	rational "github.com/mutablelogic/go-metadata/pkg/rational"
	xmp "github.com/mutablelogic/go-metadata/pkg/xmp"
)

type testMeta struct {
	key string
	val string
}

// NS returns the namespace of the key, such as "dc" of "dc:title", or an
// empty string when it has none
func (m testMeta) NS() string {
	if ns, _, ok := strings.Cut(m.key, ":"); ok {
		return ns
	}
	return ""
}

// Key returns the key without its namespace, such as "title" of "dc:title"
func (m testMeta) Key() string {
	if _, key, ok := strings.Cut(m.key, ":"); ok {
		return key
	}
	return m.key
}

func (m testMeta) Value() string      { return m.val }
func (m testMeta) Bytes() []byte      { return nil }
func (m testMeta) Image() image.Image { return nil }
func (m testMeta) Any() any           { return m.val }

const (
	testXMP       = "../../testfiles/sample.xmp"
	testXMPPDFx   = "../../testfiles/pdfx-xmp-example.xmp"
	testXMPRandom = "../../testfiles/random-xmp-example.xmp"
	testXMPBridge = "../../testfiles/bridge-2.xmp"
)

////////////////////////////////////////////////////////////////////////////////
// PARSE / READ

func Test_xmp_000(t *testing.T) {
	data, err := os.ReadFile(testXMP)
	if err != nil {
		t.Fatal(err)
	}
	x, err := xmp.Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(x.Items()) == 0 {
		t.Fatal("expected at least one item")
	}
	t.Log(x)
}

func Test_xmp_001(t *testing.T) {
	f, err := os.Open(testXMP)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	x, err := xmp.Read(f)
	if err != nil {
		t.Fatal(err)
	}
	if len(x.Items()) == 0 {
		t.Fatal("expected at least one item")
	}
}

func Test_xmp_002(t *testing.T) {
	_, err := xmp.Parse([]byte("not xml"))
	if err == nil {
		t.Fatal("expected error for invalid XMP")
	}
}

////////////////////////////////////////////////////////////////////////////////
// ITEMS

func Test_xmp_010(t *testing.T) {
	data, err := os.ReadFile(testXMP)
	if err != nil {
		t.Fatal(err)
	}
	x, err := xmp.Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	for _, it := range x.Items() {
		if it.Name() == "" {
			t.Error("item has empty key")
		}
		t.Logf("kind=%-6s key=%-30s value=%q", it.ItemKind(), it.Name(), it.Value())
	}
}

func Test_xmp_011(t *testing.T) {
	data, err := os.ReadFile(testXMP)
	if err != nil {
		t.Fatal(err)
	}
	x, err := xmp.Parse(data)
	if err != nil {
		t.Fatal(err)
	}

	// dc:title should be an Alt with x-default "Sample Image"
	titles := x.Get("dc:title")
	if len(titles) != 1 {
		t.Fatalf("expected 1 dc:title, got %d", len(titles))
	}
	if titles[0].ItemKind() != xmp.Alt {
		t.Errorf("dc:title: expected Alt kind, got %s", titles[0].ItemKind())
	}
	if got := titles[0].Value(); got != "Sample Image" {
		t.Errorf("dc:title value: expected %q, got %q", "Sample Image", got)
	}
}

func Test_xmp_012(t *testing.T) {
	data, err := os.ReadFile(testXMP)
	if err != nil {
		t.Fatal(err)
	}
	x, err := xmp.Parse(data)
	if err != nil {
		t.Fatal(err)
	}

	// dc:subject should be a Bag with 3 items
	subjects := x.Get("dc:subject")
	if len(subjects) != 1 {
		t.Fatalf("expected 1 dc:subject, got %d", len(subjects))
	}
	if subjects[0].ItemKind() != xmp.Bag {
		t.Errorf("dc:subject: expected Bag kind, got %s", subjects[0].ItemKind())
	}
	if n := len(subjects[0].Items()); n != 3 {
		t.Errorf("dc:subject: expected 3 members, got %d", n)
	}
}

func Test_xmp_013(t *testing.T) {
	data, err := os.ReadFile(testXMP)
	if err != nil {
		t.Fatal(err)
	}
	x, err := xmp.Parse(data)
	if err != nil {
		t.Fatal(err)
	}

	// dc:creator should be a Seq with 2 items
	creators := x.Get("dc:creator")
	if len(creators) != 1 {
		t.Fatalf("expected 1 dc:creator, got %d", len(creators))
	}
	if creators[0].ItemKind() != xmp.Seq {
		t.Errorf("dc:creator: expected Seq kind, got %s", creators[0].ItemKind())
	}
	if got, want := creators[0].Value(), "Jane Doe; John Smith"; got != want {
		t.Errorf("dc:creator value: expected %q, got %q", want, got)
	}
}

////////////////////////////////////////////////////////////////////////////////
// GET / ADD / DELETE

func Test_xmp_020(t *testing.T) {
	x := xmp.New()
	x.Add(xmp.NewItem("http://purl.org/dc/elements/1.1/", "dc", "format", "image/jpeg"))
	x.Add(xmp.NewItem("http://purl.org/dc/elements/1.1/", "dc", "format", "image/png"))

	if n := len(x.Get("dc:format")); n != 2 {
		t.Fatalf("expected 2 items, got %d", n)
	}
	if removed := x.Delete("dc:format"); removed != 2 {
		t.Errorf("Delete: expected 2 removed, got %d", removed)
	}
	if n := len(x.Items()); n != 0 {
		t.Errorf("expected empty document, got %d items", n)
	}
}

////////////////////////////////////////////////////////////////////////////////
// JSON

func Test_xmp_030(t *testing.T) {
	data, err := os.ReadFile(testXMP)
	if err != nil {
		t.Fatal(err)
	}
	x, err := xmp.Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	j, err := json.MarshalIndent(x, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	t.Log(string(j))
}

func Test_xmp_031(t *testing.T) {
	data, err := os.ReadFile(testXMP)
	if err != nil {
		t.Fatal(err)
	}
	x, err := xmp.Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	for _, it := range x.Items() {
		j, err := json.Marshal(it)
		if err != nil {
			t.Fatalf("item %q: %v", it.Name(), err)
		}
		t.Log(string(j))
	}
}

////////////////////////////////////////////////////////////////////////////////
// ENCODE / ROUND-TRIP

func Test_xmp_040(t *testing.T) {
	x := xmp.New()
	x.Add(xmp.NewItem("http://purl.org/dc/elements/1.1/", "dc", "format", "image/jpeg"))
	x.Add(xmp.NewBag("http://purl.org/dc/elements/1.1/", "dc", "subject", "foo", "bar"))
	x.Add(xmp.NewSeq("http://purl.org/dc/elements/1.1/", "dc", "creator", "Jane Doe"))
	x.Add(xmp.NewAlt("http://purl.org/dc/elements/1.1/", "dc", "title",
		[2]string{"x-default", "My Title"}, [2]string{"de", "Mein Titel"}))

	var buf bytes.Buffer
	if err := x.Write(&buf); err != nil {
		t.Fatal(err)
	}
	t.Log(buf.String())
}

func Test_xmp_041(t *testing.T) {
	// Round-trip: parse then re-encode
	data, err := os.ReadFile(testXMP)
	if err != nil {
		t.Fatal(err)
	}
	x, err := xmp.Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := x.Write(&buf); err != nil {
		t.Fatal(err)
	}

	// Re-parse the encoded output
	x2, err := xmp.Parse(buf.Bytes())
	if err != nil {
		t.Fatalf("re-parse failed: %v\nencoded:\n%s", err, buf.String())
	}
	if got, want := len(x2.Items()), len(x.Items()); got != want {
		t.Errorf("round-trip item count: got %d, want %d", got, want)
	}
}

////////////////////////////////////////////////////////////////////////////////
// SECURITY GUARDS

func Test_xmp_070(t *testing.T) {
	// File exceeding 1 MiB should be rejected
	big := make([]byte, 1<<20+1)
	copy(big, []byte("<x:xmpmeta"))
	_, err := xmp.Parse(big)
	if err == nil {
		t.Fatal("expected error for oversized document")
	}
	t.Log("oversized:", err)
}

func Test_xmp_080_FromMetadata(t *testing.T) {
	items := []metadata.Metadata{
		testMeta{key: "dc:title", val: "Example"},
		testMeta{key: "xmp:CreateDate", val: "2026-07-12"},
		testMeta{key: "custom:foo", val: "bar"},
		testMeta{key: "plain", val: "value"},
	}

	x := xmp.FromMetadata(items)
	if x == nil {
		t.Fatal("FromMetadata returned nil")
	}
	if got, want := len(x.Items()), 4; got != want {
		t.Fatalf("items: got %d want %d", got, want)
	}

	if got := x.Get("dc:title"); len(got) != 1 || got[0].Value() != "Example" {
		t.Fatalf("dc:title not mapped correctly: %+v", got)
	}
	if got := x.Get("xmp:CreateDate"); len(got) != 1 || got[0].Value() != "2026-07-12" {
		t.Fatalf("xmp:CreateDate not mapped correctly: %+v", got)
	}
	if got := x.Get("custom:foo"); len(got) != 1 || got[0].Value() != "bar" {
		t.Fatalf("custom:foo not mapped correctly: %+v", got)
	}
	if got := x.Get("plain"); len(got) != 1 || got[0].Value() != "value" {
		t.Fatalf("plain not mapped correctly: %+v", got)
	}

	// Ensure generated XML remains valid and round-trips, including custom namespace.
	var buf bytes.Buffer
	if err := x.Write(&buf); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	x2, err := xmp.Parse(buf.Bytes())
	if err != nil {
		t.Fatalf("re-parse failed: %v", err)
	}
	if got := x2.Get("custom:foo"); len(got) != 1 || got[0].Value() != "bar" {
		t.Fatalf("custom:foo round-trip failed: %+v", got)
	}
}

func Test_xmp_081_FromMetadataWithNamespaces(t *testing.T) {
	items := []metadata.Metadata{
		testMeta{key: "audio:Duration", val: "20.7"},
		testMeta{key: "video:Duration", val: "5.3"},
	}

	ns := map[string]string{
		"audio": "urn:gomedia:audio",
		"video": "urn:gomedia:video",
	}

	x := xmp.FromMetadata(items, ns)
	if x == nil {
		t.Fatal("FromMetadata returned nil")
	}

	var buf bytes.Buffer
	if err := x.Write(&buf); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	xml := buf.String()
	if !strings.Contains(xml, `xmlns:audio="urn:gomedia:audio"`) {
		t.Fatalf("audio namespace missing in XML: %s", xml)
	}
	if !strings.Contains(xml, `xmlns:video="urn:gomedia:video"`) {
		t.Fatalf("video namespace missing in XML: %s", xml)
	}

	x2, err := xmp.Parse(buf.Bytes())
	if err != nil {
		t.Fatalf("re-parse failed: %v", err)
	}
	if got := x2.Get("audio:Duration"); len(got) != 1 || got[0].Value() != "20.7" {
		t.Fatalf("audio:Duration round-trip failed: %+v", got)
	}
	if got := x2.Get("video:Duration"); len(got) != 1 || got[0].Value() != "5.3" {
		t.Fatalf("video:Duration round-trip failed: %+v", got)
	}
}

func Test_xmp_082_FromMetadataDuplicateKeysToSeq(t *testing.T) {
	items := []metadata.Metadata{
		testMeta{key: "dc:creator", val: "Stereolab"},
		testMeta{key: "dc:creator", val: "Tim Gane"},
		testMeta{key: "audio:Genre", val: "Post-Rock"},
		testMeta{key: "audio:Genre", val: "Kraut"},
	}

	x := xmp.FromMetadata(items, map[string]string{"audio": "urn:gomedia:audio"})
	if x == nil {
		t.Fatal("FromMetadata returned nil")
	}

	creators := x.Get("dc:creator")
	if len(creators) != 1 {
		t.Fatalf("dc:creator count: got %d want 1", len(creators))
	}
	if creators[0].ItemKind() != xmp.Seq {
		t.Fatalf("dc:creator kind: got %s want %s", creators[0].ItemKind(), xmp.Seq)
	}
	if got, want := creators[0].Value(), "Stereolab; Tim Gane"; got != want {
		t.Fatalf("dc:creator value: got %q want %q", got, want)
	}

	genres := x.Get("audio:Genre")
	if len(genres) != 1 {
		t.Fatalf("audio:Genre count: got %d want 1", len(genres))
	}
	if genres[0].ItemKind() != xmp.Seq {
		t.Fatalf("audio:Genre kind: got %s want %s", genres[0].ItemKind(), xmp.Seq)
	}
	if got, want := genres[0].Value(), "Post-Rock; Kraut"; got != want {
		t.Fatalf("audio:Genre value: got %q want %q", got, want)
	}
}

func Test_xmp_090_TypedValues(t *testing.T) {
	// The properties of XMP itself have types, and those of other schemas
	// have the types which are registered for them
	xmp.RegisterValueType("test:Duration", xmp.ValueTypeDuration)
	xmp.RegisterValueType("test:FNumber", xmp.ValueTypeRational)
	xmp.RegisterValueType("test:Latitude", xmp.ValueTypeGPSCoord)

	tItem := xmp.NewItem("http://ns.adobe.com/xap/1.0/", "xmp", "CreateDate", "2026-07-12T10:20:30Z")
	if tItem.ValueType() != xmp.ValueTypeTime {
		t.Fatalf("xmp:CreateDate type: got %v", tItem.ValueType())
	}
	if v, ok := tItem.AsTime(); !ok || v.IsZero() {
		t.Fatalf("AsTime failed: %v %v", v, ok)
	}

	dItem := xmp.NewItem("urn:test", "test", "Duration", "1088.179955")
	if dItem.ValueType() != xmp.ValueTypeDuration {
		t.Fatalf("test:Duration type: got %v", dItem.ValueType())
	}
	if d, ok := dItem.AsDuration(); !ok || d <= 0 {
		t.Fatalf("AsDuration failed: %v %v", d, ok)
	}

	bItem := xmp.NewItem("http://ns.adobe.com/xap/1.0/rights/", "xmpRights", "Marked", "true")
	if bItem.ValueType() != xmp.ValueTypeBoolean {
		t.Fatalf("xmpRights:Marked type: got %v", bItem.ValueType())
	}
	if b, ok := bItem.AsBool(); !ok || !b {
		t.Fatalf("AsBool failed: %v %v", b, ok)
	}

	rItem := xmp.NewItem("urn:test", "test", "FNumber", "35/10")
	if rItem.ValueType() != xmp.ValueTypeRational {
		t.Fatalf("test:FNumber type: got %v", rItem.ValueType())
	}
	if r, ok := rItem.AsRational(); !ok || r != rational.New(7, 2) {
		t.Fatalf("AsRational failed: %+v %v", r, ok)
	}

	gItem := xmp.NewItem("urn:test", "test", "Latitude", "51.5074N")
	if gItem.ValueType() != xmp.ValueTypeGPSCoord {
		t.Fatalf("test:Latitude type: got %v", gItem.ValueType())
	}
	if g, ok := gItem.AsGPSCoord(); !ok || math.Abs(g-51.5074) > 1e-6 {
		t.Fatalf("AsGPSCoord failed: %v %v", g, ok)
	}
	if g, ok := xmp.NewItem("urn:test", "test", "Latitude", "51,30.4440S").AsGPSCoord(); !ok || math.Abs(g+51.5074) > 1e-6 {
		t.Fatalf("AsGPSCoord failed: %v %v", g, ok)
	}

	if typed, ok := dItem.TypedValue().(time.Duration); !ok || typed <= 0 {
		t.Fatalf("TypedValue duration failed: %T %v", dItem.TypedValue(), dItem.TypedValue())
	}

	// The properties of other schemas have no types until they're
	// registered
	if typ := xmp.ValueTypeForKey("photoshop:DateCreated"); typ != xmp.ValueTypeUnknown {
		t.Fatalf("photoshop:DateCreated type: got %v", typ)
	}
}

func Test_xmp_094_TimeValues(t *testing.T) {
	// A date is in any of the forms of XMP, with or without a time zone,
	// minutes, seconds or a fraction of a second, and without a time zone
	// it's in UTC
	for _, test := range []struct {
		value string
		want  time.Time
	}{
		{"2024-01-15T10:30:00Z", time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)},
		{"2024-01-15T10:30:00+01:00", time.Date(2024, 1, 15, 9, 30, 0, 0, time.UTC)},
		{"2024-01-15T10:30:00.5-05:00", time.Date(2024, 1, 15, 15, 30, 0, 5e8, time.UTC)},
		{"2024-01-15T10:30+01:00", time.Date(2024, 1, 15, 9, 30, 0, 0, time.UTC)},
		{"2024-01-15T10:30:00", time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)},
		{"2024-01-15T10:30:00.25", time.Date(2024, 1, 15, 10, 30, 0, 25e7, time.UTC)},
		{"2024-01-15T10:30", time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)},
		{"2024-01-15", time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)},
		{"2024-01", time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)},
		{"2024", time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)},
	} {
		it := xmp.NewItem("http://ns.adobe.com/xap/1.0/", "xmp", "CreateDate", test.value)
		if got, ok := it.AsTime(); !ok || !got.Equal(test.want) {
			t.Errorf("%q: expected %v, got %v %v", test.value, test.want, got, ok)
		}
	}
	for _, value := range []string{"", "yesterday", "2024-13-01", "15/01/2024"} {
		it := xmp.NewItem("http://ns.adobe.com/xap/1.0/", "xmp", "CreateDate", value)
		if got, ok := it.AsTime(); ok {
			t.Errorf("%q: expected no time, got %v", value, got)
		}
	}

	// The date of the sample has no time zone
	data, err := os.ReadFile(testXMP)
	if err != nil {
		t.Fatal(err)
	}
	x, err := xmp.Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	if it := x.First("xmp:CreateDate"); it == nil {
		t.Fatal("expected xmp:CreateDate")
	} else if _, ok := it.TypedValue().(time.Time); !ok {
		t.Errorf("xmp:CreateDate: expected a time, got %T %v", it.TypedValue(), it.TypedValue())
	}
}

func Test_xmp_091_RegisterValueType(t *testing.T) {
	xmp.RegisterValueType("custom:is-live", xmp.ValueTypeBoolean)
	it := xmp.NewItem("urn:go-metadata:custom", "custom", "is-live", "1")
	if it.ValueType() != xmp.ValueTypeBoolean {
		t.Fatalf("custom:is-live type: got %v", it.ValueType())
	}
	if v, ok := it.AsBool(); !ok || !v {
		t.Fatalf("custom:is-live bool parse failed: %v %v", v, ok)
	}
}

func Test_xmp_071(t *testing.T) {
	// Deeply nested XML should be rejected at maxNestingDepth
	var sb strings.Builder
	sb.WriteString(`<x:xmpmeta xmlns:x="adobe:ns:meta/"><rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#"><rdf:Description rdf:about="">`)
	for i := 0; i < 70; i++ {
		sb.WriteString(`<dc:title xmlns:dc="http://purl.org/dc/elements/1.1/">`)
	}
	sb.WriteString("deep")
	for i := 0; i < 70; i++ {
		sb.WriteString(`</dc:title>`)
	}
	sb.WriteString(`</rdf:Description></rdf:RDF></x:xmpmeta>`)

	_, err := xmp.Parse([]byte(sb.String()))
	if err == nil {
		t.Fatal("expected error for deeply nested document")
	}
	t.Log("deep nesting:", err)
}

////////////////////////////////////////////////////////////////////////////////
// FIRST — priority-fallback lookup

func Test_xmp_080(t *testing.T) {
	x := xmp.New()
	x.Add(xmp.NewItem("http://ns.adobe.com/xap/1.0/", "xmp", "CreateDate", "2024-01-15"))

	// First key matches
	it := x.First("photoshop:DateCreated", "xmp:CreateDate")
	if it == nil {
		t.Fatal("expected a result")
	}
	if got := it.Value(); got != "2024-01-15" {
		t.Errorf("got %q", got)
	}
}

func Test_xmp_081(t *testing.T) {
	x := xmp.New()
	x.Add(xmp.NewItem("http://ns.adobe.com/xap/1.0/", "xmp", "CreateDate", "2024-01-15"))

	// No key matches → nil
	if got := x.First("photoshop:DateCreated", "exif:DateTimeOriginal"); got != nil {
		t.Errorf("expected nil, got %q", got.Name())
	}
}

func Test_xmp_082(t *testing.T) {
	// First against real file: dc:title exists under first preference key
	data, err := os.ReadFile(testXMP)
	if err != nil {
		t.Fatal(err)
	}
	x, err := xmp.Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	it := x.First("dc:title", "xmp:Title")
	if it == nil {
		t.Fatal("expected dc:title")
	}
	if got := it.Value(); got != "Sample Image" {
		t.Errorf("got %q", got)
	}
}

////////////////////////////////////////////////////////////////////////////////
// PDFX EXAMPLE — multiple rdf:Description blocks, unknown namespaces, xpacket padding

func Test_xmp_050(t *testing.T) {
	data, err := os.ReadFile(testXMPPDFx)
	if err != nil {
		t.Fatal(err)
	}
	x, err := xmp.Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	// Seven rdf:Description blocks should be merged into a flat item list
	if n := len(x.Items()); n != 35 {
		t.Errorf("expected 35 items from merged rdf:Description blocks, got %d", n)
	}
	for _, it := range x.Items() {
		t.Logf("kind=%-6s key=%-40s value=%q", it.ItemKind(), it.Name(), it.Value())
	}
}

func Test_xmp_051(t *testing.T) {
	data, err := os.ReadFile(testXMPPDFx)
	if err != nil {
		t.Fatal(err)
	}
	x, err := xmp.Parse(data)
	if err != nil {
		t.Fatal(err)
	}

	// Unknown prism: namespace should survive with prefix intact
	pubs := x.Get("prism:publicationName")
	if len(pubs) != 1 {
		t.Fatalf("expected 1 prism:publicationName, got %d", len(pubs))
	}
	if got := pubs[0].Value(); got != "Analytica Chimica Acta" {
		t.Errorf("prism:publicationName: got %q", got)
	}

	// pdfx:CrossMarkDomains should be a Seq
	domains := x.Get("pdfx:CrossMarkDomains")
	if len(domains) != 1 {
		t.Fatalf("expected 1 pdfx:CrossMarkDomains, got %d", len(domains))
	}
	if domains[0].ItemKind() != xmp.Seq {
		t.Errorf("pdfx:CrossMarkDomains: expected Seq, got %s", domains[0].ItemKind())
	}
	if n := len(domains[0].Items()); n != 2 {
		t.Errorf("pdfx:CrossMarkDomains: expected 2 members, got %d", n)
	}
}

func Test_xmp_052(t *testing.T) {
	// Round-trip pdfx file
	data, err := os.ReadFile(testXMPPDFx)
	if err != nil {
		t.Fatal(err)
	}
	x, err := xmp.Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := x.Write(&buf); err != nil {
		t.Fatal(err)
	}
	x2, err := xmp.Parse(buf.Bytes())
	if err != nil {
		t.Fatalf("re-parse failed: %v\nencoded:\n%s", err, buf.String())
	}
	if got, want := len(x2.Items()), len(x.Items()); got != want {
		t.Errorf("round-trip item count: got %d, want %d", got, want)
	}
}

////////////////////////////////////////////////////////////////////////////////
// RANDOM EXAMPLE — no xpacket wrapper, old xap:/xapMM: prefix aliases

func Test_xmp_060(t *testing.T) {
	data, err := os.ReadFile(testXMPRandom)
	if err != nil {
		t.Fatal(err)
	}
	x, err := xmp.Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	if n := len(x.Items()); n != 16 {
		t.Errorf("expected 16 items, got %d", n)
	}
	for _, it := range x.Items() {
		t.Logf("kind=%-6s key=%-40s value=%q", it.ItemKind(), it.Name(), it.Value())
	}
}

func Test_xmp_061(t *testing.T) {
	data, err := os.ReadFile(testXMPRandom)
	if err != nil {
		t.Fatal(err)
	}
	x, err := xmp.Parse(data)
	if err != nil {
		t.Fatal(err)
	}

	// The old xap: prefix, of the same URI as xmp:, is xmp:
	dates := x.Get("xmp:CreateDate")
	if len(dates) != 1 {
		t.Fatalf("expected 1 xmp:CreateDate, got %d", len(dates))
	}
	if got := dates[0].Value(); got != "2008-09-16T08:19:40Z" {
		t.Errorf("xmp:CreateDate: got %q", got)
	}

	// The old xapMM: prefix, of the same URI as xmpMM:, is xmpMM:
	docIDs := x.Get("xmpMM:DocumentID")
	if len(docIDs) != 1 {
		t.Fatalf("expected 1 xmpMM:DocumentID, got %d", len(docIDs))
	}
	if len(x.Get("xap:CreateDate"))+len(x.Get("xapMM:DocumentID")) != 0 {
		t.Error("expected no xap: or xapMM: properties")
	}
}

////////////////////////////////////////////////////////////////////////////////
// BRIDGE EXAMPLE — rdf:li parseType="Resource" structs, Iptc4xmpCore struct,
//                  kbrg: / stEvt: unknown-at-startup namespaces

func Test_xmp_090(t *testing.T) {
	data, err := os.ReadFile(testXMPBridge)
	if err != nil {
		t.Fatal(err)
	}
	x, err := xmp.Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	if n := len(x.Items()); n != 42 {
		t.Errorf("expected 42 items, got %d", n)
	}
	for _, it := range x.Items() {
		t.Logf("kind=%-6s key=%-45s value=%q", it.ItemKind(), it.Name(), it.Value())
	}
}

func Test_xmp_091(t *testing.T) {
	// xmpMM:History must be a Seq of Struct items, not empty strings
	data, err := os.ReadFile(testXMPBridge)
	if err != nil {
		t.Fatal(err)
	}
	x, err := xmp.Parse(data)
	if err != nil {
		t.Fatal(err)
	}

	hist := x.Get("xmpMM:History")
	if len(hist) != 1 {
		t.Fatalf("expected 1 xmpMM:History, got %d", len(hist))
	}
	if hist[0].ItemKind() != xmp.Seq {
		t.Errorf("xmpMM:History: expected Seq, got %s", hist[0].ItemKind())
	}
	entries := hist[0].Items()
	if len(entries) != 2 {
		t.Fatalf("xmpMM:History: expected 2 entries, got %d", len(entries))
	}
	for i, e := range entries {
		if e.ItemKind() != xmp.Struct {
			t.Errorf("entry %d: expected Struct, got %s", i, e.ItemKind())
		}
		if len(e.Items()) == 0 {
			t.Errorf("entry %d: struct has no fields", i)
		}
		// Each struct should contain stEvt:action
		found := false
		for _, f := range e.Items() {
			if f.Name() == "stEvt:action" {
				found = true
				t.Logf("entry %d stEvt:action=%q", i, f.Value())
			}
		}
		if !found {
			t.Errorf("entry %d: missing stEvt:action field", i)
		}
	}
}

func Test_xmp_092(t *testing.T) {
	// Iptc4xmpCore:CreatorContactInfo must be a Struct with address fields
	data, err := os.ReadFile(testXMPBridge)
	if err != nil {
		t.Fatal(err)
	}
	x, err := xmp.Parse(data)
	if err != nil {
		t.Fatal(err)
	}

	cci := x.Get("Iptc4xmpCore:CreatorContactInfo")
	if len(cci) != 1 {
		t.Fatalf("expected 1 CreatorContactInfo, got %d", len(cci))
	}
	if cci[0].ItemKind() != xmp.Struct {
		t.Errorf("CreatorContactInfo: expected Struct, got %s", cci[0].ItemKind())
	}
	if n := len(cci[0].Items()); n != 8 {
		t.Errorf("CreatorContactInfo: expected 8 fields, got %d", n)
	}
}

func Test_xmp_093(t *testing.T) {
	// Round-trip bridge file
	data, err := os.ReadFile(testXMPBridge)
	if err != nil {
		t.Fatal(err)
	}
	x, err := xmp.Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := x.Write(&buf); err != nil {
		t.Fatal(err)
	}
	x2, err := xmp.Parse(buf.Bytes())
	if err != nil {
		t.Fatalf("re-parse failed: %v\nencoded:\n%s", err, buf.String())
	}
	if got, want := len(x2.Items()), len(x.Items()); got != want {
		t.Errorf("round-trip item count: got %d, want %d", got, want)
	}
}

func Test_xmp_062(t *testing.T) {
	// Round-trip the no-xpacket file
	data, err := os.ReadFile(testXMPRandom)
	if err != nil {
		t.Fatal(err)
	}
	x, err := xmp.Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := x.Write(&buf); err != nil {
		t.Fatal(err)
	}
	x2, err := xmp.Parse(buf.Bytes())
	if err != nil {
		t.Fatalf("re-parse failed: %v\nencoded:\n%s", err, buf.String())
	}
	if got, want := len(x2.Items()), len(x.Items()); got != want {
		t.Errorf("round-trip item count: got %d, want %d", got, want)
	}
}

func Test_xmp_096_About(t *testing.T) {
	// The resource a document describes is its rdf:about, and a prefix which
	// isn't well known has a namespace of this module
	x := xmp.FromMetadata([]metadata.Metadata{testMeta{key: "image:width", val: "640"}})
	x.SetAbout("photos/a.jpg")
	if x.About() != "photos/a.jpg" {
		t.Errorf("expected photos/a.jpg, got %q", x.About())
	}
	var buf bytes.Buffer
	if err := x.Write(&buf); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`rdf:about="photos/a.jpg"`, `xmlns:image="urn:go-metadata:image"`, `<image:width>640</image:width>`} {
		if !strings.Contains(buf.String(), want) {
			t.Errorf("expected %s in %s", want, buf.String())
		}
	}
	x2, err := xmp.Parse(buf.Bytes())
	if err != nil {
		t.Fatal(err)
	} else if x2.About() != "photos/a.jpg" {
		t.Errorf("expected about to round-trip, got %q", x2.About())
	}
}

func Test_xmp_097_Sidecar(t *testing.T) {
	// A sidecar starts with its x:xmpmeta element, without the xpacket
	// wrapper, which a packet has, and both are parsed
	x := xmp.FromMetadata([]metadata.Metadata{testMeta{key: "dc:title", val: "Example"}})
	var packet, sidecar bytes.Buffer
	if err := x.Write(&packet); err != nil {
		t.Fatal(err)
	} else if err := x.WriteSidecar(&sidecar); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(packet.String(), "<?xpacket begin=") {
		t.Errorf("expected the packet to start with xpacket, got %q", packet.String()[:20])
	}
	if !strings.HasPrefix(sidecar.String(), "<x:xmpmeta") || strings.Contains(sidecar.String(), "xpacket") {
		t.Errorf("expected the sidecar to start with x:xmpmeta, got %q", sidecar.String())
	}
	for _, data := range [][]byte{packet.Bytes(), sidecar.Bytes()} {
		if x2, err := xmp.Parse(data); err != nil {
			t.Error(err)
		} else if got := x2.First("dc:title"); got == nil || got.Value() != "Example" {
			t.Errorf("expected dc:title, got %v", got)
		}
	}
}

// positionMeta is metadata whose value is a position, which marshals itself
// as the GPS properties of the exif schema
type positionMeta struct {
	ns, key string
	value   geo.LatLong
}

func (m positionMeta) NS() string         { return m.ns }
func (m positionMeta) Key() string        { return m.key }
func (m positionMeta) Value() string      { return m.value.String() }
func (m positionMeta) Bytes() []byte      { return nil }
func (m positionMeta) Image() image.Image { return nil }
func (m positionMeta) Any() any           { return m.value }

func Test_xmp_098_Marshaler(t *testing.T) {
	// A value which is a Marshaler is the properties it returns, which are
	// parsed back as a GPS coordinate, and otherwise its text, which isn't
	// escaped more than it needs to be
	ll := geo.LatLong{Lat: 53.234219, Long: -10.206025}.WithAlt(12.5)
	x := xmp.FromMetadata([]metadata.Metadata{
		positionMeta{"exif", "GPSPosition", ll},
		positionMeta{"dc", "coverage", ll},
	})
	var buf bytes.Buffer
	if err := x.WriteSidecar(&buf); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"<exif:GPSLatitude>53,14.053140N</exif:GPSLatitude>",
		"<exif:GPSLongitude>10,12.361500W</exif:GPSLongitude>",
		"<exif:GPSAltitude>1250/100</exif:GPSAltitude>",
		"<exif:GPSAltitudeRef>0</exif:GPSAltitudeRef>",
		`<dc:coverage>53°14'03.19"N 10°12'21.69"W 12.5m</dc:coverage>`,
	} {
		if !strings.Contains(buf.String(), want) {
			t.Errorf("expected %s in %s", want, buf.String())
		}
	}
	if strings.Contains(buf.String(), "GPSPosition") {
		t.Errorf("expected no exif:GPSPosition in %s", buf.String())
	}

	// The coordinates are parsed back, by the type the exif package
	// registers, or as text
	x2, err := xmp.Parse(buf.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	xmp.RegisterValueType("exif:GPSLongitude", xmp.ValueTypeGPSCoord)
	if got := x2.First("exif:GPSLongitude"); got == nil {
		t.Fatal("expected exif:GPSLongitude")
	} else if v, ok := got.AsGPSCoord(); !ok || math.Abs(v-ll.Long) > 1e-6 {
		t.Errorf("expected %v, got %v %v", ll.Long, v, ok)
	}
}

func Test_xmp_099_Metadata(t *testing.T) {
	// The properties of a document are metadata: a simple property as its
	// typed value, a localized one as its default text, and a list as its
	// values, joined as text, without structs
	x := xmp.New()
	x.Add(
		xmp.NewItem("http://ns.adobe.com/xap/1.0/", "xmp", "CreateDate", "2024-01-15T10:30:00+01:00"),
		xmp.NewAlt("http://purl.org/dc/elements/1.1/", "dc", "title", [2]string{"x-default", "Title"}, [2]string{"de", "Titel"}),
		xmp.NewSeq("http://purl.org/dc/elements/1.1/", "dc", "creator", "Ada", "Grace"),
		xmp.NewStruct("http://ns.adobe.com/exif/1.0/", "exif", "Flash"),
	)
	got := map[string]metadata.Metadata{}
	for _, m := range x.Metadata() {
		got[m.NS()+":"+m.Key()] = m
	}
	if len(got) != 3 || got["exif:Flash"] != nil {
		t.Fatalf("expected three properties, without the struct, got %v", got)
	}
	if v, ok := got["xmp:CreateDate"].Any().(time.Time); !ok || !v.Equal(time.Date(2024, 1, 15, 9, 30, 0, 0, time.UTC)) {
		t.Errorf("xmp:CreateDate: expected a time, got %v", got["xmp:CreateDate"].Any())
	}
	if got["dc:title"].Value() != "Title" {
		t.Errorf("dc:title: expected Title, got %q", got["dc:title"].Value())
	}
	if m := got["dc:creator"]; m.Value() != "Ada; Grace" || len(m.Any().([]string)) != 2 {
		t.Errorf("dc:creator: expected Ada; Grace, got %q %v", m.Value(), m.Any())
	}
}

func Test_xmp_100_Prefixes(t *testing.T) {
	// A well-known namespace has its conventional prefix, rather than the
	// prefix the document declares, such as xap of older software, so its
	// properties have their types, and another namespace has its prefix
	x, err := xmp.Parse([]byte(`<x:xmpmeta xmlns:x="adobe:ns:meta/">
  <rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#">
    <rdf:Description rdf:about=""
        xmlns:xap="http://ns.adobe.com/xap/1.0/"
        xmlns:xapMM="http://ns.adobe.com/xap/1.0/mm/"
        xmlns:custom="urn:example:custom">
      <xap:CreateDate>2004-03-01T10:00:00Z</xap:CreateDate>
      <xapMM:DocumentID>uuid:1234</xapMM:DocumentID>
      <custom:Value>42</custom:Value>
    </rdf:Description>
  </rdf:RDF>
</x:xmpmeta>`))
	if err != nil {
		t.Fatal(err)
	}
	if it := x.First("xmp:CreateDate"); it == nil {
		t.Errorf("expected xmp:CreateDate, got %v", x.Items())
	} else if _, ok := it.TypedValue().(time.Time); !ok {
		t.Errorf("expected a time, got %T", it.TypedValue())
	}
	if it := x.First("xmpMM:DocumentID"); it == nil || it.Value() != "uuid:1234" {
		t.Errorf("expected xmpMM:DocumentID, got %v", it)
	}
	if it := x.First("custom:Value"); it == nil || it.Value() != "42" {
		t.Errorf("expected custom:Value, got %v", it)
	}
	if len(x.Get("xap:CreateDate")) != 0 {
		t.Error("expected no xap:CreateDate")
	}
}

func Test_xmp_101_Identifier(t *testing.T) {
	// The document ID is the identifier too, unless there's one
	id := xmp.NewItem("http://ns.adobe.com/xap/1.0/mm/", "xmpMM", "DocumentID", "xmp.did:1234")
	identifier := xmp.NewItem("http://purl.org/dc/elements/1.1/", "dc", "identifier", "isbn:9780000000000")
	for _, test := range []struct {
		items []*xmp.Item
		want  string
	}{
		{[]*xmp.Item{id}, "xmp.did:1234"},
		{[]*xmp.Item{id, identifier}, "isbn:9780000000000"},
		{[]*xmp.Item{}, ""},
	} {
		x := xmp.New()
		x.Add(test.items...)
		var got []string
		for _, m := range x.Metadata() {
			if m.NS() == "dc" && m.Key() == "identifier" {
				got = append(got, m.Value())
			}
		}
		if test.want == "" && len(got) != 0 {
			t.Errorf("expected no dc:identifier, got %v", got)
		} else if test.want != "" && (len(got) != 1 || got[0] != test.want) {
			t.Errorf("expected dc:identifier %q once, got %v", test.want, got)
		}
	}
}

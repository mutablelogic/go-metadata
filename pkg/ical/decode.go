package ical

import (
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
	"time"

	// Packages
	metadata "github.com/mutablelogic/go-metadata"
	contentline "github.com/mutablelogic/go-metadata/internal/contentline"
	geo "github.com/mutablelogic/go-metadata/pkg/geo"

	// The zone database, for the zones of events, when the system has none,
	// such as in a container
	_ "time/tzdata"
)

////////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	// MaxLineSize is the longest line which is read, once it's unfolded,
	// which includes an image of several megabytes, as base64
	MaxLineSize = 16 << 20
)

var (
	// reDuration matches a duration, such as P1D, PT1H30M or P2W
	reDuration = regexp.MustCompile(`^([+-])?P(?:(\d+)W)?(?:(\d+)D)?(?:T(?:(\d+)H)?(?:(\d+)M)?(?:(\d+)S)?)?$`)

	// reOffset matches a zone which is an offset, such as +02:00, -0530,
	// UTC+2 or GMT-03:00, which some calendars use as a TZID
	reOffset = regexp.MustCompile(`^(?:UTC|GMT)?([+-])(\d{1,2})(?::?(\d{2}))?$`)
)

////////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Decode returns the calendar of r, which is iCalendar 2.0, with its events
// and tasks, without its other components, such as alarms and zones, or an
// empty calendar when there's none. An event without a start, or a task
// without a start or when it's due, isn't returned. When a calendar has more
// than one VCALENDAR, they're one calendar.
func Decode(r io.Reader) (*Calendar, error) {
	calendar := new(Calendar)
	var stack []string
	var event []contentline.Property
	lines := contentline.NewReader(r, MaxLineSize)
	for {
		line, err := lines.Next()
		if err == io.EOF {
			break
		} else if err != nil {
			return calendar, metadata.ErrBadParameter.Withf("ical: %v", err)
		}
		p, ok := contentline.Parse(line)
		if !ok {
			continue
		}
		switch p.Name {
		case "BEGIN":
			stack = append(stack, strings.ToUpper(strings.TrimSpace(p.Value)))
			if len(stack) == 2 {
				// An event or task, whose properties are read until it ends,
				// but not those of a component within it, such as an alarm
				event = nil
			}
		case "END":
			if len(stack) == 2 && stack[0] == "VCALENDAR" && (stack[1] == "VEVENT" || stack[1] == "VTODO") {
				if e, ok := parseEvent(stack[1], event); ok {
					calendar.Events = append(calendar.Events, e)
				}
			}
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
		default:
			switch {
			case len(stack) == 1 && stack[0] == "VCALENDAR":
				calendar.set(p)
			case len(stack) == 2 && (stack[1] == "VEVENT" || stack[1] == "VTODO"):
				event = append(event, p)
			}
		}
	}
	return calendar, nil
}

// UnmarshalText sets the calendar from iCalendar 2.0 (see Decode)
func (c *Calendar) UnmarshalText(data []byte) error {
	calendar, err := Decode(strings.NewReader(string(data)))
	if err != nil {
		return err
	}
	*c = *calendar
	return nil
}

////////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// set sets the field of the calendar of a property: its name, from
// RFC 7986's NAME, or Apple's and Google's X-WR-CALNAME, its description, its
// zone, and its image
func (c *Calendar) set(p contentline.Property) {
	switch p.Name {
	case "PRODID":
		c.ProdID = unescape(p.Value)
	case "NAME", "X-WR-CALNAME":
		if c.Name == "" {
			c.Name = unescape(p.Value)
		}
	case "DESCRIPTION", "X-WR-CALDESC":
		if c.Description == "" {
			c.Description = unescape(p.Value)
		}
	case "X-WR-TIMEZONE":
		c.TZ = unescape(p.Value)
	case "IMAGE":
		if c.Image == nil {
			c.Image = imageOf(p)
		}
	}
}

// parseEvent returns an event or task from the properties of its component,
// with a start, or for a task, when it's due, or false when it has neither.
// Its end is its end, or when a task with a start is due, or its start and
// its duration, or the day after an all-day start, or its start, so a task
// which is due on a day, without a start, is that day.
func parseEvent(component string, properties []contentline.Property) (*Event, bool) {
	e := &Event{Kind: KindEvent}
	if component == "VTODO" {
		e.Kind = KindTask
	}
	var duration string
	var hasEnd bool
	for _, p := range properties {
		switch p.Name {
		case "UID":
			e.UID = unescape(p.Value)
		case "SUMMARY":
			e.Summary = unescape(p.Value)
		case "DESCRIPTION":
			e.Description = unescape(p.Value)
		case "LOCATION":
			e.Location = unescape(p.Value)
		case "GEO":
			if lat, long, found := strings.Cut(p.Value, ";"); found {
				if ll, err := geo.Parse(strings.TrimSpace(lat) + "," + strings.TrimSpace(long)); err == nil {
					e.Geo = &ll
				}
			}
		case "X-APPLE-STRUCTURED-LOCATION":
			// A place on a map, as a geo URI, such as geo:51.5007,-0.1246,
			// which is the position when there's no GEO
			if e.Geo == nil {
				e.Geo = geoURI(p.Value)
			}
		case "URL":
			e.URL = strings.TrimSpace(p.Value)
		case "DTSTART":
			e.Start, _ = parseTime(p)
		case "DTEND":
			e.End, hasEnd = parseTime(p)
		case "DUE":
			e.Due, _ = parseTime(p)
		case "DURATION":
			duration = p.Value
		case "RRULE":
			e.RRule = strings.TrimSpace(p.Value)
		case "EXDATE":
			for _, value := range strings.Split(p.Value, ",") {
				if t, ok := parseTime(contentline.Property{Params: p.Params, Value: value}); ok {
					e.ExDates = append(e.ExDates, t)
				}
			}
		case "RECURRENCE-ID":
			e.RecurrenceID, _ = parseTime(p)
		case "STATUS":
			e.Status = strings.ToUpper(unescape(p.Value))
		case "ORGANIZER":
			e.Organizer = address(p.Value)
		case "ATTENDEE":
			if attendee := address(p.Value); attendee != "" {
				e.Attendees = append(e.Attendees, attendee)
			}
		case "CATEGORIES":
			e.Categories = append(e.Categories, nonEmpty(contentline.Split(p.Value, ','))...)
		case "TRANSP":
			e.Transparent = strings.EqualFold(strings.TrimSpace(p.Value), "TRANSPARENT")
		case "DTSTAMP":
			e.Stamp = parseInstant(p.Value)
		case "CREATED":
			e.Created = parseInstant(p.Value)
		case "LAST-MODIFIED":
			e.Modified = parseInstant(p.Value)
		case "IMAGE":
			if e.Image == nil {
				e.Image = imageOf(p)
			}
		case "ATTACH":
			if e.Image == nil && strings.HasPrefix(strings.ToLower(first(p.Params["FMTTYPE"])), "image/") {
				e.Image = imageOf(p)
			}
		case "X-APPLE-OMIT-YEAR":
			e.OmitYear, _ = strconv.Atoi(strings.TrimSpace(p.Value))
		}
	}

	// The start, which a task can do without, when it's due
	startIsDue := false
	if e.Start.IsZero() && e.Kind == KindTask && !e.Due.IsZero() {
		e.Start, startIsDue = e.Due, true
	} else if e.Start.IsZero() {
		return nil, false
	}

	// The end
	switch {
	case hasEnd:
	case e.Kind == KindTask && !e.Due.IsZero() && !startIsDue:
		e.End = e.Due
	case duration != "":
		e.End = addDuration(e.Start, duration)
	case e.Start.Date:
		e.End = addDuration(e.Start, "P1D")
	default:
		e.End = e.Start
	}
	return e, true
}

// parseTime returns a date, or a time, in UTC, when it ends with Z, or in
// its zone, by its TZID, which is a zone in the database, such as
// Europe/Berlin, or an offset, such as +02:00, or else floating, which is
// in UTC, keeping its TZID, such as a Windows zone, or false when it isn't
// one
func parseTime(p contentline.Property) (Time, bool) {
	value := strings.TrimSpace(p.Value)
	if strings.EqualFold(first(p.Params["VALUE"]), "DATE") || len(value) == len(dateLayout) {
		date, err := time.Parse(dateLayout, value)
		if err != nil {
			return Time{}, false
		}
		return Time{Time: date, Date: true}, true
	}
	if utc, found := strings.CutSuffix(value, "Z"); found {
		t, err := time.Parse(timeLayout, utc)
		return Time{Time: t}, err == nil
	}
	tzid := strings.TrimSpace(first(p.Params["TZID"]))
	if location, ok := Location(tzid); ok {
		t, err := time.ParseInLocation(timeLayout, value, location)
		return Time{Time: t, TZID: tzid}, err == nil
	}
	t, err := time.Parse(timeLayout, value)
	return Time{Time: t, TZID: tzid, Floating: true}, err == nil
}

// Location returns the zone of a TZID: a zone in the database, such as
// Europe/Berlin, or an offset, such as +02:00, which some calendars use, as
// a zone of that name, or false when it's neither
func Location(tzid string) (*time.Location, bool) {
	tzid = strings.TrimPrefix(strings.TrimSpace(tzid), "/")
	if tzid == "" {
		return nil, false
	}
	if match := reOffset.FindStringSubmatch(strings.ToUpper(tzid)); match != nil {
		hours, _ := strconv.Atoi(match[2])
		minutes, _ := strconv.Atoi(match[3])
		if hours > 18 || minutes > 59 {
			return nil, false
		}
		offset := hours*3600 + minutes*60
		if match[1] == "-" {
			offset = -offset
		}
		return time.FixedZone(fmt.Sprintf("%s%02d:%02d", match[1], hours, minutes), offset), true
	}
	if location, err := time.LoadLocation(tzid); err == nil {
		return location, true
	}
	return nil, false
}

// addDuration returns a date or time after the duration, such as P1D or
// PT1H30M, or the date or time when the duration isn't one. A date only adds
// the duration's weeks and days. A time adds its weeks and days in its zone,
// so a day is the same time the next day, across a change to or from
// daylight saving time, and then its hours, minutes and seconds.
func addDuration(t Time, duration string) Time {
	match := reDuration.FindStringSubmatch(strings.ToUpper(strings.TrimSpace(duration)))
	if match == nil {
		return t
	}
	n := func(i int) int {
		v, _ := strconv.Atoi(match[i])
		if match[1] == "-" {
			return -v
		}
		return v
	}
	result := t
	result.Time = t.Time.AddDate(0, 0, n(2)*7+n(3))
	if !t.Date {
		result.Time = result.Time.Add(time.Duration(n(4))*time.Hour + time.Duration(n(5))*time.Minute + time.Duration(n(6))*time.Second)
	}
	return result
}

// parseInstant returns a time in UTC, such as a stamp, or zero when it
// isn't one
func parseInstant(value string) time.Time {
	value = strings.TrimSpace(value)
	for _, layout := range []string{timeLayout + "Z", timeLayout, dateLayout} {
		if t, err := time.Parse(layout, value); err == nil {
			return t
		}
	}
	return time.Time{}
}

// imageOf returns the image of an IMAGE or ATTACH property, which is
// binary, base64, or a data URI, with its media type, or a link
func imageOf(p contentline.Property) *Image {
	base64Encoded := strings.EqualFold(first(p.Params["ENCODING"]), "BASE64") || strings.EqualFold(first(p.Params["VALUE"]), "BINARY")
	return parseImage(p.Value, base64Encoded, first(p.Params["FMTTYPE"]), strings.ToUpper(first(p.Params["DISPLAY"])))
}

// geoURI returns the position of a geo URI, such as geo:51.5007,-0.1246,
// whose parameters, after a semicolon, aren't kept, or nil when it isn't one
// Reference: https://www.rfc-editor.org/rfc/rfc5870
func geoURI(value string) *geo.LatLong {
	value = strings.TrimSpace(value)
	if len(value) < len("geo:") || !strings.EqualFold(value[:len("geo:")], "geo:") {
		return nil
	}
	coordinates, _, _ := strings.Cut(value[len("geo:"):], ";")
	ll, err := geo.Parse(coordinates)
	if err != nil {
		return nil
	}
	return &ll
}

// address returns the email address of an organizer or attendee, from a
// mailto: URI, or the value itself
func address(value string) string {
	value = strings.TrimSpace(value)
	if len(value) > len("mailto:") && strings.EqualFold(value[:len("mailto:")], "mailto:") {
		return value[len("mailto:"):]
	}
	return value
}

// unescape returns a text value unescaped, without its spaces
func unescape(value string) string {
	return strings.TrimSpace(contentline.Unescape(value))
}

// first returns the first value, or an empty string
func first(values []string) string {
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

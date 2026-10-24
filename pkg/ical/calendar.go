// Package ical defines an iCalendar, a calendar of events and tasks, which
// is encoded and decoded as iCalendar 2.0, and its images, which can be
// embedded in it, as artwork.
// Reference: https://www.rfc-editor.org/rfc/rfc5545
package ical

import (
	"time"

	// Packages
	geo "github.com/mutablelogic/go-metadata/pkg/geo"
)

////////////////////////////////////////////////////////////////////////////////
// TYPES

// Calendar is a calendar of events and tasks
type Calendar struct {
	ProdID      string   `json:"prodid,omitempty"`      // The product which wrote it
	Name        string   `json:"name,omitempty"`        // Its name, such as Birthdays
	Description string   `json:"description,omitempty"` // Its description
	TZ          string   `json:"tz,omitempty"`          // Its zone, such as Europe/Berlin
	Image       *Image   `json:"image,omitempty"`       // Its image, embedded or linked
	Events      []*Event `json:"events,omitempty"`      // Its events and tasks
}

// Kind is the kind of an event: an event, or a task
type Kind string

// Event is an event, or a task, of a calendar
type Event struct {
	Kind         Kind         `json:"kind"`                   // event or task
	UID          string       `json:"uid,omitempty"`          // Its unique identifier
	Summary      string       `json:"summary,omitempty"`      // Its title
	Description  string       `json:"description,omitempty"`  // Its description
	Location     string       `json:"location,omitempty"`     // Where it is
	Geo          *geo.LatLong `json:"geo,omitempty"`          // The position of where it is
	URL          string       `json:"url,omitempty"`          // A link about it
	Start        Time         `json:"start,omitzero"`         // When it starts
	End          Time         `json:"end,omitzero"`           // When it ends, which is exclusive
	Due          Time         `json:"due,omitzero"`           // When a task is due
	RRule        string       `json:"rrule,omitempty"`        // How it repeats, such as FREQ=YEARLY
	ExDates      []Time       `json:"exdates,omitempty"`      // The starts of a repeating event which it excludes
	RecurrenceID Time         `json:"recurrence_id,omitzero"` // The start of an occurrence it changes
	Status       string       `json:"status,omitempty"`       // Such as CONFIRMED or CANCELLED
	Organizer    string       `json:"organizer,omitempty"`    // The email address of its organizer
	Attendees    []string     `json:"attendees,omitempty"`    // The email addresses of its attendees
	Categories   []string     `json:"categories,omitempty"`   // Its categories, such as Birthdays
	Transparent  bool         `json:"transparent,omitempty"`  // Whether it doesn't make its time busy
	Image        *Image       `json:"image,omitempty"`        // Its image, embedded or linked
	OmitYear     int          `json:"omit_year,omitempty"`    // The year of a start whose year isn't known, such as 1604 for a birthday
	Stamp        time.Time    `json:"stamp,omitzero"`         // When it was written
	Created      time.Time    `json:"created,omitzero"`       // When it was created
	Modified     time.Time    `json:"last_modified,omitzero"` // When it was last changed
}

// Time is a date, or a time, which is in UTC, in a zone, by its TZID, such
// as Europe/Berlin, or floating, which is the same time in every zone, as
// is a time in a zone which isn't known, such as a Windows zone
type Time struct {
	Time     time.Time `json:"time"`
	Date     bool      `json:"date,omitempty"`     // A date, without a time
	TZID     string    `json:"tzid,omitempty"`     // The zone of a time
	Floating bool      `json:"floating,omitempty"` // A time without a zone, or in one which isn't known
}

////////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	KindEvent Kind = "event" // A VEVENT
	KindTask  Kind = "task"  // A VTODO
)

////////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// IsZero returns true when there's no time
func (t Time) IsZero() bool {
	return t.Time.IsZero()
}

// Date returns a date, without a time
func Date(year int, month time.Month, day int) Time {
	return Time{Time: time.Date(year, month, day, 0, 0, 0, 0, time.UTC), Date: true}
}

// UTC returns a time in UTC
func UTC(t time.Time) Time {
	return Time{Time: t.UTC()}
}

// In returns a time in a zone, which is written with its TZID, such as
// Europe/Berlin
func In(t time.Time, location *time.Location) Time {
	return Time{Time: t.In(location), TZID: location.String()}
}

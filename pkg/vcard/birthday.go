package vcard

import (
	"strings"
	"time"

	// Packages
	ical "github.com/mutablelogic/go-metadata/pkg/ical"
)

////////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	// OmitYear is the year of a birthday whose year isn't known, which is a
	// leap year, so the 29th of February is a date, as Apple's contacts
	// write it, with X-APPLE-OMIT-YEAR, so a calendar doesn't show an age
	OmitYear = 1604

	// BirthdayCategory is the category of a birthday's event
	BirthdayCategory = "Birthdays"

	// birthdaySuffix is after a contact's UID, as its birthday's UID
	birthdaySuffix = "/birthday"
)

////////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// BirthdayCalendar returns the contact's birthday as a calendar, with an
// all-day event which repeats every year, from the day they were born, or a
// leap year when it isn't known, and whose UID is their UID and /birthday,
// so it's the same each time, in the Birthdays category, so it's known to
// be a birthday, and which doesn't make the day busy. A birthday on the
// 29th of February is on the last day of February each year. The event's
// stamp is when the contact was last changed, or the Unix epoch, so the
// calendar is the same each time it's written, and its image is their
// photo, when it's embedded. Returns nil when they have no birthday with a
// month and a day.
// Reference: https://www.rfc-editor.org/rfc/rfc5545
func (c *Card) BirthdayCalendar() *ical.Calendar {
	d := c.Birthday
	if d == nil || d.Month < 1 || d.Month > 12 || d.Day < 1 || d.Day > 31 {
		return nil
	}
	year := d.Year
	if year <= 0 {
		year = OmitYear
	}
	event := &ical.Event{
		Kind:        ical.KindEvent,
		UID:         strings.TrimSpace(c.UID) + birthdaySuffix,
		Start:       ical.Date(year, time.Month(d.Month), d.Day),
		RRule:       "FREQ=YEARLY",
		Summary:     c.DisplayName() + "'s birthday",
		Categories:  []string{BirthdayCategory},
		Transparent: true,
		Stamp:       c.Rev,
	}
	if strings.TrimSpace(c.UID) == "" {
		event.UID = ""
	}
	if d.Month == 2 && d.Day == 29 {
		event.RRule = "FREQ=YEARLY;BYMONTH=2;BYMONTHDAY=-1"
	}
	if d.Year <= 0 {
		event.OmitYear = OmitYear
	}
	if c.Photo != nil && len(c.Photo.Data) > 0 {
		event.Image = &ical.Image{MediaType: c.Photo.MediaType, Data: c.Photo.Data, Display: "BADGE"}
	}
	return &ical.Calendar{Events: []*ical.Event{event}}
}

package ical

import (
	// Packages
	metadata "github.com/mutablelogic/go-metadata"
)

////////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	// ContentType is the content type of an iCalendar
	ContentType = "text/calendar"

	// TaskType and EventType are the content types of an iCalendar whose
	// components are all tasks, VTODO, or all events, VEVENT, with the
	// component parameter of RFC 5545, so a client can tell them apart
	// without reading them
	TaskType  = ContentType + "; component=VTODO"
	EventType = ContentType + "; component=VEVENT"
)

////////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

func init() {
	// An iCalendar is text, which is refined by its extension, so it's
	// detected the same on every platform, whatever its MIME types are
	for _, ext := range []string{".ics", ".ical", ".ifb"} {
		if err := metadata.AddTextExtensionType(ext, ContentType); err != nil {
			panic(err)
		}
	}
}

////////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// ContentType returns the content type of the calendar: TaskType when its
// components are all tasks, EventType when they're all events, or else
// ContentType, when they're both, or it has none (see Components)
func (c *Calendar) ContentType() string {
	events, tasks := c.Components()
	switch {
	case tasks > 0 && events == 0:
		return TaskType
	case events > 0 && tasks == 0:
		return EventType
	default:
		return ContentType
	}
}

// Components returns how many events and tasks the calendar has: those of
// its Events, as they are now, and those it was decoded of which aren't in
// Events, such as an event without a start
func (c *Calendar) Components() (events, tasks int) {
	events, tasks = c.dropped.events, c.dropped.tasks
	for _, e := range c.Events {
		if e.Kind == KindTask {
			tasks++
		} else {
			events++
		}
	}
	return events, tasks
}

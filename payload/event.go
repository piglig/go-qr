package payload

import (
	"strings"
	"time"
)

// Event is an iCalendar VEVENT that phone scanners offer to add to the
// calendar. Times are written in UTC. For an all-day event only the dates
// of Start and End are used; End is exclusive and defaults to the day after
// Start.
//
// Format reference: RFC 5545 §3.6.1.
type Event struct {
	Summary     string
	Location    string
	Description string
	Start       time.Time
	End         time.Time // optional
	AllDay      bool
}

const (
	icalDateTime = "20060102T150405Z"
	icalDate     = "20060102"
)

// icalEscaper escapes TEXT values per RFC 5545 §3.3.11.
var icalEscaper = strings.NewReplacer(`\`, `\\`, `;`, `\;`, `,`, `\,`, "\r\n", `\n`, "\n", `\n`)

func (e Event) String() string {
	var sb strings.Builder
	line := func(key, value string) {
		sb.WriteString(key)
		sb.WriteByte(':')
		sb.WriteString(value)
		sb.WriteString("\r\n")
	}
	text := func(key, value string) {
		if value != "" {
			line(key, icalEscaper.Replace(value))
		}
	}

	line("BEGIN", "VEVENT")
	text("SUMMARY", e.Summary)
	if e.AllDay {
		end := e.End
		if end.IsZero() || !end.After(e.Start) {
			end = e.Start.AddDate(0, 0, 1)
		}
		line("DTSTART;VALUE=DATE", e.Start.Format(icalDate))
		line("DTEND;VALUE=DATE", end.Format(icalDate))
	} else {
		line("DTSTART", e.Start.UTC().Format(icalDateTime))
		if !e.End.IsZero() {
			line("DTEND", e.End.UTC().Format(icalDateTime))
		}
	}
	text("LOCATION", e.Location)
	text("DESCRIPTION", e.Description)
	sb.WriteString("END:VEVENT")
	return sb.String()
}

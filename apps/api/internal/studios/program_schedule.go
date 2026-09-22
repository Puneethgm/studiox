package studios

import (
	"regexp"
	"strconv"
	"strings"
)

// ProgramSessionEntry is one parsed day of a week-by-week program document
// (e.g. an "8 Week Challenge" PDF/docx) — see ParseProgramSchedule.
type ProgramSessionEntry struct {
	WeekNumber  int
	DayOfWeek   int // ISO: Monday=1 .. Sunday=7
	DayName     string
	SessionName string
	Progression string
	KeyFocus    string
}

var isoWeekday = map[string]int{
	"Monday": 1, "Tuesday": 2, "Wednesday": 3, "Thursday": 4,
	"Friday": 5, "Saturday": 6, "Sunday": 7,
}

// dayNamePattern matches any of the seven weekday names, used both to find
// day-block boundaries and as a literal in weekHeaderPattern.
var dayNamesAlt = "Monday|Tuesday|Wednesday|Thursday|Friday|Saturday|Sunday"

var (
	weekHeaderPattern = regexp.MustCompile(`(?m)^\s*Week\s+(\d+)\s*$`)
	dayHeaderPattern  = regexp.MustCompile(`(?m)^\s*(` + dayNamesAlt + `)\s*$`)
	sessionLinePattern = regexp.MustCompile(`(?is)Session:\s*(.+?)\s*(?:\r?\n|$)`)
	progressionPattern = regexp.MustCompile(`(?is)Progression\s*&\s*changes:\s*(.+?)\s*(?:\r?\n|$)`)
	keyFocusPattern    = regexp.MustCompile(`(?is)Key focus:\s*(.*)$`)
)

// ParseProgramSchedule looks for a "Week N" / day-of-week / "Session: ..." /
// "Progression & changes: ..." / "Key focus: ..." structure in a knowledge
// base document's raw text (the format produced by this studio's weekly
// program notes — see docs/decisions or the Knowledge Base admin page for an
// example) and extracts one entry per day found. Returns nil if the text
// doesn't match this structure at all — callers should treat that as "not a
// program schedule document" rather than an error; most knowledge base
// uploads (FAQs, policies, architecture docs, ...) legitimately don't match.
//
// This exists so the AI worker can answer "what's today's session" with a
// real, code-computed lookup (see Repo.GetProgramSession) instead of asking
// an LLM to find the right day among many near-identical week-by-week
// entries via semantic search, which reliably picks the wrong week — see
// the "Week 4 Tuesday" investigation this was built from.
func ParseProgramSchedule(text string) []ProgramSessionEntry {
	weekIdx := weekHeaderPattern.FindAllStringSubmatchIndex(text, -1)
	if len(weekIdx) == 0 {
		return nil
	}

	var entries []ProgramSessionEntry
	for wi, wm := range weekIdx {
		weekNum, err := strconv.Atoi(text[wm[2]:wm[3]])
		if err != nil {
			continue
		}
		blockStart := wm[1]
		blockEnd := len(text)
		if wi+1 < len(weekIdx) {
			blockEnd = weekIdx[wi+1][0]
		}
		weekBlock := text[blockStart:blockEnd]

		dayIdx := dayHeaderPattern.FindAllStringSubmatchIndex(weekBlock, -1)
		for di, dm := range dayIdx {
			dayName := weekBlock[dm[2]:dm[3]]
			dayStart := dm[1]
			dayEnd := len(weekBlock)
			if di+1 < len(dayIdx) {
				dayEnd = dayIdx[di+1][0]
			}
			dayBlock := weekBlock[dayStart:dayEnd]

			sessionMatch := sessionLinePattern.FindStringSubmatch(dayBlock)
			if sessionMatch == nil {
				// A day header with no "Session:" line isn't this format —
				// skip rather than record a garbage entry.
				continue
			}
			progressionMatch := progressionPattern.FindStringSubmatch(dayBlock)
			keyFocusMatch := keyFocusPattern.FindStringSubmatch(dayBlock)

			entry := ProgramSessionEntry{
				WeekNumber:  weekNum,
				DayOfWeek:   isoWeekday[dayName],
				DayName:     dayName,
				SessionName: strings.TrimSpace(sessionMatch[1]),
			}
			if progressionMatch != nil {
				entry.Progression = strings.TrimSpace(progressionMatch[1])
			}
			if keyFocusMatch != nil {
				entry.KeyFocus = strings.TrimSpace(keyFocusMatch[1])
			}
			entries = append(entries, entry)
		}
	}
	return entries
}

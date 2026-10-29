package answer

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Places come from the system's tz database: every zone's city
// ("Asia/Tokyo" is tokyo), every country by name, and the abbreviations
// and big cities that have no zone of their own.
const zoneinfo = "/usr/share/zoneinfo"

var placeAliases = map[string]string{
	"utc": "UTC", "gmt": "Etc/GMT", "z": "UTC",
	"ist": "Asia/Kolkata", "india": "Asia/Kolkata", "delhi": "Asia/Kolkata", "new delhi": "Asia/Kolkata",
	"mumbai": "Asia/Kolkata", "bangalore": "Asia/Kolkata", "bengaluru": "Asia/Kolkata", "chennai": "Asia/Kolkata",
	"hyderabad": "Asia/Kolkata", "guwahati": "Asia/Kolkata", "pune": "Asia/Kolkata",
	"pst": "America/Los_Angeles", "pdt": "America/Los_Angeles", "pt": "America/Los_Angeles",
	"san francisco": "America/Los_Angeles", "sf": "America/Los_Angeles", "seattle": "America/Los_Angeles", "la": "America/Los_Angeles",
	"mst": "America/Denver", "mdt": "America/Denver",
	"cst": "America/Chicago", "cdt": "America/Chicago", "dallas": "America/Chicago", "houston": "America/Chicago",
	"est": "America/New_York", "edt": "America/New_York", "et": "America/New_York", "nyc": "America/New_York",
	"boston": "America/New_York", "washington": "America/New_York", "miami": "America/New_York",
	"bst": "Europe/London", "uk": "Europe/London", "england": "Europe/London",
	"cet": "Europe/Paris", "cest": "Europe/Paris", "eet": "Europe/Athens",
	"jst": "Asia/Tokyo", "kst": "Asia/Seoul", "aest": "Australia/Sydney", "sgt": "Asia/Singapore",
	"beijing": "Asia/Shanghai", "china": "Asia/Shanghai", "usa": "America/New_York", "us": "America/New_York",
	"dubai": "Asia/Dubai", "uae": "Asia/Dubai", "munich": "Europe/Berlin", "frankfurt": "Europe/Berlin",
	"barcelona": "Europe/Madrid", "milan": "Europe/Rome", "osaka": "Asia/Tokyo", "toronto": "America/Toronto",
	// countries across several zones: where most people are, not the
	// first zone the tz table lists
	"australia": "Australia/Sydney", "canada": "America/Toronto", "brazil": "America/Sao_Paulo",
	"russia": "Europe/Moscow", "germany": "Europe/Berlin", "mexico": "America/Mexico_City",
	"indonesia": "Asia/Jakarta", "argentina": "America/Argentina/Buenos_Aires", "spain": "Europe/Madrid",
	"portugal": "Europe/Lisbon", "chile": "America/Santiago", "kazakhstan": "Asia/Almaty",
	"united states": "America/New_York", "the us": "America/New_York", "the uk": "Europe/London",
	"united kingdom": "Europe/London", "new zealand": "Pacific/Auckland", "mongolia": "Asia/Ulaanbaatar",
	"calcutta": "Asia/Kolkata", "bombay": "Asia/Kolkata", "madras": "Asia/Kolkata", "peking": "Asia/Shanghai",
}

var places struct {
	once  sync.Once
	names map[string]string // lowercase place -> zone name
	locs  sync.Map          // zone name -> *time.Location
}

func loadPlaces() {
	names := map[string]string{}
	countries := map[string]string{} // code -> name
	if f, err := os.Open(zoneinfo + "/iso3166.tab"); err == nil {
		sc := bufio.NewScanner(f)
		for sc.Scan() {
			if code, name, ok := strings.Cut(sc.Text(), "\t"); ok && !strings.HasPrefix(code, "#") {
				countries[code] = strings.ToLower(name)
			}
		}
		f.Close()
	}
	if f, err := os.Open(zoneinfo + "/zone1970.tab"); err == nil {
		sc := bufio.NewScanner(f)
		for sc.Scan() {
			fields := strings.Split(sc.Text(), "\t")
			if len(fields) < 3 || strings.HasPrefix(fields[0], "#") {
				continue
			}
			zone := fields[2]
			city := zone[strings.LastIndexByte(zone, '/')+1:]
			names[strings.ToLower(strings.ReplaceAll(city, "_", " "))] = zone
			// a country takes its first listed zone, usually the capital's
			for _, code := range strings.Split(fields[0], ",") {
				if c := countries[code]; c != "" && names[c] == "" {
					names[c] = zone
				}
			}
		}
		f.Close()
	}
	for k, v := range placeAliases {
		names[k] = v
	}
	places.names = names
}

func location(place string) (*time.Location, string, bool) {
	places.once.Do(loadPlaces)
	zone, ok := places.names[strings.TrimSpace(place)]
	if !ok {
		return nil, "", false
	}
	if loc, ok := places.locs.Load(zone); ok {
		return loc.(*time.Location), zone, true
	}
	loc, err := time.LoadLocation(zone)
	if err != nil {
		return nil, "", false
	}
	places.locs.Store(zone, loc)
	return loc, zone, true
}

// timeIn answers "time in tokyo", "tokyo time", "what time is it in london",
// "3pm in tokyo", "3pm london to new york", "15:00 utc in ist".
func timeIn(q string, now time.Time) (Answer, bool) {
	q = strings.TrimSuffix(strings.TrimPrefix(q, "what "), "?")
	for _, lead := range []string{"the ", "current ", "local "} {
		q = strings.TrimPrefix(q, lead)
	}
	q = strings.Replace(q, "time now in ", "time in ", 1)
	for _, p := range []string{"time is it in ", "time in ", "time at "} {
		if place, ok := strings.CutPrefix(q, p); ok {
			if loc, zone, ok := location(place); ok {
				return clockAnswer(now, now.In(loc), zone, place), true
			}
		}
	}
	if place, ok := strings.CutSuffix(q, " time"); ok {
		if loc, zone, ok := location(place); ok {
			return clockAnswer(now, now.In(loc), zone, place), true
		}
	}

	// a clock time and where it is going: "3pm in tokyo"
	words := strings.Fields(q)
	h, m, used := clock(words)
	if used == 0 || used >= len(words) {
		return Answer{}, false
	}
	rest := strings.Join(words[used:], " ")
	from, to := now.Location(), ""
	fromName := "here"
	for _, sep := range []string{" in ", " to "} {
		if a, b, ok := strings.Cut(" "+rest, sep); ok {
			to = b
			if a = strings.TrimSpace(a); a != "" {
				loc, _, ok := location(a)
				if !ok {
					return Answer{}, false
				}
				from, fromName = loc, titleCase(a)
			}
			break
		}
	}
	target, zone, ok := location(to)
	if !ok {
		return Answer{}, false
	}
	y, mo, d := now.In(from).Date()
	at := time.Date(y, mo, d, h, m, 0, 0, from)
	a := clockAnswer(at, at.In(target), zone, to)
	a.Hint = fmt.Sprintf("%02d:%02d %s · %s", h, m, fromName, a.Hint)
	return a, true
}

// clock reads "3pm", "3 pm", "3:30pm", "15:00", "noon", "midnight".
func clock(words []string) (h, m, used int) {
	if len(words) == 0 {
		return 0, 0, 0
	}
	switch words[0] {
	case "noon":
		return 12, 0, 1
	case "midnight":
		return 0, 0, 1
	}
	w, used := words[0], 1
	if len(words) > 1 && (words[1] == "am" || words[1] == "pm") {
		w, used = w+words[1], 2
	}
	suffix := ""
	if s, ok := strings.CutSuffix(w, "am"); ok {
		w, suffix = s, "am"
	} else if s, ok := strings.CutSuffix(w, "pm"); ok {
		w, suffix = s, "pm"
	}
	hs, ms, hasMinutes := strings.Cut(w, ":")
	// digits only: "3:-5" and "-3:00" are no times
	if !isDigits(hs) || len(hs) > 2 || hasMinutes && (!isDigits(ms) || len(ms) != 2) {
		return 0, 0, 0
	}
	h, _ = strconv.Atoi(hs)
	if suffix == "" && !hasMinutes {
		return 0, 0, 0
	}
	if hasMinutes {
		if m, _ = strconv.Atoi(ms); m > 59 {
			return 0, 0, 0
		}
	}
	switch {
	case suffix != "" && (h < 1 || h > 12), h > 23:
		return 0, 0, 0
	case suffix == "pm" && h != 12:
		h += 12
	case suffix == "am" && h == 12:
		h = 0
	}
	return h, m, used
}

func clockAnswer(here, there time.Time, zone, place string) Answer {
	value := there.Format("15:04")
	name := value
	if there.YearDay() != here.YearDay() {
		name += " " + there.Format("Mon")
	}
	_, offset := there.Zone()
	_, ours := here.Zone()
	return Answer{
		Value: value,
		Name:  name + " in " + titleCase(place),
		Hint:  fmt.Sprintf("%s · UTC%s · %s", zone, utcOffset(offset), relative(offset-ours)),
	}
}

func utcOffset(seconds int) string {
	sign := "+"
	if seconds < 0 {
		sign, seconds = "-", -seconds
	}
	s := sign + strconv.Itoa(seconds/3600)
	if m := seconds % 3600 / 60; m != 0 {
		s += fmt.Sprintf(":%02d", m)
	}
	return s
}

func relative(seconds int) string {
	if seconds == 0 {
		return "same as you"
	}
	word := "ahead"
	if seconds < 0 {
		word, seconds = "behind", -seconds
	}
	s := strconv.Itoa(seconds/3600) + "h"
	if m := seconds % 3600 / 60; m != 0 {
		s += fmt.Sprintf(" %dm", m)
	}
	return s + " " + word
}

// titleCase: "new york" is New York, a short single word is an
// abbreviation: "ist", "nyc", "uk".
func titleCase(s string) string {
	words := strings.Fields(s)
	if len(words) == 1 && len(s) <= 3 {
		return strings.ToUpper(s)
	}
	for i, w := range words {
		words[i] = strings.ToUpper(w[:1]) + w[1:]
	}
	return strings.Join(words, " ")
}

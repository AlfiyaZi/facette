package utils

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	durationRegexp = "^([-+])?\\s*" +
		"(?:(\\d+)\\s*y(?:ears?)?)?\\s*" +
		"(?:(\\d+)\\s*mo(?:nths?)?)?\\s*" +
		"(?:(\\d+)\\s*d(?:ays?)?)?\\s*" +
		"(?:(\\d+)\\s*h(?:ours?)?)?\\s*" +
		"(?:(\\d+)\\s*m(?:inutes?)?)?\\s*" +
		"(?:(\\d+)\\s*s(?:econds?)?)?" +
		"$"
)

type durationUnit struct {
	value int
	text  string
}

// DurationToRange converts a duration into a string-defined time range.
func DurationToRange(duration time.Duration) string {
	var chunks []string

	units := []durationUnit{
		durationUnit{86400, "d"},
		durationUnit{3600, "h"},
		durationUnit{60, "m"},
		durationUnit{1, "s"},
	}

	seconds := int(math.Abs(duration.Seconds()))

	for _, unit := range units {
		count := int(math.Floor(float64(seconds / unit.value)))

		if count > 0 {
			chunks = append(chunks, fmt.Sprintf("%d%s", count, unit.text))
			seconds %= unit.value
		}
	}

	result := strings.Join(chunks, " ")

	if duration < 0 {
		result = "-" + result
	}

	return result
}

// TimeApplyRange applies a string-defined time range to a specific date.
func TimeApplyRange(refTime time.Time, input string) (time.Time, error) {
	var chunks []int

	re := regexp.MustCompile(durationRegexp)

	modifier := 1

	for key, value := range re.FindStringSubmatch(strings.Trim(input, " ")) {
		var intVal int

		if key == 0 {
			continue
		} else if key == 1 {
			if value == "-" {
				modifier = -1
			}

			continue
		}

		if value == "" {
			chunks = append(chunks, 0)
			continue
		}

		intVal, err := strconv.Atoi(value)
		if err != nil {
			return time.Time{}, fmt.Errorf("invalid range")
		}

		chunks = append(chunks, intVal*modifier)
	}

	if len(chunks) == 0 {
		return time.Time{}, fmt.Errorf("invalid range")
	}

	newTime := refTime.
		AddDate(chunks[0], chunks[1], chunks[2]).
		Add(time.Duration(chunks[3]) * time.Hour).
		Add(time.Duration(chunks[4]) * time.Minute).
		Add(time.Duration(chunks[5]) * time.Second)

	return newTime, nil
}

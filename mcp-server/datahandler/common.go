package datahandler

import "time"

func parseTimeParam(t string) (time.Time, error) {
	return time.Parse(time.RFC3339, t)
}

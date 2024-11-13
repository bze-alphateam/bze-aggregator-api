package converter

import "time"

func MillisecondsToTime(milliseconds int64) time.Time {
	if milliseconds <= 0 {
		return time.Time{}
	}

	return time.Unix(milliseconds/1000, (milliseconds%1000)*int64(time.Millisecond))
}

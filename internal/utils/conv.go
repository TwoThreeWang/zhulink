package utils

import (
	"strconv"
)

// StringToInt converts string to int, returns 0 if error
func StringToInt(s string) int {
	i, err := strconv.Atoi(s)
	if err != nil {
		return 0
	}
	return i
}

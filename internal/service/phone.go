package service

import (
	"errors"
	"strings"
)

// NormalizePhone приводит телефон к виду +7XXXXXXXXXX.
// Принимает «8 900 123-45-67», «+7(900)1234567», «79001234567» и т.п.
func NormalizePhone(raw string) (string, error) {
	var digits strings.Builder
	for _, r := range raw {
		if r >= '0' && r <= '9' {
			digits.WriteRune(r)
		}
	}
	d := digits.String()

	switch {
	case len(d) == 11 && d[0] == '8':
		d = "7" + d[1:]
	case len(d) == 10 && d[0] == '9':
		d = "7" + d
	}
	if len(d) < 10 || len(d) > 15 {
		return "", errors.New("некорректный номер телефона")
	}
	return "+" + d, nil
}

// Package bytefmt contains helper methods and constants for converting to and from a human-readable byte format.
//
//	bytefmt.ByteSize(100.5*bytefmt.MEGABYTE) // "100.5M"
//	bytefmt.ByteSize(uint64(1024)) // "1K"
//
package bytefmt

import (
	"errors"
	"strconv"
	"strings"
	"unicode"
)

const (
	BYTE = 1 << (10 * iota)
	KILOBYTE
	MEGABYTE
	GIGABYTE
	TERABYTE
)

var invalidByteQuantityError = errors.New("byte quantity must be a positive integer with a unit of measurement like M, MB, MiB, G, GiB, or GB")

// ByteSize returns a human-readable byte string of the form 10M, 12.5K, and so forth.  The following units are available:
//	T: Terabyte
//	G: Gigabyte
//	M: Megabyte
//	K: Kilobyte
//	B: Byte
// The unit that results in the smallest number greater than or equal to 1 is always chosen.
func ByteSize(bytes int64) string {
	unit := ""
	value := float64(bytes)

	switch {
	case bytes >= TERABYTE:
		unit = "T"
		value = value / TERABYTE
	case bytes >= GIGABYTE:
		unit = "G"
		value = value / GIGABYTE
	case bytes >= MEGABYTE:
		unit = "M"
		value = value / MEGABYTE
	case bytes >= KILOBYTE:
		unit = "K"
		value = value / KILOBYTE
	case bytes >= BYTE:
		unit = "B"
	case bytes == 0:
		return "0"
	}

	result := strconv.FormatFloat(value, 'f', 1, 64)
	result = strings.TrimSuffix(result, ".0")
	return result + unit
}

// ToMegabytes parses a string formatted by ByteSize as megabytes.
func ToMegabytes(s string) (int64, error) {
	bytes, err := ToBytes(s)
	if err != nil {
		return 0, err
	}

	return bytes / MEGABYTE, nil
}

// ToBytes parses a string formatted by ByteSize as bytes. Note binary-prefixed and SI prefixed units both mean a base-2 units
// KB = K = KiB	= 1024
// MB = M = MiB = 1024 * K
// GB = G = GiB = 1024 * M
// TB = T = TiB = 1024 * G
func ToBytes(s string) (int64, error) {
	s = strings.TrimSpace(s)
	s = strings.ToUpper(s)

	i := strings.IndexFunc(s, unicode.IsLetter)

	if i == -1 {
		return 0, invalidByteQuantityError
	}

	bytesString, multiple := s[:i], s[i:]
	bytes, err := strconv.ParseFloat(bytesString, 64)
	if err != nil || bytes <= 0 {
		return 0, invalidByteQuantityError
	}

	switch multiple {
	case "T", "TB", "TIB":
		return int64(bytes * TERABYTE), nil
	case "G", "GB", "GIB":
		return int64(bytes * GIGABYTE), nil
	case "M", "MB", "MIB", "MI":
		return int64(bytes * MEGABYTE), nil
	case "K", "KB", "KIB", "KI":
		return int64(bytes * KILOBYTE), nil
	case "B":
		return int64(bytes), nil
	default:
		return 0, invalidByteQuantityError
	}
}

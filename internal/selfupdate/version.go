package selfupdate

import (
	"strconv"
	"strings"
)

type semanticVersion struct {
	major      uint64
	minor      uint64
	patch      uint64
	prerelease []string
}

func validSemanticVersion(value string) bool {
	_, ok := parseSemanticVersion(value)
	return ok
}

func isNewerVersion(latest, current string) bool {
	latestVersion, latestOK := parseSemanticVersion(latest)
	if !latestOK {
		return false
	}
	currentVersion, currentOK := parseSemanticVersion(current)
	if !currentOK {
		return latest != current
	}
	return compareSemanticVersions(latestVersion, currentVersion) > 0
}

func parseSemanticVersion(value string) (semanticVersion, bool) {
	value = strings.TrimSpace(value)
	value = strings.TrimPrefix(value, "v")
	if value == "" {
		return semanticVersion{}, false
	}
	if build := strings.IndexByte(value, '+'); build >= 0 {
		if build == len(value)-1 || !validIdentifiers(value[build+1:], false) {
			return semanticVersion{}, false
		}
		value = value[:build]
	}
	var prerelease []string
	if dash := strings.IndexByte(value, '-'); dash >= 0 {
		if dash == len(value)-1 || !validIdentifiers(value[dash+1:], true) {
			return semanticVersion{}, false
		}
		prerelease = strings.Split(value[dash+1:], ".")
		value = value[:dash]
	}
	parts := strings.Split(value, ".")
	if len(parts) != 3 {
		return semanticVersion{}, false
	}
	numbers := make([]uint64, 3)
	for index, part := range parts {
		if !validNumericIdentifier(part) {
			return semanticVersion{}, false
		}
		number, err := strconv.ParseUint(part, 10, 64)
		if err != nil {
			return semanticVersion{}, false
		}
		numbers[index] = number
	}
	return semanticVersion{major: numbers[0], minor: numbers[1], patch: numbers[2], prerelease: prerelease}, true
}

func validIdentifiers(value string, rejectLeadingZeros bool) bool {
	for _, identifier := range strings.Split(value, ".") {
		if identifier == "" {
			return false
		}
		numeric := true
		for _, character := range identifier {
			if character < '0' || character > '9' {
				numeric = false
			}
			if !((character >= '0' && character <= '9') || (character >= 'A' && character <= 'Z') || (character >= 'a' && character <= 'z') || character == '-') {
				return false
			}
		}
		if rejectLeadingZeros && numeric && len(identifier) > 1 && identifier[0] == '0' {
			return false
		}
	}
	return true
}

func validNumericIdentifier(value string) bool {
	if value == "" || (len(value) > 1 && value[0] == '0') {
		return false
	}
	for _, character := range value {
		if character < '0' || character > '9' {
			return false
		}
	}
	return true
}

func compareSemanticVersions(left, right semanticVersion) int {
	for _, pair := range [][2]uint64{{left.major, right.major}, {left.minor, right.minor}, {left.patch, right.patch}} {
		if pair[0] < pair[1] {
			return -1
		}
		if pair[0] > pair[1] {
			return 1
		}
	}
	if len(left.prerelease) == 0 && len(right.prerelease) == 0 {
		return 0
	}
	if len(left.prerelease) == 0 {
		return 1
	}
	if len(right.prerelease) == 0 {
		return -1
	}
	limit := min(len(left.prerelease), len(right.prerelease))
	for index := 0; index < limit; index++ {
		comparison := comparePrereleaseIdentifier(left.prerelease[index], right.prerelease[index])
		if comparison != 0 {
			return comparison
		}
	}
	if len(left.prerelease) < len(right.prerelease) {
		return -1
	}
	if len(left.prerelease) > len(right.prerelease) {
		return 1
	}
	return 0
}

func comparePrereleaseIdentifier(left, right string) int {
	leftNumber, leftNumeric := numericPrerelease(left)
	rightNumber, rightNumeric := numericPrerelease(right)
	switch {
	case leftNumeric && rightNumeric:
		if leftNumber < rightNumber {
			return -1
		}
		if leftNumber > rightNumber {
			return 1
		}
		return 0
	case leftNumeric:
		return -1
	case rightNumeric:
		return 1
	case left < right:
		return -1
	case left > right:
		return 1
	default:
		return 0
	}
}

func numericPrerelease(value string) (uint64, bool) {
	if !validNumericIdentifier(value) {
		return 0, false
	}
	number, err := strconv.ParseUint(value, 10, 64)
	return number, err == nil
}

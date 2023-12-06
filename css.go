package siteperf

import (
	"os"
	"regexp"
	"slices"
)

// ExtractClassesFromFile reads the CSS file specified by the given path and
// extracts a sorted list of unique class names found within it. If reading the
// file fails, it returns an error. Otherwise, it returns a slice of class names
// without leading dots and ensures that each class name is valid according to
// CSS naming conventions.
func ExtractClassesFromFile(path string) ([]string, error) {
	bytes, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return ExtractClasses(string(bytes))
}

// ExtractClasses extracts class names from a provided CSS string. It returns a
// sorted, unique list of class names without the leading dot, ensuring that
// each class name is valid according to CSS naming conventions. If any error
// occurs during the extraction, an error is returned alongside an empty slice.
func ExtractClasses(css string) ([]string, error) {
	var classes []string
	re := regexp.MustCompile(`\.[a-zA-Z0-9_-]+`)

	matches := re.FindAllStringSubmatch(css, -1)

	for _, match := range matches {
		classes = append(classes, match[0][1:])
	}

	classes = filter(classes, func(s string) bool {
		return isValidClass(s)
	})

	classes = unique(classes)
	slices.Sort(classes)

	return classes, nil
}

var validClassRE = regexp.MustCompile(`^(?:[a-zA-Z_][a-zA-Z0-9_-]*$)`)

func isValidClass(name string) bool {
	return validClassRE.MatchString(name)
}

func unique[S ~[]E, E comparable](s S) S {
	if s == nil {
		return nil
	}
	seen := make(map[E]bool)
	unique := make(S, 0, len(s))
	for _, e := range s {
		if !seen[e] {
			unique = append(unique, e)
			seen[e] = true
		}
	}
	return unique
}

package siteperf

import (
	"os"
	"regexp"
	"slices"
)

func ExtractClassesFromFile(path string) ([]string, error) {
	bytes, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return ExtractClasses(string(bytes))
}

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

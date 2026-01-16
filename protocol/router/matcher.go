package router

import (
	"net/http"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"
)

const (
	maxRegexLength    = 1000
	maxHeaderRules    = 20
	maxPathPrefixes   = 50
	maxPathRegexes    = 20
	maxHostMatchers   = 50
	maxMethods        = 10
	maxHeaderPatterns = 50
)

// routeMatcher compiles and evaluates route matching rules
type routeMatcher struct {
	pathPrefixes  []string
	pathRegexes   []*regexp.Regexp
	hostPatterns  []string // Wildcard patterns like *.example.com
	headerRules   map[string][]string // Header name -> patterns
	methods       map[string]bool
}

// newRouteMatcher creates a compiled matcher from configuration
func newRouteMatcher(match option.RouteMatch) (*routeMatcher, error) {
	matcher := &routeMatcher{
		headerRules: make(map[string][]string),
		methods:     make(map[string]bool),
	}

	// Validate and compile path prefixes
	if len(match.PathPrefix) > maxPathPrefixes {
		return nil, E.New("too many path prefixes (max ", maxPathPrefixes, ")")
	}
	matcher.pathPrefixes = match.PathPrefix

	// Validate and compile path regexes
	if len(match.PathRegex) > maxPathRegexes {
		return nil, E.New("too many path regexes (max ", maxPathRegexes, ")")
	}
	for _, pattern := range match.PathRegex {
		if len(pattern) > maxRegexLength {
			return nil, E.New("regex pattern too long (max ", maxRegexLength, " chars)")
		}
		re, err := regexp.Compile(pattern)
		if err != nil {
			return nil, E.Cause(err, "invalid regex pattern: ", pattern)
		}
		matcher.pathRegexes = append(matcher.pathRegexes, re)
	}

	// Validate and store host patterns (support wildcards with filepath.Match)
	if len(match.Host) > maxHostMatchers {
		return nil, E.New("too many host matchers (max ", maxHostMatchers, ")")
	}
	matcher.hostPatterns = match.Host

	// Validate and store header rules
	if len(match.Header) > maxHeaderRules {
		return nil, E.New("too many header rules (max ", maxHeaderRules, ")")
	}
	totalHeaderPatterns := 0
	for headerName, patterns := range match.Header {
		for range patterns {
			totalHeaderPatterns++
			if totalHeaderPatterns > maxHeaderPatterns {
				return nil, E.New("too many header patterns (max ", maxHeaderPatterns, ")")
			}
		}
		// Normalize header name to lowercase for case-insensitive matching
		matcher.headerRules[strings.ToLower(headerName)] = patterns
	}

	// Validate and store methods
	if len(match.Method) > maxMethods {
		return nil, E.New("too many methods (max ", maxMethods, ")")
	}
	for _, method := range match.Method {
		matcher.methods[strings.ToUpper(method)] = true
	}

	return matcher, nil
}

// matches evaluates if an HTTP request matches this route
// Returns true if ALL specified criteria match (AND logic)
// Evaluation order: method → path prefix → host → path regex → headers
// Short-circuits on first mismatch for performance
func (m *routeMatcher) matches(req *http.Request) bool {
	// 1. Method check (fastest - simple map lookup)
	if len(m.methods) > 0 {
		if !m.methods[req.Method] {
			return false
		}
	}

	// 2. Path prefix check (fast - string comparison)
	if len(m.pathPrefixes) > 0 {
		matched := false
		for _, prefix := range m.pathPrefixes {
			if strings.HasPrefix(req.URL.Path, prefix) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}

	// 3. Host check (medium - wildcard matching)
	if len(m.hostPatterns) > 0 {
		matched := false
		host := req.Host
		// Remove port if present
		if colonIdx := strings.IndexByte(host, ':'); colonIdx != -1 {
			host = host[:colonIdx]
		}
		for _, pattern := range m.hostPatterns {
			if matchWildcard(pattern, host) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}

	// 4. Path regex check (slower - regex evaluation)
	if len(m.pathRegexes) > 0 {
		matched := false
		for _, re := range m.pathRegexes {
			if re.MatchString(req.URL.Path) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}

	// 5. Header check (medium - case-insensitive with wildcard)
	if len(m.headerRules) > 0 {
		for headerName, patterns := range m.headerRules {
			// Get header values (http.Header.Values handles case-insensitivity)
			headerValues := req.Header.Values(headerName)
			if len(headerValues) == 0 {
				return false
			}

			// At least one header value must match at least one pattern
			matched := false
			for _, value := range headerValues {
				for _, pattern := range patterns {
					if matchWildcard(pattern, value) {
						matched = true
						break
					}
				}
				if matched {
					break
				}
			}
			if !matched {
				return false
			}
		}
	}

	return true
}

// matchWildcard matches a string against a pattern with * wildcards
// Supports patterns like "*.example.com", "Bearer *", etc.
func matchWildcard(pattern, str string) bool {
	// If no wildcard, do exact match (case-insensitive for domains)
	if !strings.Contains(pattern, "*") {
		return strings.EqualFold(pattern, str)
	}

	// Simple wildcard matching using filepath.Match
	// This supports * (match any) and ? (match one char)
	matched, err := filepath.Match(pattern, str)
	if err != nil {
		// If pattern is invalid, try case-insensitive exact match
		return strings.EqualFold(pattern, str)
	}
	return matched
}

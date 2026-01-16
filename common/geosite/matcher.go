package geosite

import (
	"regexp"
	"strings"
	"sync"
)

// Matcher provides domain to geosite code lookup functionality
type Matcher struct {
	access   sync.RWMutex
	reader   *Reader
	matchers map[string]*domainMatcher
	codes    []string
}

// domainMatcher matches domains against a single geosite code's rules
type domainMatcher struct {
	code        string
	domainMap   map[string]bool
	suffixList  []string
	keywordList []string
	regexList   []*regexp.Regexp
}

// NewMatcher creates a new geosite matcher from a reader.
// It preloads and compiles all geosite codes for fast lookup.
func NewMatcher(reader *Reader, codes []string) (*Matcher, error) {
	m := &Matcher{
		reader:   reader,
		matchers: make(map[string]*domainMatcher),
		codes:    codes,
	}

	// Preload and compile all codes
	for _, code := range codes {
		items, err := reader.Read(code)
		if err != nil {
			return nil, err
		}

		matcher, err := newDomainMatcher(code, items)
		if err != nil {
			// Skip codes that fail to compile (e.g., invalid regex)
			continue
		}

		m.matchers[code] = matcher
	}

	return m, nil
}

// Lookup returns the first matching geosite code for the given domain.
// Returns empty string if no match is found.
// Matching order: exact domain -> suffix -> keyword -> regex
func (m *Matcher) Lookup(domain string) string {
	m.access.RLock()
	defer m.access.RUnlock()

	// Normalize domain: lowercase
	domain = strings.ToLower(domain)

	// Try each code's matcher in order
	// Priority: codes are tried in the order they appear in the database
	for _, code := range m.codes {
		matcher := m.matchers[code]
		if matcher != nil && matcher.Match(domain) {
			return code
		}
	}

	return ""
}

// LookupAll returns all matching geosite codes for the given domain.
// Useful when a domain may match multiple categories.
func (m *Matcher) LookupAll(domain string) []string {
	m.access.RLock()
	defer m.access.RUnlock()

	domain = strings.ToLower(domain)
	var matches []string

	for _, code := range m.codes {
		matcher := m.matchers[code]
		if matcher != nil && matcher.Match(domain) {
			matches = append(matches, code)
		}
	}

	return matches
}

// Codes returns all available geosite codes
func (m *Matcher) Codes() []string {
	return m.codes
}

// newDomainMatcher creates a matcher for a single geosite code
func newDomainMatcher(code string, items []Item) (*domainMatcher, error) {
	domainMap := make(map[string]bool)
	var suffixList []string
	var keywordList []string
	var regexList []*regexp.Regexp

	for _, item := range items {
		switch item.Type {
		case RuleTypeDomain:
			domainMap[strings.ToLower(item.Value)] = true
		case RuleTypeDomainSuffix:
			suffixList = append(suffixList, strings.ToLower(item.Value))
		case RuleTypeDomainKeyword:
			keywordList = append(keywordList, strings.ToLower(item.Value))
		case RuleTypeDomainRegex:
			regex, err := regexp.Compile(item.Value)
			if err != nil {
				// Skip invalid regex patterns
				continue
			}
			regexList = append(regexList, regex)
		}
	}

	return &domainMatcher{
		code:        code,
		domainMap:   domainMap,
		suffixList:  suffixList,
		keywordList: keywordList,
		regexList:   regexList,
	}, nil
}

// Match checks if the domain matches any rule in this matcher
func (m *domainMatcher) Match(domain string) bool {
	// Exact domain match (fastest)
	if m.domainMap[domain] {
		return true
	}

	// Suffix match (e.g., ".google.com" matches "api.google.com")
	for _, suffix := range m.suffixList {
		if domain == suffix || strings.HasSuffix(domain, "."+suffix) {
			return true
		}
	}

	// Keyword match
	for _, keyword := range m.keywordList {
		if strings.Contains(domain, keyword) {
			return true
		}
	}

	// Regex match (slowest)
	for _, regex := range m.regexList {
		if regex.MatchString(domain) {
			return true
		}
	}

	return false
}

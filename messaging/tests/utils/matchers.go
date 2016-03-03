package utils

import (
	"bytes"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"

	"github.com/anovikov1984/go-vcr/cassette"
)

var logMu sync.Mutex

func NewPubnubMatcher(skipFields []string) cassette.Matcher {
	return &PubnubMatcher{
		skipFields: skipFields,
	}
}

func NewPubnubSubscribeMatcher(skipFields []string) cassette.Matcher {
	return &PubnubMatcher{
		skipFields: skipFields,
	}
}

// Matcher for non-subscribe requests
type PubnubMatcher struct {
	cassette.Matcher

	isSubscribe bool
	skipFields  []string
}

func (m *PubnubMatcher) Match(interactions []*cassette.Interaction,
	r *http.Request) (*cassette.Interaction, error) {

interactionsLoop:
	for _, i := range interactions {
		if r.Method != i.Request.Method {
			continue
		}

		expectedURL, err := url.Parse(i.URL)
		if err != nil {
			continue
		}

		if expectedURL.Host != r.URL.Host {
			continue
		}

		if !m.matchPath(expectedURL.Path, r.URL.Path) {
			// fmt.Println("!!!!!!!!!!!!!!paths doesnt match", expectedURL.Path, r.URL.Path)
			continue
		} else {
			// fmt.Println("!!!!!!!!!!!!!!paths MATCH", expectedURL.Path, r.URL.Path)
		}

		eQuery := expectedURL.Query()
		aQuery := r.URL.Query()

		for fKey, _ := range eQuery {
			if hasKey(fKey, m.skipFields) {
				continue
			}

			if aQuery[fKey] == nil || eQuery.Get(fKey) != aQuery.Get(fKey) {
				continue interactionsLoop
			}
		}

		return i, nil
	}

	return nil, errorInteractionNotFound(interactions)
}

func (m *PubnubMatcher) matchPath(expected, actual string) bool {
	re := regexp.MustCompile("^(/subscribe/[^/]+/)([^/]+)(/.+)$")

	eAllMatches := re.FindAllStringSubmatch(expected, -4)
	aAllMatches := re.FindAllStringSubmatch(actual, -4)

	if len(eAllMatches) > 0 && len(aAllMatches) > 0 {
		eMatches := eAllMatches[0][1:]
		aMatches := aAllMatches[0][1:]

		if eMatches[0] != aMatches[0] {
			return false
		}

		eChannels := strings.Split(eMatches[1], ",")
		aChannels := strings.Split(aMatches[1], ",")

		if !AssertStringSliceElementsEqual(eChannels, aChannels) {
			fmt.Println("chanels are NOT equal", eChannels, aChannels)
			return false
		} else {
			fmt.Println("chanels ARE equal", eChannels, aChannels)
		}

		if eMatches[2] != aMatches[2] {
			return false
		}

		return true
	} else {
		return expected == actual
	}
}

func errorInteractionNotFound(
	interactions []*cassette.Interaction) error {

	var urlsBuffer bytes.Buffer

	for _, i := range interactions {
		urlsBuffer.WriteString(i.URL)
		urlsBuffer.WriteString("\n")
	}

	return errors.New(fmt.Sprintf(
		"Interaction not found in:\n%s",
		urlsBuffer.String()))
}

package navigation

import (
	"bytes"
	"errors"
	"net/http"
	"strings"

	"github.com/mfonda/simhash"
	stringsutil "github.com/projectdiscovery/utils/strings"
	"golang.org/x/net/html"
)

// State identifies a unique navigation webapp state that might be reached by many means
type State struct {
	Name      string
	Structure []Content
	Features  []*Feature
	Hash      uint64
}

func NewState() (*State, error) {
	return &State{}, nil
}

// FromResponse calculates a state only based on the web page content
func (s *State) FromResponse(resp Response) error {
	// first we collect the raw material
	headers := resp.Resp.Header.Clone()

	return s.hash(headers, resp.Body)
}

func ContentTypeIsTextHtml(headers http.Header, body []byte) bool {
	return ContentTypeIs(headers, body, TextHtml)
}

func ContentTypeIs(headers http.Header, body []byte, contentTypes ...string) bool {
	contentType := headers.Get("Content-Type")
	if contentType == "" {
		contentType = http.DetectContentType(body)
	}
	return stringsutil.HasPrefixAny(contentType, contentTypes...)
}

func (s *State) hash(headers http.Header, body []byte) error {
	if !ContentTypeIsTextHtml(headers, body) {
		// static files can have a deterministic hash based on content
		return s.hashSimple(headers, body)
	}

	// we need to perform feature engineering: identify, extract and process features from raw material
	// then create a unique hash of the web state

	// we handle the most common case of HTML and we attempt to identify the page structure by considering only the most significative html items
	var tokenizedContents []Content
	htmlTokenizer := html.NewTokenizer(bytes.NewReader(body))
	for {
		// if next token is an error means we either reached the end of the file or the HTML was malformed
		if tokenType := htmlTokenizer.Next(); tokenType == html.ErrorToken {
			break
		}
		token := htmlTokenizer.Token()
		tokenizedContent := Content{
			Data:       token.Data,
			Short:      token.String(),
			Attributes: htmlAttributesToCoreAttributes(token.Attr),
		}
		switch token.Type {
		case html.TextToken:
			tokenizedContent.Type = Dynamic
		case html.StartTagToken, html.EndTagToken:
			tokenizedContent.Type = Core
		default:
			continue
		}
		tokenizedContents = append(tokenizedContents, tokenizedContent)
	}

	// filter out dynamic content
	filteredContents := filterContent(tokenizedContents)

	// the extracted content will be used to build the vectorized set of weighted features
	// Note #1: using unitary weight (for now)
	// Note #2: the weight cohefficient should keep into account => boost ratio of significant content (eg. forms) + frequency (eg. tfidf)
	// Note #3: more weight recommendations at http://www2007.org/papers/paper215.pdf
	features, err := extractFeatures(filteredContents)
	if err != nil {
		return err
	}

	// Now the hash can be used to compute the bitwise hamming distance with any other hash:
	// ≈1: structures can be considered the same
	// ≈0: structures are different
	s.Hash = simhash.Fingerprint(simhashVectorize(features))

	// During the vectorization process, tendentially locality information is lost (page structure)
	// so we save it for later to compute ordered sequences similarity
	s.Structure = filteredContents
	s.Features = features

	return nil
}

func htmlAttributesToCoreAttributes(htmlAttributes []html.Attribute) (attributes []Attribute) {
	for _, htmlAttribute := range htmlAttributes {
		attributes = append(attributes, Attribute{
			Name:      htmlAttribute.Key,
			Value:     htmlAttribute.Val,
			Namespace: htmlAttribute.Namespace,
		})
	}
	return
}

func filterContent(contents []Content) []Content {
	var filteredContent []Content
	for _, content := range contents {
		// removing items consisting only of new lines
		if content.Type == Dynamic && strings.ContainsAny(content.Data, "\n") {
			continue
		}
		filteredContent = append(filteredContent, content)
	}
	return filteredContent
}

func extractFeatures(contents []Content) ([]*Feature, error) {
	var features []*Feature
	for _, contentItem := range contents {
		feature, err := NewFeature(contentItem.ID(), 1)
		if err != nil {
			return nil, err
		}
		features = append(features, feature)
	}
	return features, nil
}

func (s *State) hashSimple(headers http.Header, body []byte) error {
	s.Hash = simhash.Simhash(simhash.NewWordFeatureSet(body))
	return nil
}

type Feature struct {
	ID     string
	Weight int
}

func NewFeature(id string, weight int) (*Feature, error) {
	if id == "" {
		return nil, errors.New("id can't be empty")
	}
	if weight <= 0 {
		return nil, errors.New("weight can't be negative")
	}
	return &Feature{ID: id, Weight: weight}, nil
}

func simhashVectorize(features []*Feature) simhash.Vector {
	var simhashFeatures []simhash.Feature
	for _, feature := range features {
		simhashFeatures = append(simhashFeatures, simhash.NewFeatureWithWeight([]byte(feature.ID), feature.Weight))
	}
	return simhash.Vectorize(simhashFeatures)
}

package navigation

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"math"
	"math/rand"
	"net/http"

	"github.com/mfonda/simhash"
	stringsutil "github.com/projectdiscovery/utils/strings"
	"golang.org/x/net/html"
)

const maxFeatures = 10000

// State identifies a unique navigation webapp state that might be reached by many means
type State struct {
	Name      string
	Structure []Content
	Hash      uint64
	Digest    string
	Data      []byte
}

// FromResponse calculates a state only based on the web page content
func NewState(req Request, resp Response, name string) (*State, error) {
	s := &State{}
	s.Name = name

	// first we collect the raw material
	headers := resp.Resp.Header.Clone()
	if err := s.hash(headers, resp.Body); err != nil {
		return nil, err
	}
	return s, nil
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
		// and a random simhash so they are counted as a unique node
		s.Hash = s.randomHash(headers, body)
		s.Digest = s.digest(headers, body)
		return nil
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
			tokenizedContent.TagType = Text
			tokenizedContent.Type = Core
		case html.StartTagToken:
			tokenizedContent.TagType = StartTag
			tokenizedContent.Type = Core
		case html.EndTagToken:
			tokenizedContent.TagType = EndTag
			tokenizedContent.Type = Core
		case html.CommentToken:
			tokenizedContent.TagType = Comment
			tokenizedContent.Type = Core
		case html.SelfClosingTagToken:
			tokenizedContent.TagType = SelfClosingTag
			tokenizedContent.Type = Core
		case html.DoctypeToken:
			tokenizedContent.TagType = Doctype
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
	// Now the hash can be used to compute the bitwise hamming distance with any other hash:
	// ≈1: structures can be considered the same
	// ≈0: structures are different
	hash, err := fingerprintFeatures(filteredContents, 2)
	if err != nil {
		return err
	}

	s.Hash = hash
	s.Digest = s.digest(headers, body)

	// During the vectorization process, tendentially locality information is lost (page structure)
	// so we save it for later to compute ordered sequences similarity
	s.Structure = filteredContents
	s.Data = body

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
		// removing dynamic content
		if content.Type == Dynamic {
			continue
		}

		filteredContent = append(filteredContent, content)
	}
	return filteredContent
}

func fingerprintFeatures(contents []Content, shingle int) (uint64, error) {
	var (
		simhashVector    simhash.Vector
		numberOfFeatures uint
	)

content_loop:
	for _, contentItem := range contents {
		for _, id := range contentItem.IDs() {
			if numberOfFeatures >= maxFeatures {
				break content_loop
			}
			// shingled k-gram feature
			skgram := make([][]byte, shingle)
			skgram = append(skgram[1:], []byte(id))
			featureSum := simhash.NewFeature(bytes.Join(skgram, []byte(" "))).Sum()

			for idx := uint8(0); idx < 64; idx++ {
				bit := ((featureSum >> idx) & 1)
				if bit == 1 {
					simhashVector[idx]++
				} else {
					simhashVector[idx]--
				}
			}
			numberOfFeatures++
		}
	}

	return simhash.Fingerprint(simhashVector), nil
}

// generate a probalistic far hash so that the node is classified as unique
func (s *State) randomHash(headers http.Header, body []byte) uint64 {
	return rand.Uint64()
}

func (s *State) digest(headers http.Header, body []byte) string {
	digest := sha256.Sum256(body)
	return hex.EncodeToString(digest[:])
}

func StateHash(s State) string {
	return s.Name
	// return fmt.Sprintf("%v", s.Hash)
}

func Similarity(s1, s2 *State) float64 {
	hammingDistance := simhash.Compare(s1.Hash, s2.Hash)
	// normalize the distance in [0-100] range
	normalizedDistance := float64(hammingDistance) / float64(math.MaxUint8)
	return 100 - (normalizedDistance * 100)
}

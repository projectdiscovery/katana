package navigation

import (
	"fmt"
	"strings"
)

// ContentType represent the nature of page content
type ContentType uint8

// Types of content type
const (
	Core ContentType = iota
	Dynamic
)

type Attribute struct {
	Name      string
	Value     string
	Namespace string
}

type Content struct {
	Type       ContentType
	Data       string
	Short      string
	Attributes []Attribute
}

func (c *Content) ID() string {
	var attributesId strings.Builder
	for _, attribute := range c.Attributes {
		attributesId.WriteString(fmt.Sprintf("A:%s=%s", attribute.Name, attribute.Value))
	}
	// generate an unique id for the content (C:) + attributes (A:)
	return fmt.Sprintf("C:%s-%s", c.Data, attributesId.String())
}

const (
	TextHtml string = "text/html"
)

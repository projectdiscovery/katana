package navigation

import (
	"fmt"
)

// ContentType represent the nature of page content
type ContentType uint8

// Types of content type
const (
	Core ContentType = iota
	Dynamic
)

// TagType represents the tag type
type TagType uint8

const (
	StartTag TagType = iota
	EndTag
	SelfClosingTag
	Doctype
	Comment
	Text
)

func (t TagType) String() string {
	switch t {
	case StartTag:
		return "ST"
	case EndTag:
		return "ET"
	case Doctype:
		return "D"
	case Comment:
		return "C"
	case Text:
		return "T"
	default:
		return ""
	}
}

const (
	TextHtml string = "text/html"
)

type Attribute struct {
	Name      string
	Value     string
	Namespace string
}

type Content struct {
	TagType    TagType
	Type       ContentType
	Data       string
	Short      string
	Attributes []Attribute
}

func (c *Content) IDs() (ids []string) {
	for _, attribute := range c.Attributes {
		id := fmt.Sprintf("A:%s:%s", attribute.Name, attribute.Value)
		ids = append(ids, id)
	}
	ids = append(ids, fmt.Sprintf("%s:%s", c.TagType.String(), c.Data))
	return
}

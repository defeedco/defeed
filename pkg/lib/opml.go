package lib

import (
	"encoding/xml"
	"fmt"
)

// OPML represents the top-level structure of an OPML file
// See: https://opml.org
type OPML struct {
	Head OPMLHead `xml:"head"`
	Body OPMLBody `xml:"body"`
}

// OPMLHead represents the head section of an OPML file
type OPMLHead struct {
	Title string `xml:"title"`
}

// OPMLBody represents the body section of an OPML file
type OPMLBody struct {
	Outlines []OPMLOutline `xml:"outline"`
}

// OPMLOutline represents an outline element in an OPML file
type OPMLOutline struct {
	Text     string        `xml:"text,attr"`
	Title    string        `xml:"title,attr"`
	Type     string        `xml:"type,attr"`
	XMLUrl   string        `xml:"xmlUrl,attr"`
	Outlines []OPMLOutline `xml:"outline"`
}

// ParseOPML parses OPML data and returns sources
func ParseOPML(opmlData string) (*OPML, error) {
	var out OPML
	err := xml.Unmarshal([]byte(opmlData), &out)
	if err != nil {
		return nil, fmt.Errorf("parse OPML: %w", err)
	}

	return &out, nil
}

package lib

import (
	"fmt"
	"strings"

	"github.com/go-shiori/go-readability"
)

func HTMLToText(html string) (string, error) {
	article, err := readability.FromReader(strings.NewReader(html), nil)
	if err != nil {
		return "", fmt.Errorf("readability from reader: %w", err)
	}

	return article.TextContent, nil
}

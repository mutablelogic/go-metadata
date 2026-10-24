package email

import (
	"context"
	"strings"

	// Packages
	htmltomarkdown "github.com/JohannesKaufmann/html-to-markdown/v2"
	converter "github.com/JohannesKaufmann/html-to-markdown/v2/converter"
)

////////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// htmlMarkdown returns the HTML of a message converted to Markdown, or
// nothing when it has no content, such as a body with only a space
func htmlMarkdown(ctx context.Context, html string) (string, error) {
	if strings.TrimSpace(html) == "" {
		return "", nil
	}
	markdown, err := htmltomarkdown.ConvertString(html, converter.WithContext(ctx))
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(markdown), nil
}

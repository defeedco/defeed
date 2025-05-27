package widgets

import (
	"bytes"
	"fmt"
	"github.com/glanceapp/glance/web"
	"html/template"
	"regexp"
)

var (
	themeStyleTemplate = web.MustParseTemplate("theme-style.gotmpl")
)

func DefaultThemePresets() []Theme {
	return []Theme{
		{
			Key:                      "default",
			Light:                    true,
			BackgroundColor:          &HSLColor{H: 240, S: 13, L: 95},
			PrimaryColor:             &HSLColor{H: 230, S: 100, L: 30},
			NegativeColor:            &HSLColor{S: 70, L: 50},
			ContrastMultiplier:       1.3,
			TextSaturationMultiplier: 0.5,
		},
	}
}

type Theme struct {
	BackgroundColor          *HSLColor
	PrimaryColor             *HSLColor
	PositiveColor            *HSLColor
	NegativeColor            *HSLColor
	Light                    bool
	ContrastMultiplier       float32
	TextSaturationMultiplier float32
	Key                      string
	CSS                      template.CSS
	BackgroundColorAsHex     string
}

var whitespaceAtBeginningOfLinePattern = regexp.MustCompile(`(?m)^\s+`)

func (t *Theme) Init() error {
	css, err := executeTemplateToString(themeStyleTemplate, t)
	if err != nil {
		return fmt.Errorf("compiling theme style: %v", err)
	}
	t.CSS = template.CSS(whitespaceAtBeginningOfLinePattern.ReplaceAllString(css, ""))

	if t.BackgroundColor != nil {
		t.BackgroundColorAsHex = t.BackgroundColor.ToHex()
	} else {
		t.BackgroundColorAsHex = "#151519"
	}

	return nil
}

func (t *Theme) SameAs(t1 *Theme) bool {
	if t == nil && t1 == nil {
		return true
	}
	if t == nil || t1 == nil {
		return false
	}
	if t.Light != t1.Light {
		return false
	}
	if t.ContrastMultiplier != t1.ContrastMultiplier {
		return false
	}
	if t.TextSaturationMultiplier != t1.TextSaturationMultiplier {
		return false
	}
	if t.BackgroundColor != t1.BackgroundColor {
		return false
	}
	if t.PrimaryColor != t1.PrimaryColor {
		return false
	}
	if t.PositiveColor != t1.PositiveColor {
		return false
	}
	if t.NegativeColor != t1.NegativeColor {
		return false
	}
	return true
}

func executeTemplateToString(t *template.Template, data any) (string, error) {
	var b bytes.Buffer
	err := t.Execute(&b, data)
	if err != nil {
		return "", fmt.Errorf("executing template: %w", err)
	}

	return b.String(), nil
}

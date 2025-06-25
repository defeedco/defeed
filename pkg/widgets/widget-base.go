package widgets

import (
	"bytes"
	"context"
	"html/template"

	"github.com/glanceapp/glance/pkg/sources"
	"github.com/glanceapp/glance/pkg/sources/activities/types"
	"github.com/glanceapp/glance/web"
)

type widgetBase struct {
	id            uint64
	Typ           string
	HideHeader    bool   `json:"hide_header"`
	CSSClass      string `json:"css_class"`
	CollapseAfter int    `json:"collapse_after"`
	// SourceID is the filter parameter for fetching activities.
	SourceID string `json:"source_id"`
	// Query is the search query for filtering with natural language.
	Query string `json:"query"`
	// MinSimilarity is the minimum similarity (0-1) for filtering with natural language.
	MinSimilarity float32 `json:"min_similarity"`
	// Limit for the number of activities to show.
	Limit          int `json:"limit" default:"10"`
	Error          error
	Notice         error
	templateBuffer bytes.Buffer
}

func newWidgetBase(id uint64, typ string) *widgetBase {
	return &widgetBase{
		id:             id,
		Typ:            typ,
		HideHeader:     false,
		CSSClass:       "",
		Error:          nil,
		CollapseAfter:  3,
		Notice:         nil,
		templateBuffer: bytes.Buffer{},
	}
}

func (w *widgetBase) Type() string {
	return w.Typ
}

func (w *widgetBase) ID() uint64 {
	return w.id
}

func (w *widgetBase) setHideHeader(value bool) {
	w.HideHeader = value
}

var widgetBaseContentTemplate = web.MustParseTemplate("widget-base-content.html", "widget-base.html")

type renderData struct {
	*widgetBase
	Activities []*types.DecoratedActivity
}

func (w *widgetBase) Render(registry *sources.Registry) template.HTML {
	sortBy := types.SortByDate
	if w.Query != "" {
		sortBy = types.SortBySimilarity
	}
	activities, err := registry.Search(
		context.Background(),
		w.Query,
		[]string{w.SourceID},
		w.MinSimilarity,
		w.Limit,
		sortBy,
	)
	w.Error = err
	return w.renderTemplate(renderData{w, activities}, widgetBaseContentTemplate)
}

func (w *widgetBase) Initialize() error {
	if w.CollapseAfter <= 0 {
		w.CollapseAfter = 3
	}
	return nil
}

func (w *widgetBase) renderTemplate(data any, t *template.Template) template.HTML {
	w.templateBuffer.Reset()
	err := t.Execute(&w.templateBuffer, data)
	if err != nil {
		w.Error = err

		// need to immediately re-render with the error,
		// otherwise risk breaking the page since the widget
		// will likely be partially rendered with tags not closed.
		w.templateBuffer.Reset()
		err2 := t.Execute(&w.templateBuffer, data)

		if err2 != nil {
			w.templateBuffer.Reset()
			// TODO: add some kind of a generic widget error template when the widget
			// failed to render, and we also failed to re-render the widget with the error
		}
	}

	return template.HTML(w.templateBuffer.String())
}

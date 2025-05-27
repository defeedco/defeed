package widgets

import (
	"github.com/glanceapp/glance/pkg/sources"
	"html/template"
	"sync/atomic"
)

var widgetIDCounter atomic.Uint64

func NewWidget(widgetType string) (Widget, error) {
	id := widgetIDCounter.Add(1)

	var w Widget

	switch widgetType {
	case "group":
		w = newWidgetGroup(id, widgetType)
	case "split-column":
		w = newWidgetSplitColumn(id, widgetType)
	default:
		w = newWidgetBase(id, widgetType)
	}

	return w, nil
}

type Widget interface {
	// Render is called within templates.
	Render(registry *sources.Registry) template.HTML
	// Type is called within templates.
	Type() string
	// ID is called within templates.
	ID() uint64

	// Initialize is called after the widget is created.
	Initialize() error

	setHideHeader(bool)
}

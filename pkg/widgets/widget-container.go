package widgets

import "fmt"

type containerWidgetBase struct {
	Widgets []Widget `yaml:"widgets"`
}

func (widget *containerWidgetBase) _initializeWidgets() error {
	for i := range widget.Widgets {
		if err := widget.Widgets[i].Initialize(); err != nil {
			return formatWidgetInitError(err, widget.Widgets[i])
		}
	}

	return nil
}

func formatWidgetInitError(err error, w Widget) error {
	return fmt.Errorf("%s widget: %v", w.Type(), err)
}

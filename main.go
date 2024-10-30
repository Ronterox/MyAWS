package main

import (
	"log"

	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

func main() {
	a := app.New()
	w := a.NewWindow("Toolbar Widget")

	input := widget.NewMultiLineEntry()
	input.SetPlaceHolder("Enter text here...")

	getSelected := func(action string) string {
		log.Println(action)

		selected := input.SelectedText()
		if selected == "" {
			selected = input.Text
		}
		return selected
	}

	toolbar := widget.NewToolbar(
		widget.NewToolbarAction(theme.DocumentCreateIcon(), func() {
			log.Println("New document")

            writer, err := storage.Writer(storage.NewFileURI("test.txt"))
            if err != nil {
                log.Println(err)
                return
            }

            writer.Write([]byte(input.Text))
		}),
		widget.NewToolbarSeparator(),
		widget.NewToolbarAction(theme.ContentCutIcon(), func() {
			w.Clipboard().SetContent(getSelected("Cut"))
			input.SetText("")
		}),
		widget.NewToolbarAction(theme.ContentCopyIcon(), func() {
			w.Clipboard().SetContent(getSelected("Copy"))
		}),
		widget.NewToolbarAction(theme.ContentPasteIcon(), func() {
			input.SetText(w.Clipboard().Content())
		}),
		widget.NewToolbarAction(theme.SearchReplaceIcon(), func() {}),
		widget.NewToolbarSpacer(),
		widget.NewToolbarAction(theme.WindowCloseIcon(), func() {
			a.Quit()
		}),
		widget.NewToolbarAction(theme.HelpIcon(), func() {
			log.Println("Display help")
		}),
	)

	w.SetContent(container.NewBorder(toolbar, nil, nil, nil, input))
	w.ShowAndRun()
}


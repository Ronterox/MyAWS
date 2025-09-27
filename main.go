package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/widget"
)

type User struct {
	ID          int    `json:"id"`
	Username    string `json:"nickname"`
	ProfileLink string `json:"github_profile"`
}

func main() {
	var users []User

	a := app.New()
	w := a.NewWindow("My AWS")

	text := widget.NewLabel("Fetch data test")
	data := binding.BindStringList(&[]string{})
	list := widget.NewListWithData(data,
		func() fyne.CanvasObject {
			return widget.NewLabel("template")
		},
		func(i binding.DataItem, o fyne.CanvasObject) {
			o.(*widget.Label).Bind(i.(binding.String))
		},
	)
	list.OnSelected = func(i widget.ListItemID) {
		user := users[i]
		path := strings.Replace(user.ProfileLink, "https://github.com/", "", 1)
		url := url.URL{Scheme: "https", Host: "github.com", Path: path}
		fyne.CurrentApp().OpenURL(&url)
	}

	content := container.NewBorder(
		container.NewHBox(
			text,
			widget.NewButton("Fetch", func() {
				text.SetText("Fetching data...")

				go func() {
					res, err := http.Get("https://24pullrequests.com/users.json?page=2")
					if err != nil {
						fmt.Printf("Error fetching data: %v", err)
						return
					}
					defer res.Body.Close()

					bytes, _ := io.ReadAll(res.Body)

					if err := json.Unmarshal(bytes, &users); err != nil {
						fmt.Printf("Error unmarshalling data: %v", err)
						return
					}

					for _, user := range users {
						data.Append(user.Username)
					}

					fyne.Do(func() {
						text.SetText("Click to open the user profile on your browser")
					})
				}()
			}),
			widget.NewButton("Close", func() { a.Quit() }),
		),
		nil, nil, nil,
		list,
	)

	w.SetContent(content)
	w.ShowAndRun()
}

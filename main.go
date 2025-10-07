package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

type Job struct {
	Name  string `json:"name"`
	URL   string `json:"url"`
	Color string `json:"color"`
}

type State struct {
	Jobs []Job `json:"jobs"`
}

type LastBuild struct {
	Building bool   `json:"building"`
	Result   string `json:"result,omitempty"`
	URL      string `json:"url"`
}

type Executable struct {
	Number int    `json:"number"`
	URL    string `json:"url"`
}

type QueueItem struct {
	Executable *Executable `json:"executable"`
}

func jenkinsRequest(method string, path string) (*http.Response, error) {
	prefs := fyne.CurrentApp().Preferences()

	auth := prefs.String("username") + ":" + prefs.String("password")
	basicAuth := "Basic " + base64.StdEncoding.EncodeToString([]byte(auth))

	baseURL := prefs.String("url")
	req, _ := http.NewRequest(method, baseURL+strings.Replace(path, baseURL, "", 1), nil)
	req.Header.Add("Authorization", basicAuth)

	client := &http.Client{}
	return client.Do(req)
}

func parseBody[T any](req *http.Response, v *T) error {
	bytes, _ := io.ReadAll(req.Body)
	return json.Unmarshal(bytes, &v)
}

func main() {
	var jobs []Job

	a := app.NewWithID("com.github.rontero.myaws")
	w := a.NewWindow("My AWS")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	text := widget.NewLabel("My AWS")
	hyperlink := widget.NewHyperlink("", nil)
	hyperlink.Hide()
	flex := container.NewHBox(text, hyperlink)

	data := binding.BindStringList(&[]string{})
	list := widget.NewListWithData(data,
		func() fyne.CanvasObject {
			return widget.NewLabel("template")
		},
		func(i binding.DataItem, o fyne.CanvasObject) {
			o.(*widget.Label).Bind(i.(binding.String))
		},
	)

	updateText := func(message string) {
		fmt.Println(message)
		fyne.Do(func() {
			text.SetText(message)
		})
	}

	fetchButton := widget.NewButton("Fetch Jobs", func() {
		updateText("Fetching data...")

		go func() {
			res, err := jenkinsRequest("GET", "/api/json")
			if err != nil {
				updateText("Error fetching data: " + err.Error())
				return
			}
			defer res.Body.Close()

			state := State{}
			if err := parseBody(res, &state); err != nil {
				updateText("Error parsing state response: " + err.Error())
				return
			}

			jobs = state.Jobs
			for _, user := range jobs {
				data.Append(user.Name)
			}

			updateText("Tap a job to select it")
		}()
	})

	list.OnSelected = func(i widget.ListItemID) {
		text.SetText("Job: " + jobs[i].Name)
		fetchButton.SetText("Launch Job")
		fetchButton.OnTapped = func() {
			fmt.Println("Launching job: " + jobs[i].Name)
			fetchButton.Disable()
			hyperlink.Hide()

			fyne.CurrentApp().SendNotification(&fyne.Notification{
				Title:   "Job launched: " + jobs[i].Name,
				Content: "Waiting for build to finish...",
			})

			go func() {
				updateText("Launching job: " + jobs[i].Name + "...")

				res, err := jenkinsRequest("POST", jobs[i].URL+"build")
				if err != nil {
					updateText("Error launching job: " + err.Error())
					return
				}

				queueURL := res.Header.Get("Location")
				ticker := time.NewTicker(time.Second * 2)

				defer ticker.Stop()
				defer fyne.Do(func() {
					fetchButton.Enable()
				})

				for {
					select {
					case <-ctx.Done():
						fmt.Println("Cancelled")
						return
					case <-ticker.C:
						fmt.Println("Checking job status...")

						res, err := jenkinsRequest("GET", queueURL+"api/json")
						if err != nil {
							updateText("Error fetching queue status: " + err.Error())
							return
						}

						queueItem := QueueItem{}
						if err := parseBody(res, &queueItem); err != nil {
							updateText("Error parsing queue response: " + err.Error())
							return
						}

						if queueItem.Executable == nil {
							updateText("Job in queue...")
							continue
						}

						res, err = jenkinsRequest("GET", jobs[i].URL+"lastBuild/api/json")
						if err != nil {
							updateText("Error fetching job status: " + err.Error())
							return
						}
						defer res.Body.Close()

						lastBuild := LastBuild{}
						if err := parseBody(res, &lastBuild); err != nil {
							updateText("Error parsing build response: " + err.Error())
							return
						}

						if lastBuild.Building {
							updateText("Building...")
							continue
						} else {
							fyne.Do(func() {
								text.SetText("Build finished with " + lastBuild.Result)
								hyperlink.SetText("See output")
								hyperlink.SetURLFromString(lastBuild.URL + "console")
								hyperlink.Show()
							})
							fyne.CurrentApp().SendNotification(&fyne.Notification{
								Title:   "Build finished with " + lastBuild.Result,
								Content: lastBuild.URL + "console",
							})
							return
						}
					}
				}
			}()
		}
	}

	content := container.NewBorder(
		container.NewHBox(
			flex,
			fetchButton,
			widget.NewButton("Set up", func() {
				urlEntry := widget.NewEntry()
				urlEntry.SetText(a.Preferences().String("url"))

				nameEntry := widget.NewEntry()
				nameEntry.SetText(a.Preferences().String("username"))

				passwordEntry := widget.NewPasswordEntry()
				passwordEntry.SetText(a.Preferences().String("password"))

				dialog.ShowForm("Setup Jenkins API", "Save", "Discard",
					[]*widget.FormItem{
						widget.NewFormItem("Jenkins URL", urlEntry),
						widget.NewFormItem("Username", nameEntry),
						widget.NewFormItem("User Token", passwordEntry),
					},
					func(accept bool) {
						if accept {
							app := fyne.CurrentApp()
							pref := app.Preferences()

							pref.SetString("url", urlEntry.Text)
							pref.SetString("username", nameEntry.Text)
							pref.SetString("password", passwordEntry.Text)

							app.SendNotification(&fyne.Notification{
								Title:   "Settings saved!",
								Content: "Jenkins URL: " + urlEntry.Text,
							})
						}
					}, w,
				)
			}),
			widget.NewButton("Close", func() { a.Quit() }),
		),
		nil, nil, nil,
		list,
	)

	w.SetContent(content)
	w.ShowAndRun()
}

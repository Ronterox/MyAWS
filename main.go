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

const (
	userName = "rontero"
	token    = ""
	baseURL  = "http://localhost:8080"
)

func jenkinsRequest(method string, path string) (*http.Response, error) {
	auth := userName + ":" + token
	basicAuth := "Basic " + base64.StdEncoding.EncodeToString([]byte(auth))

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

	a := app.New()
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

	fetchButton := widget.NewButton("Fetch Jobs", func() {
		text.SetText("Fetching data...")
		go func() {
			res, err := jenkinsRequest("GET", "/api/json")
			if err != nil {
				fmt.Printf("Error fetching data: %v", err)
				return
			}
			defer res.Body.Close()

			state := State{}
			if err := parseBody(res, &state); err != nil {
				fmt.Printf("Error parsing state response: %v", err)
				return
			}

			jobs = state.Jobs
			for _, user := range jobs {
				data.Append(user.Name)
			}

			fyne.Do(func() {
				text.SetText("Tap a job to select it")
			})
		}()
	})

	list.OnSelected = func(i widget.ListItemID) {
		text.SetText("Job: " + jobs[i].Name)
		fetchButton.SetText("Launch Job")
		fetchButton.OnTapped = func() {
			fmt.Println("Launching job: " + jobs[i].Name)
			fetchButton.Disable()
			hyperlink.Hide()

			go func() {
				fyne.Do(func() {
					text.SetText("Launching job: " + jobs[i].Name + "...")
				})

				res, err := jenkinsRequest("POST", jobs[i].URL+"build")
				if err != nil {
					fmt.Printf("Error launching job: %v", err)
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
							fmt.Printf("Error fetching queue status: %v", err)
							return
						}

						queueItem := QueueItem{}
						if err := parseBody(res, &queueItem); err != nil {
							fmt.Printf("URL: %s\n", res.Request.URL)
							fmt.Printf("Error parsing queue response: %v", err)
							return
						}

						if queueItem.Executable == nil {
							fyne.Do(func() {
								text.SetText("Job in queue...")
							})
							continue
						}

						res, err = jenkinsRequest("GET", jobs[i].URL+"lastBuild/api/json")
						if err != nil {
							fmt.Printf("Error fetching job status: %v", err)
							return
						}
						defer res.Body.Close()

						lastBuild := LastBuild{}
						if err := parseBody(res, &lastBuild); err != nil {
							fmt.Printf("Error parsing build response: %v", err)
							return
						}

						if lastBuild.Building {
							fyne.Do(func() {
								text.SetText("Building...")
							})
							continue
						} else {
							fyne.Do(func() {
								text.SetText("Build finished with " + lastBuild.Result)
								hyperlink.SetText("See output")
								hyperlink.SetURLFromString(lastBuild.URL + "console")
								hyperlink.Show()
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
			widget.NewButton("Close", func() { a.Quit() }),
		),
		nil, nil, nil,
		list,
	)

	w.SetContent(content)
	w.ShowAndRun()
}

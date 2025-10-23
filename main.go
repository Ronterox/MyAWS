package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/driver/mobile"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

type DefaultParameter struct {
	Class string `json:"_class"`
	Value string `json:"value"`
}

type Parameter struct {
	Name    string           `json:"name"`
	Desc    string           `json:"description"`
	Default DefaultParameter `json:"defaultParameterValue"`
}

type Property struct {
	Class      string      `json:"_class"`
	Parameters []Parameter `json:"parameterDefinitions"`
}

type Job struct {
	Name       string     `json:"name"`
	URL        string     `json:"url"`
	Color      string     `json:"color"`
	Properties []Property `json:"property"`
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

type TouchableLabel struct {
	widget.Label
	holdDuration time.Duration
	lastPress    time.Time
	OnHold       func()
	OnTapped     func()
}

func NewTouchableLabel(label string, tapped func(), hold func()) *TouchableLabel {
	btn := &TouchableLabel{
		holdDuration: time.Millisecond * 500,
		OnHold:       hold,
		OnTapped:     tapped,
	}
	btn.ExtendBaseWidget(btn)
	btn.SetText(label)
	return btn
}

func (b *TouchableLabel) OnDown() {
	b.lastPress = time.Now()
	go func() {
		time.Sleep(b.holdDuration)
		if time.Since(b.lastPress) > b.holdDuration {
			fyne.Do(func() {
				b.OnHold()
			})
		}
	}()
}

func (b *TouchableLabel) OnUp() {
	b.OnTapped()
	b.lastPress = time.Now()
}

func (b *TouchableLabel) MouseDown(e *desktop.MouseEvent) {
	b.OnDown()
}

func (b *TouchableLabel) MouseUp(e *desktop.MouseEvent) {
	b.OnUp()
}

func (b *TouchableLabel) TouchDown(e *mobile.TouchEvent) {
	b.OnDown()
}

func (b *TouchableLabel) TouchUp(e *mobile.TouchEvent) {
	b.OnUp()
}

func (b *TouchableLabel) TouchCancel(e *mobile.TouchEvent) {
}

func jenkinsRequest(method string, path string, data *url.Values) (*http.Response, error) {
	prefs := fyne.CurrentApp().Preferences()

	auth := prefs.String("username") + ":" + prefs.String("password")
	basicAuth := "Basic " + base64.StdEncoding.EncodeToString([]byte(auth))

	var reqData io.Reader
	if data != nil {
		reqData = strings.NewReader(data.Encode())
	}

	baseURL := prefs.String("url")
	req, _ := http.NewRequest(method, baseURL+strings.Replace(path, baseURL, "", 1), reqData)
	req.Header.Add("Authorization", basicAuth)
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{}
	return client.Do(req)
}

func parseBody[T any](req *http.Response, v *T) error {
	bytes, _ := io.ReadAll(req.Body)
	return json.Unmarshal(bytes, &v)
}

func TextToPositiveInt(s string) int {
	h := fnv.New32a()
	h.Write([]byte(s))
	return int(h.Sum32() & 0x7fffffff) // mask sign bit
}

func main() {
	var jobs []Job
	var list *widget.List

	a := app.NewWithID("com.github.rontero.myaws")
	w := a.NewWindow("My AWS")
	a.SetIcon(resourceIconPng)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	text := widget.NewLabel("My AWS")
	hyperlink := widget.NewHyperlink("", nil)
	hyperlink.Hide()

	flex := container.NewHBox(text, hyperlink)

	data := binding.BindStringList(&[]string{})
	list = widget.NewListWithData(data,
		func() fyne.CanvasObject {
			return container.NewPadded(
				NewTouchableLabel("template", func() {
					fmt.Println("Tapped!")
				}, func() {
					fmt.Println("Hold")
				}),
				widget.NewIcon(theme.HomeIcon()),
			)
		},
		func(i binding.DataItem, o fyne.CanvasObject) {
			label := o.(*fyne.Container).Objects[0].(*TouchableLabel)
			name, _ := i.(binding.String).Get()
			label.Bind(i.(binding.String))

			label.OnTapped = func() {
				items, _ := data.Get()
				for i, item := range items {
					if item == label.Text {
						list.Select(i)
						break
					}
				}
			}
			label.OnHold = func() {
				var popup *widget.PopUp
				var url *url.URL

				label.OnTapped()

				for _, job := range jobs {
					if job.Name == name {
						url, _ = url.Parse(job.URL)
						break
					}
				}

				popup = widget.NewModalPopUp(
					container.NewVBox(
						widget.NewHyperlink("See '"+name+"' on Jenkins", url),
						widget.NewButton("Close", func() {
							popup.Hide()
						}),
					),
					w.Canvas(),
				)

				popup.Resize(fyne.NewSize(200, 100))
				popup.Show()
			}

			icons := []fyne.Resource{
				theme.HomeIcon(),
				theme.ComputerIcon(),
				theme.AccountIcon(),
				theme.DesktopIcon(),
				theme.FileTextIcon(),
				theme.FileApplicationIcon(),
				theme.HistoryIcon(),
				theme.GridIcon(),
				theme.DocumentPrintIcon(),
				theme.DocumentSaveIcon(),
			}

			icon := o.(*fyne.Container).Objects[1].(*widget.Icon)
			icon.SetResource(icons[TextToPositiveInt(name)%len(icons)])
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
			res, err := jenkinsRequest("GET", "/api/json", nil)
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

	launchJob := func(job Job, request func() (*http.Response, error)) {
		updateText("Launching job: " + job.Name + "...")
		res, err := request()

		fyne.CurrentApp().SendNotification(&fyne.Notification{
			Title:   "Job launched: " + job.Name,
			Content: "Waiting for job to finish...",
		})

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

				res, err := jenkinsRequest("GET", queueURL+"api/json", nil)
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

				res, err = jenkinsRequest("GET", job.URL+"lastBuild/api/json", nil)
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
						hyperlink.SetURLFromString(lastBuild.URL + "consoleText")
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
	}

	list.OnSelected = func(i widget.ListItemID) {
		text.SetText("Job: " + jobs[i].Name)
		fetchButton.SetText("Launch Job")

		go func() {
			res, err := jenkinsRequest("GET", "/job/"+jobs[i].Name+"/api/json", nil)
			if err != nil {
				updateText("Error fetching job properties: " + err.Error())
				return
			}

			job := Job{}
			if err := parseBody(res, &job); err != nil {
				updateText("Error parsing job properties: " + err.Error())
				return
			}

			var parameters []Parameter
			if len(job.Properties) > 1 {
				for _, property := range job.Properties {
					if property.Class == "hudson.model.ParametersDefinitionProperty" {
						parameters = property.Parameters
						break
					}
				}
			}

			if len(parameters) > 0 {
				fyne.Do(func() { fetchButton.SetText("Launch Job With Parameters") })
				fetchButton.OnTapped = func() {
					fetchButton.Disable()
					hyperlink.Hide()

					widgets := make([]*widget.FormItem, len(parameters))
					entries := make([]*widget.Entry, len(parameters))
					for i, parameter := range parameters {
						entry := widget.NewEntry()
						entry.SetText(parameter.Default.Value)

						item := widget.NewFormItem(parameter.Name, entry)
						item.HintText = parameter.Desc

						widgets[i] = item
						entries[i] = entry
					}

					dialog.ShowForm("Job properties", "Launch", "Cancel",
						widgets,
						func(accept bool) {
							if accept {
								go launchJob(jobs[i], func() (*http.Response, error) {
									data := url.Values{}
									for i, entry := range entries {
										data.Add(parameters[i].Name, entry.Text)
									}
									return jenkinsRequest("POST", jobs[i].URL+"buildWithParameters", &data)
								})
							} else {
								fetchButton.Enable()
							}
						}, w,
					)
				}
			} else {
				fetchButton.OnTapped = func() {
					fetchButton.Disable()
					hyperlink.Hide()
					go launchJob(jobs[i], func() (*http.Response, error) {
						return jenkinsRequest("POST", jobs[i].URL+"build", nil)
					})
				}
			}

		}()
	}

	setUpButton := widget.NewButton("Set up", func() {
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
	})

	actionbar := container.NewScroll(container.NewHBox(
		flex,
		fetchButton,
		setUpButton,
	))
	actionbar.Direction = container.ScrollHorizontalOnly

	openJenkins := func(path string) {
		baseURL := a.Preferences().String("url")
		url, err := url.Parse(baseURL + path)
		if err != nil {
			updateText("Error parsing URL: " + err.Error())
			return
		}
		a.OpenURL(url)
	}

	content := container.NewBorder(
		actionbar,
		widget.NewToolbar(
			widget.NewToolbarAction(theme.FolderNewIcon(), func() {
				openJenkins("/view/all/newJob")
			}),
			widget.NewToolbarSeparator(),
			widget.NewToolbarAction(theme.ComputerIcon(), func() {
				openJenkins("/computer")
			}),
			widget.NewToolbarAction(theme.SettingsIcon(), func() {
				openJenkins("/manage")
			}),
			widget.NewToolbarSpacer(),
			widget.NewToolbarAction(theme.HelpIcon(), func() {
				var popup *widget.PopUp
				userURL := a.Preferences().String("url")
				user := a.Preferences().String("username")

				fullURL := userURL + "/user/" + user + "/security/"
				link := widget.NewHyperlink(fullURL, nil)
				if err := link.SetURLFromString(fullURL); err != nil {
					link.Hide()
				}

				syntaxURL, _ := url.Parse("https://www.jenkins.io/doc/book/pipeline/syntax/")

				popup = widget.NewModalPopUp(
					container.NewVBox(
						widget.NewLabel(`
Welcome to my custom AWS app!

If you are using a VPN, make sure to connect to it first.

First you will have to set up your Jenkins URL in the settings.
Also you will need to create an user token in Jenkins.

jenkins.url/user/username/security/

Note: If you add the url and username in the settings, you can come here and click your generated url.
						`),
						link,
						widget.NewHyperlink("See scripts examples", syntaxURL),
						widget.NewButton("Close", func() {
							popup.Hide()
						}),
					),
					w.Canvas(),
				)
				popup.Show()
			}),
		), nil, nil,
		list,
	)

	go func() {
		fetchButton.Tapped(nil)
	}()

	fmt.Println("Starting app...")
	w.SetContent(content)
	w.ShowAndRun()
}

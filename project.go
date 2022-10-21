package main

import (
	"io"
	"os"
	"os/exec"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/widget"
)

func (d *defyne) showNewProjectDialog(w fyne.Window) {
	parent := widget.NewButton("Choose directory", nil)
	dir := defaultDir()
	if dir != nil {
		parent.SetText(dir.Name())
	}
	parent.OnTapped = func() {
		dialog.ShowFolderOpen(func(u fyne.ListableURI, err error) {
			if err != nil {
				dialog.ShowError(err, d.win)
				return
			}
			if u == nil {
				return
			}

			dir = u
			parent.SetText(u.Name())
		}, w)
	}

	name := widget.NewEntry()
	dialog.ShowForm("Create project", "Create", "Cancel", []*widget.FormItem{
		widget.NewFormItem("Parent directory", parent),
		widget.NewFormItem("Project name", name),
	}, func(ok bool) {
		if !ok {
			return
		}

		dir, err := createProject(dir, name.Text)
		if err != nil {
			dialog.ShowError(err, w)
		} else {
			d.setProject(dir)
			d.win.Show()
			w.Close()

			addRecent(dir, fyne.CurrentApp().Preferences())
		}
	}, w)
}

func (d *defyne) showOpenProjectDialog(w fyne.Window) {
	dialog.ShowFolderOpen(func(u fyne.ListableURI, err error) {
		if err != nil {
			dialog.ShowError(err, d.win)
			return
		}
		if u == nil {
			return
		}

		d.setProject(u)
		d.win.Show()
		w.Close()

		addRecent(u, fyne.CurrentApp().Preferences())
	}, w)
}

func (d *defyne) showProjectSelect() {
	a := fyne.CurrentApp()
	w := a.NewWindow("Defyne : Open Project")
	if d.projectRoot == nil { // this is our welcome screen
		w.SetMainMenu(fyne.NewMainMenu(
			fyne.NewMenu("File",
				fyne.NewMenuItem("Open Project...", w.Show),
			),
			d.makeHelpMenu(),
		))
	}

	img := canvas.NewImageFromResource(resourceIconPng)
	img.FillMode = canvas.ImageFillContain
	img.SetMinSize(fyne.NewSize(192, 192))
	openProject := widget.NewButton("Open Project", func() {
		d.showOpenProjectDialog(w)
	})
	openProject.Importance = widget.HighImportance
	open := container.NewBorder(widget.NewLabelWithStyle("Defyne", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
		openProject, nil, nil, container.NewCenter(img))

	create := widget.NewButton("New Project", func() {
		d.showNewProjectDialog(w)
	})
	w.SetContent(container.NewGridWithColumns(2,
		container.NewBorder(widget.NewLabel("Recent projects"), create, nil, nil,
			makeRecentList(a.Preferences(), func(u fyne.URI) {
				d.setProject(u)
				d.win.Show()
				w.Close()

				addRecent(u, fyne.CurrentApp().Preferences())
			})),
		open))
	w.Resize(fyne.NewSize(620, 440))
	w.Show()
}

func createProject(parent fyne.URI, name string) (fyne.URI, error) {
	dir, err := storage.Child(parent, name)
	if err != nil {
		return nil, err
	}

	err = storage.CreateListable(dir)
	if err != nil {
		return nil, err
	}

	err = writeFile(dir, "main.go", `package main

import "fyne.io/fyne/v2/app"

func main() {
	a := app.New()
	w := a.NewWindow("`+name+`")

	g := newGUI()
	w.SetContent(g.makeUI())
	w.ShowAndRun()
}
`)
	if err != nil {
		fyne.LogError("Failed to write main.go", err) // we can just return the partial project
		return dir, nil
	}
	err = writeFile(dir, "main.gui.go", `// auto-generated
// Code generated by Defyne GUI builder.

package main

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

type gui struct {}

func newGUI() *gui {
	return &gui{}
}

func (g *gui) makeUI() fyne.CanvasObject {
	return container.NewVBox(
		widget.NewLabel("Hello `+name+`!"))

}
`)
	if err != nil {
		fyne.LogError("Failed to write main.go", err) // we can just return the partial project
		return dir, nil
	}
	err = writeFile(dir, "main.gui.json", `{
  "Type": "*fyne.Container",
  "Layout": "VBox",
  "Name": "",
  "Objects": [
    {
      "Type": "*widget.Label",
      "Struct": {
        "Hidden": false,
        "Text": "Hello `+name+`!",
        "Alignment": 0,
        "Wrapping": 0,
        "TextStyle": {
          "Bold": false,
          "Italic": false,
          "Monospace": false,
          "Symbol": false,
          "TabWidth": 0
        }
      }
    }
  ],
  "Properties": {
    "dir": "vertical",
    "layout": "VBox"
  }
}
`)
	if err != nil {
		fyne.LogError("Failed to write main.gui.json", err) // we can just return the partial project
		return dir, nil
	}

	err = writeFile(dir, "go.mod", `module `+name+`

require fyne.io/fyne/v2 v2.2.0
`)
	if err != nil {
		fyne.LogError("Failed to write go.mod", err) // we can just return the partial project
		return dir, nil
	}

	cmd := exec.Command("go", "mod", "tidy")
	cmd.Dir = dir.Path()
	err = cmd.Start() // run in background - may take a little while but should not block file editing
	if err != nil {   // just print, can just continue to open project
		fyne.LogError("Could not run go mod tidy", err)
	}
	return dir, nil
}

func defaultDir() fyne.ListableURI {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		fyne.LogError("Failed to get user home directory", err)
		return nil
	}
	defaultDir := storage.NewFileURI(homeDir)
	newDir, err := storage.ListerForURI(defaultDir)
	if err != nil {
		fyne.LogError("Failed to list home directory", err)
		return nil
	}
	return newDir
}

func writeFile(dir fyne.URI, name, content string) error {
	modURI, _ := storage.Child(dir, name)

	w, err := storage.Writer(modURI)
	if err != nil {
		return err
	}
	_, err = io.WriteString(w, content)
	if err != nil {
		fyne.LogError("Failed to write go.mod", err)
		return err
	}
	return w.Close()
}

package main

import (
	"fmt"
	"log"

	"fyne.io/fyne"
	"fyne.io/fyne/app"
	"fyne.io/fyne/canvas"
	"fyne.io/fyne/container"
	"fyne.io/fyne/theme"
)

var images map[string]*canvas.Image

func check(e error) {
	if e != nil {
		log.Fatal(e)
	}
}

func loadImages() map[string]*canvas.Image {
	res := make(map[string]*canvas.Image)
	files := []string{
		"0", "1", "2", "3", "4", "5", "6", "7", "8",
		"bomb_gray", "bomb_red", "flag",
		"unknown", "uncertain",
	}
	for _, f := range files {
		filename := fmt.Sprintf("%s.png", f)
		path := "image/" + filename
		res[f] = canvas.NewImageFromFile(path)
	}
	return res
}

func canvasScreen(_ fyne.Window) fyne.CanvasObject {
	imgs := []fyne.CanvasObject{}
	for i := 0; i < 9; i++ {
		for j := 0; j < 9; j++ {
			p := j % 10
			name := fmt.Sprintf("image/%d.png", p)
			if p == 9 {
				name = "image/uncertain.png"
			}
			imgs = append(imgs, canvas.NewImageFromFile(name))
		}
	}
	return container.NewGridWrap(fyne.NewSize(24, 24), imgs...)
}

func main() {
	images = loadImages()
	a := app.NewWithID("minebot")
	a.SetIcon(theme.FyneLogo())
	w := a.NewWindow("minebot")

	w.SetMaster()

	content := container.NewMax()
	tutorial := container.NewBorder(nil, nil, nil, nil, content)
	w.SetContent(tutorial)
	w.Resize(fyne.NewSize(260, 100)) //size(845, 400) for (30, 16)
	w.SetFixedSize(true)
	content.Objects = []fyne.CanvasObject{canvasScreen(w)}
	content.Refresh()
	w.ShowAndRun()
}

package main

import (
	"fmt"

	"fyne.io/fyne"
	"fyne.io/fyne/app"
	"fyne.io/fyne/canvas"
	"fyne.io/fyne/container"
)

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
	a := app.NewWithID("minebot")
	w := a.NewWindow("minebot")

	w.SetMaster()

	content := container.NewMax()
	tutorial := container.NewBorder(nil, nil, nil, nil, content)
	w.SetContent(tutorial)
	w.Resize(fyne.NewSize(260, 100)) //size(845, 400) for (30, 16)
	w.SetFixedSize(true)

	repaint := func() {
		content.Objects = []fyne.CanvasObject{canvasScreen(w)}
		content.Refresh()
	}
	repaint()
	w.Canvas().SetOnTypedKey(func(k *fyne.KeyEvent) {
		if k.Name == fyne.KeySpace {
			repaint()
		}
	})
	w.ShowAndRun()
}

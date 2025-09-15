package main

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"

	"oks/internal/serialterminal"
	"oks/internal/ui"
)

func main() {
	myApp := app.New()
	myWindow := myApp.NewWindow("COM Port Communicator")
	myWindow.Resize(fyne.NewSize(800, 700))

	terminal1 := serialterminal.New("/tmp/ttyS0")
	terminal2 := serialterminal.New("/tmp/ttyS1")

	ui1 := ui.New(terminal1, myWindow)
	ui2 := ui.New(terminal2, myWindow)

	tabs := container.NewAppTabs(
		container.NewTabItem("Terminal 1", ui1.Layout()),
		container.NewTabItem("Terminal 2", ui2.Layout()),
	)

	myWindow.SetContent(tabs)
	myWindow.ShowAndRun()

	err := terminal1.Disconnect()
	if err != nil {
		return
	}
	err = terminal2.Disconnect()
	if err != nil {
		return
	}
}

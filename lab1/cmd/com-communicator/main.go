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
	myWindow := myApp.NewWindow("Serial Port Communicator")
	myWindow.Resize(fyne.NewSize(800, 700))

	terminal1 := serialterminal.New("/dev/ttys001")
	terminal2 := serialterminal.New("/dev/ttys003")

	ui1 := ui.New(terminal1, myWindow)
	ui2 := ui.New(terminal2, myWindow)

	tabs := container.NewAppTabs(
		container.NewTabItem("Port 1", ui1.Layout()),
		container.NewTabItem("Port 2", ui2.Layout()),
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

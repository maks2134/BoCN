package ui

import (
	"strconv"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"oks/internal/serialterminal"
)

type TerminalUI struct {
	terminal       *serialterminal.SerialTerminal
	inputEntry     *widget.Entry
	outputArea     *widget.Entry
	connectButton  *widget.Button
	statusLabel    *widget.Label
	portEntry      *widget.Entry
	byteSizeSelect *widget.Select
	window         fyne.Window
}

func New(term *serialterminal.SerialTerminal, w fyne.Window) *TerminalUI {
	ui := &TerminalUI{
		terminal:       term,
		window:         w,
		inputEntry:     widget.NewEntry(),
		outputArea:     widget.NewMultiLineEntry(),
		statusLabel:    widget.NewLabel("Disconnected"),
		portEntry:      widget.NewEntry(),
		byteSizeSelect: widget.NewSelect([]string{"5", "6", "7", "8"}, nil),
	}

	ui.outputArea.Disable()
	ui.outputArea.SetMinRowsVisible(10)
	ui.outputArea.Wrapping = fyne.TextWrapWord
	ui.inputEntry.Disable()

	ui.terminal.OnMessage = ui.handleMessage
	ui.terminal.OnStatus = ui.handleStatus

	ui.portEntry.SetText(ui.terminal.GetPortName())
	ui.byteSizeSelect.SetSelected(strconv.Itoa(ui.terminal.GetDataBits()))

	ui.portEntry.OnChanged = func(s string) {
		ui.terminal.SetPortName(s)
	}

	ui.byteSizeSelect.OnChanged = func(s string) {
		if val, err := strconv.Atoi(s); err == nil {
			ui.terminal.SetDataBits(val)
		}
	}

	ui.connectButton = widget.NewButton("Connect", ui.toggleConnect)

	return ui
}

func (ui *TerminalUI) handleMessage(msg string) {
	currentText := ui.outputArea.Text
	if currentText != "" {
		currentText += "\n"
	}
	ui.outputArea.SetText(currentText + msg)
	ui.outputArea.CursorRow = len(ui.outputArea.Text)
}

func (ui *TerminalUI) handleStatus(status string) {
	ui.statusLabel.SetText(status)
	if ui.terminal.IsConnected() {
		ui.connectButton.SetText("Disconnect")
		ui.inputEntry.Enable()
	} else {
		ui.connectButton.SetText("Connect")
		ui.inputEntry.Disable()
	}
}

func (ui *TerminalUI) toggleConnect() {
	if !ui.terminal.IsConnected() {
		err := ui.terminal.Connect()
		if err != nil {
			dialog.ShowError(err, ui.window)
		}
	} else {
		err := ui.terminal.Disconnect()
		if err != nil {
			dialog.ShowError(err, ui.window)
		}
	}
}

func (ui *TerminalUI) sendMessage() {
	msg := ui.inputEntry.Text
	if msg == "" {
		return
	}

	err := ui.terminal.SendMessage(msg)
	if err != nil {
		dialog.ShowError(err, ui.window)
	} else {
		ui.inputEntry.SetText("")
	}
}

func (ui *TerminalUI) Layout() fyne.CanvasObject {
	sendButton := widget.NewButton("Send", func() {
		ui.sendMessage()
	})

	settingsGrid := container.NewGridWithColumns(2,
		widget.NewLabel("Port Name:"),
		ui.portEntry,
		widget.NewLabel("Byte Size:"),
		ui.byteSizeSelect,
	)

	settingsBox := container.NewBorder(
		widget.NewLabel("Connection Settings"),
		nil, nil, nil,
		settingsGrid,
	)

	outputScroll := container.NewScroll(ui.outputArea)
	outputScroll.SetMinSize(fyne.NewSize(400, 300))

	return container.NewVBox(
		container.NewHBox(
			widget.NewLabel("Status:"),
			ui.statusLabel,
			ui.connectButton,
		),
		settingsBox,
		widget.NewLabel("Input:"),
		ui.inputEntry,
		sendButton,
		widget.NewLabel("Output:"),
		outputScroll,
	)
}

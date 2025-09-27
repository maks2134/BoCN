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
	terminal         *serialterminal.SerialTerminal
	inputEntry       *widget.Entry
	sentMessages     *widget.Entry
	receivedMessages *widget.Entry
	sentPacketInfo   *widget.RichText
	openButton       *widget.Button
	statusLabel      *widget.Label
	portEntry        *widget.Entry
	byteSizeSelect   *widget.Select
	window           fyne.Window
}

func New(term *serialterminal.SerialTerminal, w fyne.Window) *TerminalUI {
	ui := &TerminalUI{
		terminal:         term,
		window:           w,
		inputEntry:       widget.NewEntry(),
		sentMessages:     widget.NewMultiLineEntry(),
		receivedMessages: widget.NewMultiLineEntry(),
		sentPacketInfo:   widget.NewRichTextFromMarkdown(""),
		statusLabel:      widget.NewLabel("Port closed"),
		portEntry:        widget.NewEntry(),
		byteSizeSelect:   widget.NewSelect([]string{"5", "6", "7", "8"}, nil),
	}

	ui.sentMessages.Disable()
	ui.sentMessages.SetMinRowsVisible(5)
	ui.sentMessages.Wrapping = fyne.TextWrapWord
	ui.sentMessages.SetPlaceHolder("")

	ui.receivedMessages.Disable()
	ui.receivedMessages.SetMinRowsVisible(5)
	ui.receivedMessages.Wrapping = fyne.TextWrapWord
	ui.receivedMessages.SetPlaceHolder("")

	ui.sentPacketInfo.ParseMarkdown("Frame structure will appear here after sending a message")

	ui.inputEntry.Disable()

	ui.terminal.OnMessage = ui.handleMessage
	ui.terminal.OnStatus = ui.handleStatus
	ui.terminal.OnPacket = ui.handlePacket

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

	ui.openButton = widget.NewButton("Open Port", ui.togglePort)

	return ui
}

func (ui *TerminalUI) handleMessage(msg string) {
	if len(msg) > 3 && msg[:3] == "TX:" {
		message := msg[3:]
		currentText := ui.sentMessages.Text
		if currentText != "" {
			currentText += "\n"
		}
		ui.sentMessages.SetText(currentText + message)
		ui.sentMessages.CursorRow = len(ui.sentMessages.Text)
	} else if len(msg) > 3 && msg[:3] == "RX:" {
		message := msg[3:]
		currentText := ui.receivedMessages.Text
		if currentText != "" {
			currentText += "\n"
		}
		ui.receivedMessages.SetText(currentText + message)
		ui.receivedMessages.CursorRow = len(ui.receivedMessages.Text)
	} else {
		currentText := ui.receivedMessages.Text
		if currentText != "" {
			currentText += "\n"
		}
		ui.receivedMessages.SetText(currentText + msg)
		ui.receivedMessages.CursorRow = len(ui.receivedMessages.Text)
	}
}

func (ui *TerminalUI) handlePacket(packetInfo string) {
	if ui.terminal.GetPortName() == ui.portEntry.Text {
		ui.sentPacketInfo.ParseMarkdown(packetInfo)
	}
}

func (ui *TerminalUI) handleStatus(status string) {
	ui.statusLabel.SetText(status)
	if ui.terminal.IsConnected() {
		ui.openButton.SetText("Close Port")
		ui.inputEntry.Enable()
	} else {
		ui.openButton.SetText("Open Port")
		ui.inputEntry.Disable()
	}
}

func (ui *TerminalUI) togglePort() {
	if !ui.terminal.IsConnected() {
		err := ui.terminal.Connect()
		if err != nil {
			ui.showErrorDialog("Port Opening Failed", err.Error())
		}
	} else {
		err := ui.terminal.Disconnect()
		if err != nil {
			ui.showErrorDialog("Port Closing Failed", err.Error())
		}
	}
}

func (ui *TerminalUI) sendData() {
	msg := ui.inputEntry.Text
	if msg == "" {
		return
	}

	err := ui.terminal.SendMessage(msg)
	if err != nil {
		ui.showErrorDialog("Message Sending Failed", err.Error())
	} else {
		ui.inputEntry.SetText("")
	}
}

func (ui *TerminalUI) showErrorDialog(title, message string) {
	dialog.ShowCustom(title, "OK", widget.NewLabel(message), ui.window)
}

func (ui *TerminalUI) Layout() fyne.CanvasObject {
	sendButton := widget.NewButton("Send Message", func() {
		ui.sendData()
	})

	settingsGrid := container.NewGridWithColumns(2,
		widget.NewLabel("Port:"),
		ui.portEntry,
		widget.NewLabel("Data Bits:"),
		ui.byteSizeSelect,
	)

	settingsBox := container.NewBorder(
		widget.NewLabel("Port Configuration"),
		nil, nil, nil,
		settingsGrid,
	)

	sentScroll := container.NewScroll(ui.sentMessages)
	sentScroll.SetMinSize(fyne.NewSize(380, 150))

	receivedScroll := container.NewScroll(ui.receivedMessages)
	receivedScroll.SetMinSize(fyne.NewSize(380, 150))

	messagesGroup := container.NewGridWithColumns(2,
		container.NewBorder(
			widget.NewLabel("Sent Messages"),
			nil, nil, nil,
			sentScroll,
		),
		container.NewBorder(
			widget.NewLabel("Received Messages"),
			nil, nil, nil,
			receivedScroll,
		),
	)

	packetInfoScroll := container.NewScroll(ui.sentPacketInfo)
	packetInfoScroll.SetMinSize(fyne.NewSize(800, 200))

	packetInfoGroup := container.NewBorder(
		widget.NewLabel("Transmitted Frame Structure"),
		nil, nil, nil,
		packetInfoScroll,
	)

	return container.NewVBox(
		container.NewHBox(
			widget.NewLabel("Status:"),
			ui.statusLabel,
			ui.openButton,
		),
		settingsBox,
		widget.NewLabel("Message to send:"),
		ui.inputEntry,
		sendButton,
		messagesGroup,
		packetInfoGroup,
	)
}

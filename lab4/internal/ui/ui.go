package ui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

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

	eventLog          *widget.Entry
	emulationCheckbox *widget.Check

	window fyne.Window
}

func New(term *serialterminal.SerialTerminal, w fyne.Window) *TerminalUI {
	ui := &TerminalUI{
		terminal:          term,
		window:            w,
		inputEntry:        widget.NewEntry(),
		sentMessages:      widget.NewMultiLineEntry(),
		receivedMessages:  widget.NewMultiLineEntry(),
		sentPacketInfo:    widget.NewRichTextFromMarkdown(""),
		statusLabel:       widget.NewLabel("Port closed"),
		portEntry:         widget.NewEntry(),
		byteSizeSelect:    widget.NewSelect([]string{"5", "6", "7", "8"}, nil),
		eventLog:          widget.NewMultiLineEntry(),
		emulationCheckbox: widget.NewCheck("Enable CSMA/CD Emulation", nil),
	}

	ui.sentMessages.Disable()
	ui.sentMessages.SetMinRowsVisible(3)
	ui.sentMessages.Wrapping = fyne.TextWrapWord

	ui.receivedMessages.Disable()
	ui.receivedMessages.SetMinRowsVisible(3)
	ui.receivedMessages.Wrapping = fyne.TextWrapWord

	ui.sentPacketInfo.ParseMarkdown("Frame structure will appear here after sending a message")

	ui.inputEntry.Disable()

	ui.terminal.OnMessage = ui.handleMessage
	ui.terminal.OnStatus = ui.handleStatus
	ui.terminal.OnPacket = ui.handlePacket
	ui.terminal.OnCollision = ui.handleCollision
	ui.terminal.OnChannelBusy = ui.handleChannelBusy
	ui.terminal.OnChannelState = ui.handleChannelState

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

	ui.emulationCheckbox.SetChecked(true)
	ui.emulationCheckbox.OnChanged = func(checked bool) {
		ui.terminal.SetCSMAEmulation(checked)
		if checked {
			ui.terminal.SetCSMAProbabilities(0.25, 0.75)
		}
	}

	ui.eventLog.Disable()
	ui.eventLog.SetMinRowsVisible(6)
	ui.eventLog.Wrapping = fyne.TextWrapWord

	ui.openButton = widget.NewButton("Open Port", ui.togglePort)

	return ui
}

func (ui *TerminalUI) handleCollision() {
	ui.appendEventLogWithStats("Collision detected!")
}

func (ui *TerminalUI) handleChannelBusy() {
	ui.appendEventLogWithStats("Channel busy detected")
}

func (ui *TerminalUI) handleChannelState(state string) {
	ui.appendEventLogWithStats("Channel state changed: " + state)
}

func (ui *TerminalUI) appendEventLogWithStats(entry string) {
	timestamp := time.Now().Format("15:04:05")
	collisions, busy, total := ui.terminal.GetCSMAStatistics()
	text := fmt.Sprintf("[%s] %s | Collisions=%d Busy=%d Total=%d", timestamp, entry, collisions, busy, total)
	ui.appendEventLog(text)
}

func (ui *TerminalUI) appendEventLog(entry string) {
	currentText := ui.eventLog.Text
	if currentText != "" {
		currentText += "\n"
	}
	ui.eventLog.SetText(currentText + entry)
	ui.eventLog.CursorRow = len(strings.Split(ui.eventLog.Text, "\n"))
}

func (ui *TerminalUI) handleMessage(msg string) {
	if len(msg) > 3 && msg[:3] == "TX:" {
		message := msg[3:]
		ui.sentMessages.SetText(ui.sentMessages.Text + "\n" + message)
		ui.appendEventLogWithStats("Message sent: " + message)
	} else if len(msg) > 3 && msg[:3] == "RX:" {
		message := msg[3:]
		ui.receivedMessages.SetText(ui.receivedMessages.Text + "\n" + message)
		ui.appendEventLogWithStats("Message received: " + message)
	} else {
		ui.receivedMessages.SetText(ui.receivedMessages.Text + "\n" + msg)
		ui.appendEventLogWithStats("Message received: " + msg)
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

	csmaConfigBox := container.NewVBox(
		widget.NewLabel("CSMA/CD Configuration"),
		ui.emulationCheckbox,
	)

	csmaLogScroll := container.NewScroll(ui.eventLog)
	csmaLogScroll.SetMinSize(fyne.NewSize(100, 160))
	csmaLogContainer := container.NewVBox(csmaLogScroll)

	csmaPanel := container.NewHSplit(csmaConfigBox, csmaLogContainer)
	csmaPanel.SetOffset(0.3)

	sentScroll := container.NewScroll(ui.sentMessages)
	sentScroll.SetMinSize(fyne.NewSize(100, 120))
	receivedScroll := container.NewScroll(ui.receivedMessages)
	receivedScroll.SetMinSize(fyne.NewSize(100, 120))

	messagesGroup := container.NewHSplit(
		container.NewVBox(widget.NewLabel("Sent Messages"), sentScroll),
		container.NewVBox(widget.NewLabel("Received Messages"), receivedScroll),
	)
	messagesGroup.SetOffset(0.5)

	messageInputPanel := container.NewVBox(
		widget.NewLabel("Message to send:"),
		ui.inputEntry,
		sendButton,
	)

	topBlock := container.NewVBox(
		settingsBox,
		csmaPanel,
		messageInputPanel,
		messagesGroup,
	)

	packetInfoScroll := container.NewScroll(ui.sentPacketInfo)
	packetInfoScroll.SetMinSize(fyne.NewSize(100, 160))
	packetInfoContainer := container.NewVBox(packetInfoScroll)
	packetInfoGroup := container.NewVBox(
		widget.NewLabel("Transmitted Frame Structure"),
		packetInfoContainer,
	)

	topBar := container.NewHBox(
		widget.NewLabel("Status:"), ui.statusLabel, ui.openButton,
	)

	topArea := container.NewVBox(
		topBar,
		topBlock,
	)

	mainSplit := container.NewVSplit(topArea, packetInfoGroup)
	mainSplit.SetOffset(0.8)

	return mainSplit
}

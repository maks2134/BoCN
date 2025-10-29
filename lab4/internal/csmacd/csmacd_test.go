package csmacd

import (
	"testing"
	"time"
)

func TestCSMACDInitialization(t *testing.T) {
	csma := NewCSMACD()

	if csma.GetChannelState() != ChannelIdle {
		t.Errorf("Expected initial channel state to be Idle, got %v", csma.GetChannelState())
	}

	collisions, busy, total := csma.GetStatistics()
	if collisions != 0 || busy != 0 || total != 0 {
		t.Errorf("Expected initial statistics to be all zeros, got collisions=%d, busy=%d, total=%d",
			collisions, busy, total)
	}
}

func TestChannelListening(t *testing.T) {
	csma := NewCSMACD()
	csma.SetEmulationEnabled(false)

	if !csma.ListenToChannel() {
		t.Error("Expected channel to be idle and listening to succeed")
	}

	if !csma.StartTransmission() {
		t.Error("Expected to be able to start transmission on idle channel")
	}

	if csma.GetChannelState() != ChannelBusy {
		t.Errorf("Expected channel state to be Busy after starting transmission, got %v",
			csma.GetChannelState())
	}

	if csma.ListenToChannel() {
		t.Error("Expected listening to fail when channel is busy")
	}

	csma.EndTransmission()
	if csma.GetChannelState() != ChannelIdle {
		t.Errorf("Expected channel state to be Idle after ending transmission, got %v",
			csma.GetChannelState())
	}
}

func TestCollisionDetection(t *testing.T) {
	csma := NewCSMACD()
	csma.SetEmulationEnabled(false)

	csma.StartTransmission()

	if csma.DetectCollision() {
		t.Error("Expected no collision when emulation is disabled")
	}

	csma.EndTransmission()
}

func TestBackoffCalculation(t *testing.T) {
	csma := NewCSMACD()

	delay := csma.CalculateBackoffDelay()
	if delay < 0 {
		t.Errorf("Expected backoff delay to be non-negative, got %v", delay)
	}

	if delay > time.Second {
		t.Errorf("Expected backoff delay to be reasonable, got %v", delay)
	}
}

func TestStatistics(t *testing.T) {
	csma := NewCSMACD()
	csma.SetEmulationEnabled(false)

	csma.ListenToChannel()
	csma.StartTransmission()
	csma.EndTransmission()

	_, _, total := csma.GetStatistics()
	if total < 1 {
		t.Errorf("Expected total attempts to be at least 1, got %d", total)
	}
}

func TestStateString(t *testing.T) {
	csma := NewCSMACD()

	stateStr := csma.GetStateString()
	if stateStr != "Idle" {
		t.Errorf("Expected state string to be 'Idle', got '%s'", stateStr)
	}

	csma.StartTransmission()
	stateStr = csma.GetStateString()
	if stateStr != "Busy" {
		t.Errorf("Expected state string to be 'Busy', got '%s'", stateStr)
	}

	csma.EndTransmission()
}

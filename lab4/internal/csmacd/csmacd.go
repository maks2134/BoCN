package csmacd

import (
	"fmt"
	"math/rand"
	"sync"
	"time"
)

type ChannelState int

const (
	ChannelIdle ChannelState = iota
	ChannelBusy
	ChannelCollision
)

type CSMACD struct {
	channelState         ChannelState
	channelMutex         sync.RWMutex
	collisionCount       int
	busyCount            int
	totalAttempts        int
	backoffAttempts      int
	maxBackoff           int
	jamSignal            bool
	emulationEnabled     bool
	busyProbability      float64
	collisionProbability float64
	onStateChange        func(ChannelState)
	onCollision          func()
	onChannelBusy        func()
}

func NewCSMACD() *CSMACD {
	return &CSMACD{
		channelState:         ChannelIdle,
		maxBackoff:           10,
		emulationEnabled:     true,
		busyProbability:      0.25,
		collisionProbability: 0.75,
		onStateChange:        func(ChannelState) {},
		onCollision:          func() {},
		onChannelBusy:        func() {},
	}
}

func (c *CSMACD) SetEmulationEnabled(enabled bool) {
	c.emulationEnabled = enabled
}

func (c *CSMACD) SetProbabilities(busyProb, collisionProb float64) {
	c.busyProbability = busyProb
	c.collisionProbability = collisionProb
}

func (c *CSMACD) SetCallbacks(onStateChange func(ChannelState), onCollision func(), onChannelBusy func()) {
	c.onStateChange = onStateChange
	c.onCollision = onCollision
	c.onChannelBusy = onChannelBusy
}

func (c *CSMACD) GetChannelState() ChannelState {
	c.channelMutex.RLock()
	defer c.channelMutex.RUnlock()
	return c.channelState
}

func (c *CSMACD) GetStatistics() (collisions, busy, totalAttempts int) {
	c.channelMutex.RLock()
	defer c.channelMutex.RUnlock()
	return c.collisionCount, c.busyCount, c.totalAttempts
}

func (c *CSMACD) ListenToChannel() bool {
	c.channelMutex.Lock()
	defer c.channelMutex.Unlock()

	c.totalAttempts++

	if c.channelState == ChannelBusy {
		c.busyCount++
		c.onChannelBusy()
		return false
	}

	if c.emulationEnabled && rand.Float64() < c.busyProbability {
		c.channelState = ChannelBusy
		c.busyCount++

		go func() {
			time.Sleep(time.Duration(rand.Intn(1000)+500) * time.Millisecond)
			c.channelMutex.Lock()
			c.channelState = ChannelIdle
			c.channelMutex.Unlock()
		}()

		return false
	}

	return true
}

func (c *CSMACD) DetectCollision() bool {
	c.channelMutex.Lock()
	defer c.channelMutex.Unlock()

	if c.emulationEnabled && rand.Float64() < c.collisionProbability {
		c.channelState = ChannelCollision
		c.collisionCount++
		c.backoffAttempts++

		go func() {
			time.Sleep(time.Duration(rand.Intn(100)+50) * time.Millisecond)
			c.channelMutex.Lock()
			c.channelState = ChannelIdle
			c.channelMutex.Unlock()
		}()

		return true
	}

	return false
}

func (c *CSMACD) CalculateBackoffDelay() time.Duration {
	attempts := c.backoffAttempts
	if attempts > c.maxBackoff {
		attempts = c.maxBackoff
	}

	slotTime := 51 * time.Microsecond
	backoffWindow := (1 << attempts) - 1
	randomDelay := rand.Intn(backoffWindow + 1)
	delay := time.Duration(randomDelay) * slotTime

	return delay
}

func (c *CSMACD) ResetBackoff() {
	c.channelMutex.Lock()
	defer c.channelMutex.Unlock()
	c.backoffAttempts = 0
}

func (c *CSMACD) StartTransmission() bool {
	c.channelMutex.Lock()
	defer c.channelMutex.Unlock()

	if c.channelState != ChannelIdle {
		return false
	}

	c.channelState = ChannelBusy
	return true
}

func (c *CSMACD) EndTransmission() {
	c.channelMutex.Lock()
	defer c.channelMutex.Unlock()

	c.channelState = ChannelIdle
}

func (c *CSMACD) SendJamSignal() {
	c.channelMutex.Lock()
	defer c.channelMutex.Unlock()

	c.jamSignal = true
	go func() {
		time.Sleep(4 * time.Microsecond)
		c.channelMutex.Lock()
		c.jamSignal = false
		c.channelMutex.Unlock()
	}()
}

func (c *CSMACD) IsJamSignalActive() bool {
	c.channelMutex.RLock()
	defer c.channelMutex.RUnlock()
	return c.jamSignal
}

func (c *CSMACD) GetStateString() string {
	switch c.GetChannelState() {
	case ChannelIdle:
		return "Idle"
	case ChannelBusy:
		return "Busy"
	case ChannelCollision:
		return "Collision"
	default:
		return "Unknown"
	}
}

func (c *CSMACD) GetStatisticsString() string {
	collisions, busy, total := c.GetStatistics()
	return fmt.Sprintf("Collisions: %d | Busy: %d | Total Attempts: %d | Backoff Attempts: %d",
		collisions, busy, total, c.backoffAttempts)
}

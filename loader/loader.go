package loader

import (
	"fmt"
	"time"

	"github.com/fatih/color"
)

// Loader represents a custom loading animation
type Loader struct {
	frames        []string
	interval      time.Duration
	prefix        string
	color         *color.Color
	isActive      bool
	stopChan      chan struct{}
	currentFrame  int
}

// New creates a new Loader instance
func New(prefix string) *Loader {
	return &Loader{
		frames:   []string{"⠾", "⠽", "⠻", "⠯", "⠷"},
		interval: 100 * time.Millisecond,
		prefix:   prefix,
		color:    color.New(color.FgCyan),
		stopChan: make(chan struct{}),
	}
}

// SetColor sets the color of the loader animation
// Color returns the loader's current color
func (l *Loader) Color() *color.Color {
	return l.color
}

// CurrentFrame returns the current animation frame
func (l *Loader) CurrentFrame() string {
	return l.frames[l.currentFrame]
}

func (l *Loader) SetColor(c *color.Color) {
	l.color = c
}

// SetInterval sets the animation interval
func (l *Loader) SetInterval(d time.Duration) {
	l.interval = d
}

// Start begins the loading animation
func (l *Loader) Start() {
	if l.isActive {
		return
	}
	l.isActive = true

	go func() {

		for {
			select {
			case <-l.stopChan:
				return
			default:
				fmt.Printf("\r%s %s", l.prefix, l.Color().Sprint(l.CurrentFrame()))
				l.currentFrame = (l.currentFrame + 1) % len(l.frames)
				time.Sleep(l.interval)
			}
		}
	}()
}

// Stop stops the loading animation
func (l *Loader) Stop() {
	if !l.isActive {
		return
	}
	l.isActive = false
	l.stopChan <- struct{}{}
	fmt.Print("\r") // Clear the line
}
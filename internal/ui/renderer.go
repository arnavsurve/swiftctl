package ui

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/fatih/color"
)

type Renderer struct {
	mu          sync.Mutex
	spinning    bool
	spinnerDone chan struct{}
}

func NewRenderer() *Renderer {
	return &Renderer{}
}

var (
	green  = color.New(color.FgGreen).SprintFunc()
	red    = color.New(color.FgRed).SprintFunc()
	yellow = color.New(color.FgYellow).SprintFunc()
	cyan   = color.New(color.FgCyan).SprintFunc()
	dim    = color.New(color.Faint).SprintFunc()
	bold   = color.New(color.Bold).SprintFunc()
)

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

func (r *Renderer) StartSpinner(format string, args ...any) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.spinning {
		return
	}

	r.spinning = true
	r.spinnerDone = make(chan struct{})

	msg := fmt.Sprintf(format, args...)

	go func() {
		frame := 0
		ticker := time.NewTicker(80 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-r.spinnerDone:
				return
			case <-ticker.C:
				r.mu.Lock()
				fmt.Fprintf(os.Stderr, "\r%s %s", cyan(spinnerFrames[frame]), msg)
				r.mu.Unlock()
				frame = (frame + 1) % len(spinnerFrames)
			}
		}
	}()
}

func (r *Renderer) StopSpinner(success bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.spinning {
		return
	}

	close(r.spinnerDone)
	r.spinning = false

	fmt.Fprint(os.Stderr, "\r\033[K")
}

func (r *Renderer) Success(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "%s %s\n", green("✓"), fmt.Sprintf(format, args...))
}

func (r *Renderer) Error(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "%s %s\n", red("✗"), fmt.Sprintf(format, args...))
}

func (r *Renderer) Warning(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "%s %s\n", yellow("!"), fmt.Sprintf(format, args...))
}

func (r *Renderer) Info(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "  %s\n", fmt.Sprintf(format, args...))
}

func (r *Renderer) Dim(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "  %s\n", dim(fmt.Sprintf(format, args...)))
}

type DeviceInfo struct {
	Name      string
	UDID      string
	State     string
	OSVersion string
	Platform  string
}

func (r *Renderer) RenderDeviceList(devices []DeviceInfo) {
	if len(devices) == 0 {
		r.Info("No devices found")
		return
	}

	byPlatform := make(map[string][]DeviceInfo)
	for _, d := range devices {
		byPlatform[d.Platform] = append(byPlatform[d.Platform], d)
	}

	for platform, devs := range byPlatform {
		fmt.Fprintf(os.Stderr, "\n%s\n", bold(strings.ToUpper(platform)))
		for _, d := range devs {
			stateColor := dim
			if d.State == "Booted" {
				stateColor = green
			}
			fmt.Fprintf(os.Stderr, "  %s %s %s\n",
				d.Name,
				dim(d.OSVersion),
				stateColor(fmt.Sprintf("[%s]", d.State)),
			)
		}
	}
	fmt.Fprintln(os.Stderr)
}

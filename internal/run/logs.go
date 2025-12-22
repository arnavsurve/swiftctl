package run

import (
	"context"

	"github.com/arnavsurve/swiftctl/internal/device"
	"github.com/arnavsurve/swiftctl/internal/process"
)

type LogStreamer struct {
	runner   *process.Runner
	device   *device.Device
	bundleID string
}

func NewLogStreamer(dev *device.Device, bundleID string) *LogStreamer {
	return &LogStreamer{
		runner:   process.NewRunner(),
		device:   dev,
		bundleID: bundleID,
	}
}

// Stream starts streaming logs and returns a channel of log lines.
func (l *LogStreamer) Stream(ctx context.Context) (<-chan string, <-chan error) {
	outChan := make(chan string, 100)
	errChan := make(chan error, 1)

	go func() {
		defer close(outChan)
		defer close(errChan)

		args := []string{
			"simctl", "spawn", l.device.UDID,
			"log", "stream",
			"--style", "compact",
			"--predicate", `processImagePath CONTAINS "` + l.bundleID + `"`,
		}

		lines, errs := l.runner.Run(ctx, "xcrun", args)

		for {
			select {
			case <-ctx.Done():
				return
			case line, ok := <-lines:
				if !ok {
					lines = nil
				} else {
					outChan <- line.Content
				}
			case err, ok := <-errs:
				if ok && err != nil {
					errChan <- err
					return
				}
				errs = nil
			}

			if lines == nil && errs == nil {
				break
			}
		}
	}()

	return outChan, errChan
}

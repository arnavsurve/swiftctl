package build

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/arnavsurve/swiftctl/internal/device"
	"github.com/arnavsurve/swiftctl/internal/process"
	"github.com/arnavsurve/swiftctl/internal/project"
)

type Configuration string

const (
	ConfigDebug   Configuration = "Debug"
	ConfigRelease Configuration = "Release"
)

type Config struct {
	Scheme        string
	Configuration Configuration
	Platform      device.Platform
	Destination   string
	DerivedData   string
	ExtraArgs     []string
}

type EventType int

const (
	EventCompileStart EventType = iota
	EventCompileFile
	EventLink
	EventSign
	EventWarning
	EventError
	EventSuccess
	EventFailure
)

type Event struct {
	Type    EventType
	Message string
	File    string
	Line    int
	Column  int
}

type Result struct {
	Success     bool
	ProductPath string
	Duration    time.Duration
	Warnings    []Event
	Errors      []Event
}

type Builder struct {
	project *project.ProjectInfo
	runner  *process.Runner
}

func NewBuilder(proj *project.ProjectInfo) *Builder {
	return &Builder{
		project: proj,
		runner:  process.NewRunner(),
	}
}

// Build compiles the project and streams events to the channel (can be nil).
func (b *Builder) Build(ctx context.Context, cfg Config, events chan<- Event) (*Result, error) {
	startTime := time.Now()
	result := &Result{}

	args := b.buildArgs(cfg)

	outChan, errChan := b.runner.Run(ctx, "xcodebuild", args)
	parser := &outputParser{events: events, result: result}

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()

		case line, ok := <-outChan:
			if !ok {
				outChan = nil
			} else {
				parser.parseLine(line.Content)
			}

		case err, ok := <-errChan:
			if !ok {
				errChan = nil
			} else if err != nil {
				result.Success = false
				result.Duration = time.Since(startTime)
				return result, fmt.Errorf("build failed: %w", err)
			}
		}

		if outChan == nil && errChan == nil {
			break
		}
	}

	result.Duration = time.Since(startTime)
	return result, nil
}

func (b *Builder) Clean(ctx context.Context, cfg Config) error {
	args := b.buildArgs(cfg)
	args = append(args, "clean")

	_, err := b.runner.RunSilent(ctx, "xcodebuild", args)
	return err
}

func (b *Builder) buildArgs(cfg Config) []string {
	var args []string

	switch b.project.Type {
	case project.ProjectTypeWorkspace:
		args = append(args, "-workspace", b.project.Path)
	case project.ProjectTypeXcodeProj:
		args = append(args, "-project", b.project.Path)
	case project.ProjectTypeSPM:
	}

	if cfg.Scheme != "" {
		args = append(args, "-scheme", cfg.Scheme)
	} else if len(b.project.Schemes) > 0 {
		args = append(args, "-scheme", b.project.Schemes[0])
	}

	if cfg.Configuration != "" {
		args = append(args, "-configuration", string(cfg.Configuration))
	}

	if cfg.Destination != "" {
		args = append(args, "-destination", cfg.Destination)
	} else if cfg.Platform != "" {
		args = append(args, "-destination", b.defaultDestination(cfg.Platform))
	}

	if cfg.DerivedData != "" {
		args = append(args, "-derivedDataPath", cfg.DerivedData)
	}

	args = append(args, cfg.ExtraArgs...)

	return args
}

func (b *Builder) defaultDestination(platform device.Platform) string {
	switch platform {
	case device.PlatformIOS:
		return "platform=iOS Simulator,name=iPhone 17 Pro"
	case device.PlatformMacOS:
		return "platform=macOS"
	case device.PlatformWatchOS:
		return "platform=watchOS Simulator,name=Apple Watch Series 9 (45mm)"
	case device.PlatformTVOS:
		return "platform=tvOS Simulator,name=Apple TV 4K (3rd generation)"
	case device.PlatformVisionOS:
		return "platform=visionOS Simulator,name=Apple Vision Pro"
	default:
		return "platform=iOS Simulator,name=iPhone 17 Pro"
	}
}

type outputParser struct {
	events chan<- Event
	result *Result
}

var (
	compilePattern    = regexp.MustCompile(`^CompileSwift\s+\w+\s+\w+\s+(.+)$`)
	diagnosticPattern = regexp.MustCompile(`^(.+):(\d+):(\d+):\s+(warning|error):\s+(.+)$`)
	linkPattern       = regexp.MustCompile(`^Linking\s+(.+)$`)
	signPattern       = regexp.MustCompile(`^CodeSign\s+(.+)$`)
	successPattern    = regexp.MustCompile(`\*\* BUILD SUCCEEDED \*\*`)
	failurePattern    = regexp.MustCompile(`\*\* BUILD FAILED \*\*`)
)

func (p *outputParser) parseLine(line string) {
	line = strings.TrimSpace(line)
	if line == "" {
		return
	}

	if matches := compilePattern.FindStringSubmatch(line); matches != nil {
		if p.events != nil {
			p.events <- Event{
				Type:    EventCompileFile,
				File:    matches[1],
				Message: matches[1],
			}
		}
		return
	}

	if matches := diagnosticPattern.FindStringSubmatch(line); matches != nil {
		evType := EventWarning
		if matches[4] == "error" {
			evType = EventError
		}
		lineNum, _ := strconv.Atoi(matches[2])
		col, _ := strconv.Atoi(matches[3])

		ev := Event{
			Type:    evType,
			File:    matches[1],
			Line:    lineNum,
			Column:  col,
			Message: matches[5],
		}

		if evType == EventWarning {
			p.result.Warnings = append(p.result.Warnings, ev)
		} else {
			p.result.Errors = append(p.result.Errors, ev)
		}

		if p.events != nil {
			p.events <- ev
		}
		return
	}

	if matches := linkPattern.FindStringSubmatch(line); matches != nil {
		if p.events != nil {
			p.events <- Event{
				Type:    EventLink,
				Message: matches[1],
			}
		}
		return
	}

	if matches := signPattern.FindStringSubmatch(line); matches != nil {
		if p.events != nil {
			p.events <- Event{
				Type:    EventSign,
				Message: matches[1],
			}
		}
		return
	}

	if successPattern.MatchString(line) {
		p.result.Success = true
		if p.events != nil {
			p.events <- Event{Type: EventSuccess}
		}
		return
	}

	if failurePattern.MatchString(line) {
		p.result.Success = false
		if p.events != nil {
			p.events <- Event{Type: EventFailure}
		}
		return
	}
}

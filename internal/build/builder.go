package build

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/asurve/swiftctl/internal/device"
	"github.com/asurve/swiftctl/internal/process"
	"github.com/asurve/swiftctl/internal/project"
)

// Configuration represents a build configuration
type Configuration string

const (
	ConfigDebug   Configuration = "Debug"
	ConfigRelease Configuration = "Release"
)

// Config holds build configuration options
type Config struct {
	Scheme        string
	Configuration Configuration
	Platform      device.Platform
	Destination   string   // e.g., "platform=iOS Simulator,name=iPhone 15 Pro"
	DerivedData   string   // custom derived data path
	ExtraArgs     []string // passthrough args
}

// EventType represents the type of build event
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

// Event represents a build event
type Event struct {
	Type    EventType
	Message string
	File    string
	Line    int
	Column  int
}

// Result contains the outcome of a build
type Result struct {
	Success     bool
	ProductPath string
	Duration    time.Duration
	Warnings    []Event
	Errors      []Event
}

// Builder compiles Swift projects
type Builder struct {
	project *project.ProjectInfo
	runner  *process.Runner
}

// NewBuilder creates a new Builder for the given project
func NewBuilder(proj *project.ProjectInfo) *Builder {
	return &Builder{
		project: proj,
		runner:  process.NewRunner(),
	}
}

// Build compiles the project and streams events
func (b *Builder) Build(ctx context.Context, cfg Config, events chan<- Event) (*Result, error) {
	startTime := time.Now()
	result := &Result{}

	args := b.buildArgs(cfg)

	// Start the build process
	outChan, errChan := b.runner.Run(ctx, "xcodebuild", args)

	// Parse output
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

// Clean removes build artifacts
func (b *Builder) Clean(ctx context.Context, cfg Config) error {
	args := b.buildArgs(cfg)
	args = append(args, "clean")

	_, err := b.runner.RunSilent(ctx, "xcodebuild", args)
	return err
}

// buildArgs constructs xcodebuild arguments
func (b *Builder) buildArgs(cfg Config) []string {
	var args []string

	// Project/workspace
	switch b.project.Type {
	case project.ProjectTypeWorkspace:
		args = append(args, "-workspace", b.project.Path)
	case project.ProjectTypeXcodeProj:
		args = append(args, "-project", b.project.Path)
	case project.ProjectTypeSPM:
		// SPM projects don't need -project flag
	}

	// Scheme
	if cfg.Scheme != "" {
		args = append(args, "-scheme", cfg.Scheme)
	} else if len(b.project.Schemes) > 0 {
		args = append(args, "-scheme", b.project.Schemes[0])
	}

	// Configuration
	if cfg.Configuration != "" {
		args = append(args, "-configuration", string(cfg.Configuration))
	}

	// Destination
	if cfg.Destination != "" {
		args = append(args, "-destination", cfg.Destination)
	} else if cfg.Platform != "" {
		args = append(args, "-destination", b.defaultDestination(cfg.Platform))
	}

	// Derived data
	if cfg.DerivedData != "" {
		args = append(args, "-derivedDataPath", cfg.DerivedData)
	}

	// Extra args
	args = append(args, cfg.ExtraArgs...)

	return args
}

// defaultDestination returns a default destination for a platform
func (b *Builder) defaultDestination(platform device.Platform) string {
	switch platform {
	case device.PlatformIOS:
		return "platform=iOS Simulator,name=iPhone 15 Pro"
	case device.PlatformMacOS:
		return "platform=macOS"
	case device.PlatformWatchOS:
		return "platform=watchOS Simulator,name=Apple Watch Series 9 (45mm)"
	case device.PlatformTVOS:
		return "platform=tvOS Simulator,name=Apple TV 4K (3rd generation)"
	case device.PlatformVisionOS:
		return "platform=visionOS Simulator,name=Apple Vision Pro"
	default:
		return "platform=iOS Simulator,name=iPhone 15 Pro"
	}
}

// outputParser parses xcodebuild output
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

	// Check for compile
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

	// Check for warning/error diagnostics
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

	// Check for link
	if matches := linkPattern.FindStringSubmatch(line); matches != nil {
		if p.events != nil {
			p.events <- Event{
				Type:    EventLink,
				Message: matches[1],
			}
		}
		return
	}

	// Check for code signing
	if matches := signPattern.FindStringSubmatch(line); matches != nil {
		if p.events != nil {
			p.events <- Event{
				Type:    EventSign,
				Message: matches[1],
			}
		}
		return
	}

	// Check for success/failure
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

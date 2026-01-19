package log

import (
	"context"
	"io"
	"os"
	"time"

	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"
)

type Options struct {
	Context        context.Context
	Options        option.LogOptions
	Observable     bool
	DefaultWriter  io.Writer
	BaseTime       time.Time
	PlatformWriter PlatformWriter
}

func New(options Options) (Factory, error) {
	logOptions := options.Options

	if logOptions.Disabled {
		return NewNOPFactory(), nil
	}

	var outputs []Output

	// Check if using new multi-output configuration or legacy single output
	if len(logOptions.Outputs) > 0 {
		// Multi-output mode
		for i, outputConfig := range logOptions.Outputs {
			output, err := createOutput(outputConfig, options)
			if err != nil {
				return nil, E.Cause(err, "create output ", i)
			}
			outputs = append(outputs, output)
		}
	} else {
		// Legacy single output mode (backward compatibility)
		output, err := createLegacyOutput(logOptions, options)
		if err != nil {
			return nil, E.Cause(err, "create legacy output")
		}
		outputs = []Output{output}
	}

	// Create platform formatter for platform writer
	platformFormatter := Formatter{
		BaseTime:         options.BaseTime,
		DisableLineBreak: true,
	}
	if options.PlatformWriter != nil {
		platformFormatter.DisableColors = options.PlatformWriter.DisableColors()
	}

	// Create multi-output factory
	factory := NewMultiOutputFactory(
		options.Context,
		outputs,
		platformFormatter,
		options.PlatformWriter,
		options.Observable,
	)

	// Set log level
	if logOptions.Level != "" {
		logLevel, err := ParseLevel(logOptions.Level)
		if err != nil {
			return nil, E.Cause(err, "parse log level")
		}
		factory.SetLevel(logLevel)
	} else {
		factory.SetLevel(LevelTrace)
	}

	return factory, nil
}

// createLegacyOutput creates an output from legacy single-output configuration
func createLegacyOutput(logOptions option.LogOptions, options Options) (Output, error) {
	var logWriter io.Writer
	var logFilePath string

	switch logOptions.Output {
	case "":
		logWriter = options.DefaultWriter
		if logWriter == nil {
			logWriter = os.Stderr
		}
	case "stderr":
		logWriter = os.Stderr
	case "stdout":
		logWriter = os.Stdout
	default:
		logFilePath = logOptions.Output
	}

	logFormatter := Formatter{
		BaseTime:         options.BaseTime,
		DisableColors:    logOptions.DisableColor || logFilePath != "",
		DisableTimestamp: !logOptions.Timestamp && logFilePath != "",
		FullTimestamp:    logOptions.Timestamp,
		TimestampFormat:  "-0700 2006-01-02 15:04:05",
	}

	return NewFormattedOutput(logFormatter, logWriter, logFilePath), nil
}

// createOutput creates an output from the new multi-output configuration
func createOutput(config option.LogOutput, options Options) (Output, error) {
	switch config.Type {
	case "stdout":
		return createStdOutput(config, options, os.Stdout)
	case "stderr":
		return createStdOutput(config, options, os.Stderr)
	case "file":
		return createFileOutput(config, options)
	case "http":
		return createHTTPOutput(config, options)
	default:
		return nil, E.New("unknown output type: ", config.Type)
	}
}

// createStdOutput creates a stdout/stderr output
func createStdOutput(config option.LogOutput, options Options, writer io.Writer) (Output, error) {
	if config.Format == "json" {
		return NewJSONOutput(writer, "", config.Hostname, config.Version), nil
	}

	// Default to formatted output
	formatter := Formatter{
		BaseTime:         options.BaseTime,
		DisableColors:    config.DisableColor,
		DisableTimestamp: !config.Timestamp,
		FullTimestamp:    config.Timestamp,
		TimestampFormat:  "-0700 2006-01-02 15:04:05",
	}
	return NewFormattedOutput(formatter, writer, ""), nil
}

// createFileOutput creates a file output
func createFileOutput(config option.LogOutput, options Options) (Output, error) {
	if config.Path == "" {
		return nil, E.New("file output requires path")
	}

	if config.Format == "json" {
		return NewJSONOutput(nil, config.Path, config.Hostname, config.Version), nil
	}

	// Default to formatted output
	formatter := Formatter{
		BaseTime:         options.BaseTime,
		DisableColors:    config.DisableColor || true, // Always disable colors for files
		DisableTimestamp: !config.Timestamp,
		FullTimestamp:    config.Timestamp,
		TimestampFormat:  "-0700 2006-01-02 15:04:05",
	}
	return NewFormattedOutput(formatter, nil, config.Path), nil
}

// createHTTPOutput creates an HTTP batch output
func createHTTPOutput(config option.LogOutput, options Options) (Output, error) {
	return CreateHTTPOutput(config, options.BaseTime)
}

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

	// If multi-output configuration is provided, use multi-output factory
	if len(logOptions.Outputs) > 0 {
		return newMultiOutput(options)
	}

	// Check if JSON format is requested on legacy path
	if logOptions.Format == "json" {
		return newJSONLegacyOutput(options)
	}

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
		logWriter = io.Discard
		logFilePath = logOptions.Output
	}
	logFormatter := Formatter{
		BaseTime:         options.BaseTime,
		DisableColors:    logOptions.DisableColor || logFilePath != "",
		DisableTimestamp: !logOptions.Timestamp && logFilePath != "",
		FullTimestamp:    logOptions.Timestamp,
		TimestampFormat:  "-0700 2006-01-02 15:04:05",
	}
	factory := NewDefaultFactory(
		options.Context,
		logFormatter,
		logWriter,
		logFilePath,
		options.PlatformWriter,
		options.Observable,
	)
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

// newMultiOutput creates a multi-output factory from the provided options
func newMultiOutput(options Options) (Factory, error) {
	logOptions := options.Options

	var outputs []Output
	for i, outputConfig := range logOptions.Outputs {
		output, err := createOutput(outputConfig, options)
		if err != nil {
			return nil, E.Cause(err, "create output ", i)
		}
		outputs = append(outputs, output)
	}

	platformFormatter := Formatter{
		BaseTime:         options.BaseTime,
		DisableLineBreak: true,
	}

	// Create event bus if enabled
	var eventBus *EventBus
	if logOptions.EventBus != nil && logOptions.EventBus.Enabled {
		eventBus = NewEventBus()
	}

	factory := NewMultiOutputFactoryWithBus(
		options.Context,
		outputs,
		platformFormatter,
		options.PlatformWriter,
		options.Observable,
		eventBus,
	)

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

// createOutput creates an Output from a LogOutput config
func createOutput(config option.LogOutput, options Options) (Output, error) {
	switch config.Type {
	case "stdout":
		return createStdOutput(config, options, os.Stdout)
	case "stderr":
		return createStdOutput(config, options, os.Stderr)
	case "file":
		return createFileOutput(config, options)
	case "http":
		return CreateHTTPOutput(config, options.BaseTime)
	default:
		return nil, E.New("unknown output type: ", config.Type)
	}
}

// createStdOutput creates a stdout/stderr output
func createStdOutput(config option.LogOutput, options Options, writer io.Writer) (Output, error) {
	if config.Format == "json" {
		return NewJSONOutput(writer, "", config.Hostname, config.Version), nil
	}

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

	formatter := Formatter{
		BaseTime:         options.BaseTime,
		DisableColors:    true, // Always disable colors for files
		DisableTimestamp: !config.Timestamp,
		FullTimestamp:    config.Timestamp,
		TimestampFormat:  "-0700 2006-01-02 15:04:05",
	}
	return NewFormattedOutput(formatter, nil, config.Path), nil
}

// newJSONLegacyOutput creates a JSON output for the legacy config path
func newJSONLegacyOutput(options Options) (Factory, error) {
	logOptions := options.Options

	var output Output
	var err error

	switch logOptions.Output {
	case "", "stderr":
		output, err = createStdOutput(option.LogOutput{
			Type:      "stderr",
			Format:    "json",
			Timestamp: logOptions.Timestamp,
		}, options, os.Stderr)
	case "stdout":
		output, err = createStdOutput(option.LogOutput{
			Type:      "stdout",
			Format:    "json",
			Timestamp: logOptions.Timestamp,
		}, options, os.Stdout)
	default:
		output, err = createFileOutput(option.LogOutput{
			Type:      "file",
			Path:      logOptions.Output,
			Format:    "json",
			Timestamp: logOptions.Timestamp,
		}, options)
	}

	if err != nil {
		return nil, err
	}

	platformFormatter := Formatter{
		BaseTime:         options.BaseTime,
		DisableLineBreak: true,
	}

	factory := NewMultiOutputFactory(
		options.Context,
		[]Output{output},
		platformFormatter,
		options.PlatformWriter,
		options.Observable,
	)

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

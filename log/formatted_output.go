package log

import (
	"context"
	"io"
	"os"

	"github.com/sagernet/sing/common"
)

var _ Output = (*FormattedOutput)(nil)

// FormattedOutput wraps an io.Writer with a formatter
type FormattedOutput struct {
	formatter Formatter
	writer    io.Writer
	file      *os.File
	filePath  string
}

// NewFormattedOutput creates a new formatted output
func NewFormattedOutput(formatter Formatter, writer io.Writer, filePath string) Output {
	return &FormattedOutput{
		formatter: formatter,
		writer:    writer,
		filePath:  filePath,
	}
}

// Start opens the file if this is a file output
func (o *FormattedOutput) Start() error {
	if o.filePath != "" && o.writer == nil {
		file, err := os.OpenFile(o.filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if err != nil {
			return err
		}
		o.file = file
		o.writer = file
	}
	return nil
}

// Write writes a formatted log entry
func (o *FormattedOutput) Write(entry LogEntry) error {
	if o.writer == nil {
		return nil
	}

	// Build context with connection ID if present
	var ctx context.Context
	if entry.ConnectionID != 0 {
		id := ID{
			ID:        entry.ConnectionID,
			CreatedAt: entry.Timestamp.Add(-entry.ConnectionDuration),
		}
		ctx = ContextWithID(context.Background(), id)
	} else {
		ctx = context.Background()
	}

	// Format the message using existing formatter
	message := o.formatter.Format(ctx, entry.Level, entry.Tag, entry.Message, entry.Timestamp)

	// Write to output
	_, err := o.writer.Write([]byte(message))
	return err
}

// Close flushes and closes the output
func (o *FormattedOutput) Close() error {
	return common.Close(common.PtrOrNil(o.file))
}

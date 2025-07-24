package log_test

import (
	"bytes"
	"os"
	"testing"

	"github.com/lattesec/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoggerLevelFiltering(t *testing.T) {
	var buf bytes.Buffer

	l, err := log.NewLogger().
		WithLevel(log.INFO).
		WithStderr(false).
		WithStdout(false).
		WithHandlers(log.NewWriterHandler(&buf)).
		Build()
	require.NoError(t, err)
	require.NoError(t, l.Start(), "failed to start logger")

	l.Debug().Msg("debug msg").Send() // should be filtered out
	l.Info().Msg("info msg").Send()

	require.NoError(t, l.Close())

	got := buf.String()
	assert.NotContains(t, got, "debug msg", "expected debug message to be filtered out")
	assert.Contains(t, got, "info msg", "expected info message to be logged")
}

func TestMultipleHandlers(t *testing.T) {
	var out1, out2 bytes.Buffer

	l, err := log.NewLogger().
		WithLevel(log.INFO).
		WithStderr(false).
		WithStdout(false).
		WithHandlers(log.NewWriterHandler(&out1), log.NewWriterHandler(&out2)).
		Build()
	require.NoError(t, err)
	require.NoError(t, l.Start(), "failed to start logger")

	l.Info().Msg("log once").Send()

	require.NoError(t, l.Close())
	assert.Contains(t, out1.String(), "log once", "expected log message to be logged to first handler")
	assert.Contains(t, out2.String(), "log once", "expected log message to be logged to second handler")
}

func TestFileHandlerWritesToFile(t *testing.T) {
	tmpfile, err := os.CreateTemp("", "logtest-*.log")
	require.NoError(t, err)
	defer os.Remove(tmpfile.Name())

	require.NoError(t, tmpfile.Close())

	l, err := log.NewLogger().
		WithLevel(log.INFO).
		WithStderr(false).
		WithStdout(false).
		WithFile(tmpfile.Name(), 0).
		Build()
	require.NoError(t, err)
	require.NoError(t, l.Start(), "failed to start logger")

	l.Info().Msg("file handler test").Send()

	require.NoError(t, l.Close())

	data, err := os.ReadFile(tmpfile.Name())
	require.NoError(t, err)
	assert.Contains(t, string(data), "file handler test", "expected log message to be written to file")
}

func TestClosingOneOfManyFileHandlersStillWorks(t *testing.T) {
	tmpfile, err := os.CreateTemp("", "logtest-*.log")
	require.NoError(t, err)
	defer os.Remove(tmpfile.Name())

	require.NoError(t, tmpfile.Close())

	h1, err := log.NewFileHandler(tmpfile.Name())
	require.NoError(t, err)
	h2, err := log.NewFileHandler(tmpfile.Name())
	require.NoError(t, err)

	t.Log("starting handlers")
	require.ErrorIs(t, h1.Start(), log.ErrAlreadyStarted)
	require.ErrorIs(t, h2.Start(), log.ErrAlreadyStarted)

	t.Log("sending logs")
	h1.Handle("test h1", &log.LogMessage{Level: log.INFO, Message: "from h1"})
	h2.Handle("test h2", &log.LogMessage{Level: log.INFO, Message: "from h2"})

	t.Log("closing h1")
	require.NoError(t, h1.Close())

	t.Log("sending logs to h2")
	h2.Handle("test h2", &log.LogMessage{Level: log.INFO, Message: "still from h2"})

	t.Log("closing h2")
	require.NoError(t, h2.Close())

	t.Log("verifying file content")
	data, err := os.ReadFile(tmpfile.Name())
	require.NoError(t, err)
	content := string(data)

	assert.Contains(t, content, "from h1")
	assert.Contains(t, content, "from h2")
	assert.Contains(t, content, "still from h2")
}

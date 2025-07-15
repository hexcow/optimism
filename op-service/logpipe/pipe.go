package logpipe

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"

	oplog "github.com/ethereum-optimism/optimism/op-service/log"
	"github.com/ethereum/go-ethereum/log"
)

// PipeLogs reads logs from the provided io.ReadCloser (e.g., subprocess stdout),
// and outputs them to the provider logger.
//
// This:
// 1. assumes each line is a JSON object
// 2. parses it
// 3. extracts the "level" and optional "msg"
// 4. treats remaining fields as structured attributes
// 5. logs the entries using the provided log.Logger
//
// Non-JSON lines are logged as warnings.
// Crit level is mapped to error-level, to prevent untrusted crit logs from stopping the process.
// This function processes until the stream ends, and closes the reader.
// This returns the first read error (If we run into EOF, nil returned is returned instead).
func PipeLogs(r io.ReadCloser, logger log.Logger) (outErr error) {
	defer func() {
		outErr = errors.Join(outErr, r.Close())
	}()

	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		lineBytes := scanner.Bytes()
		if len(lineBytes) == 0 {
			continue // Skip empty lines
		}
		dec := json.NewDecoder(bytes.NewReader(lineBytes))
		dec.UseNumber() // to preserve number formatting

		var m map[string]any
		if err := dec.Decode(&m); err != nil {
			logger.Warn("Invalid JSON log line", "line", string(lineBytes), "err", err)
			continue
		}

		levelAny, ok := m["level"]
		if !ok {
			logger.Warn("Log line missing 'level' field", "line", string(lineBytes))
			continue
		}
		delete(m, "level")

		msgAny, ok := m["msg"]
		msg := ""
		if ok {
			if s, ok := msgAny.(string); ok {
				msg = s
			} else {
				msg = fmt.Sprint(msgAny)
			}
			delete(m, "msg")
		}

		// Build attributes
		attrs := make([]any, 0, len(m))
		for k, v := range m {
			if x, ok := v.(json.Number); ok {
				v = x.String()
			}
			attrs = append(attrs, slog.Any(k, v))
		}

		// Determine log level
		levelStr, ok := levelAny.(string)
		if !ok {
			logger.Warn("Invalid 'level' type", "value", levelAny, "line", string(lineBytes))
			continue
		}

		lvl, err := oplog.LevelFromString(levelStr)
		if err != nil {
			logger.Warn("Invalid 'level' value, defaulting to INFO now", "value", levelStr)
			lvl = log.LevelInfo
		}
		if lvl >= log.LevelCrit {
			// If a sub-process has a critical error, this process can handle it
			// Don't force an os.Exit, downgrade to error instead
			lvl = log.LevelError
			attrs = append(attrs, slog.String("innerLevel", "CRIT"))
		}
		logger.Log(lvl, msg, attrs...)
	}

	return scanner.Err()
}

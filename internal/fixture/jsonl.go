package fixture

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

const maxFixtureLine = 64 << 20

func readJSONLines(path string, strict bool, visit func(line int, data []byte) error) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64<<10), maxFixtureLine)
	line := 0
	for scanner.Scan() {
		line++
		data := bytes.TrimSuffix(scanner.Bytes(), []byte{'\r'})
		if len(bytes.TrimSpace(data)) == 0 {
			return fmt.Errorf("%s:%d: blank JSON Lines record", path, line)
		}
		if err := visit(line, append([]byte(nil), data...)); err != nil {
			return fmt.Errorf("%s:%d: %w", path, line, err)
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	return nil
}

func decodeJSON(data []byte, dst any, strict bool) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	if strict {
		decoder.DisallowUnknownFields()
	}
	if err := decoder.Decode(dst); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("multiple JSON values")
		}
		return err
	}
	return nil
}

func decodeJSONObject(data []byte, dst any, strict bool) error {
	if len(bytes.TrimSpace(data)) == 0 || bytes.TrimSpace(data)[0] != '{' {
		return errors.New("expected JSON object")
	}
	return decodeJSON(data, dst, strict)
}

func rejectUnknownFields(data []byte, allowed ...string) error {
	seen := make(map[string]struct{}, len(allowed))
	for _, name := range allowed {
		seen[name] = struct{}{}
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(data, &object); err != nil {
		return err
	}
	for name := range object {
		if _, ok := seen[name]; !ok {
			return fmt.Errorf("unknown field %q", name)
		}
	}
	return nil
}

func validateUniqueIDs[T any](path string, records []T, id func(T) string) error {
	seen := make(map[string]int, len(records))
	for i, record := range records {
		value := id(record)
		if value == "" {
			return fmt.Errorf("%s:%d: empty record ID", path, i+1)
		}
		if previous, ok := seen[value]; ok {
			return fmt.Errorf("%s:%d: duplicate record ID %q (first at record %d)", path, i+1, value, previous)
		}
		seen[value] = i + 1
	}
	return nil
}

func parseNumber(raw json.RawMessage, field string) (float64, error) {
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return 0, fmt.Errorf("%s must be a number, not null", field)
	}
	var number float64
	if err := json.Unmarshal(raw, &number); err != nil {
		return 0, fmt.Errorf("%s must be a number: %w", field, err)
	}
	return number, nil
}

func parseInt(raw json.RawMessage, field string) (int, error) {
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return 0, fmt.Errorf("%s must be an integer, not null", field)
	}
	var number int
	if err := json.Unmarshal(raw, &number); err != nil {
		return 0, fmt.Errorf("%s must be an integer: %w", field, err)
	}
	return number, nil
}

func splitNonEmptyLines(data []byte) []string {
	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	text = strings.TrimSuffix(text, "\n")
	if text == "" {
		return nil
	}
	return strings.Split(text, "\n")
}

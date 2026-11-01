package httpsink

import (
	"encoding/json"
	"log/slog"
	"strconv"
	"time"
)

func marshalBatch(format PayloadFormat, batch []slog.Record, static map[string]any) ([]byte, error) {
	buf := make([]byte, 0, len(batch)*256)
	if format == FormatJSONArray {
		buf = append(buf, '[')
	}
	for i, rec := range batch {
		if format == FormatJSONArray && i > 0 {
			buf = append(buf, ',')
		}
		var err error
		buf, err = appendRecord(buf, rec, static)
		if err != nil {
			return nil, err
		}
		if format != FormatJSONArray {
			buf = append(buf, '\n')
		}
	}
	if format == FormatJSONArray {
		buf = append(buf, ']')
	}
	return buf, nil
}

func appendRecord(dst []byte, rec slog.Record, static map[string]any) ([]byte, error) {
	dst = append(dst, '{')
	first := true
	dst = appendStringField(dst, &first, "@timestamp", rec.Time.UTC().Format(time.RFC3339Nano))
	dst = appendStringField(dst, &first, "message", rec.Message)
	dst = appendStringField(dst, &first, "log.level", rec.Level.String())
	for key, value := range static {
		var err error
		dst, err = appendAnyField(dst, &first, key, value)
		if err != nil {
			return nil, err
		}
	}
	var encodeErr error
	rec.Attrs(func(attr slog.Attr) bool {
		dst, encodeErr = appendAttr(dst, &first, attr)
		return encodeErr == nil
	})
	if encodeErr != nil {
		return nil, encodeErr
	}
	return append(dst, '}'), nil
}

func appendAttr(dst []byte, first *bool, attr slog.Attr) ([]byte, error) {
	value := attr.Value.Resolve()
	if attr.Key == "" {
		if value.Kind() != slog.KindGroup {
			return dst, nil
		}
		for _, nested := range value.Group() {
			var err error
			dst, err = appendAttr(dst, first, nested)
			if err != nil {
				return nil, err
			}
		}
		return dst, nil
	}
	dst = appendFieldName(dst, first, attr.Key)
	return appendValue(dst, value)
}

func appendValue(dst []byte, value slog.Value) ([]byte, error) {
	switch value.Kind() {
	case slog.KindString:
		return strconv.AppendQuote(dst, value.String()), nil
	case slog.KindInt64:
		return strconv.AppendInt(dst, value.Int64(), 10), nil
	case slog.KindUint64:
		return strconv.AppendUint(dst, value.Uint64(), 10), nil
	case slog.KindFloat64:
		return strconv.AppendFloat(dst, value.Float64(), 'g', -1, 64), nil
	case slog.KindBool:
		return strconv.AppendBool(dst, value.Bool()), nil
	case slog.KindDuration:
		return strconv.AppendInt(dst, int64(value.Duration()), 10), nil
	case slog.KindTime:
		return strconv.AppendQuote(dst, value.Time().Format(time.RFC3339Nano)), nil
	case slog.KindGroup:
		dst = append(dst, '{')
		first := true
		for _, nested := range value.Group() {
			var err error
			dst, err = appendAttr(dst, &first, nested)
			if err != nil {
				return nil, err
			}
		}
		return append(dst, '}'), nil
	default:
		raw, err := json.Marshal(value.Any())
		return append(dst, raw...), err
	}
}

func appendStringField(dst []byte, first *bool, key, value string) []byte {
	dst = appendFieldName(dst, first, key)
	return strconv.AppendQuote(dst, value)
}

func appendAnyField(dst []byte, first *bool, key string, value any) ([]byte, error) {
	dst = appendFieldName(dst, first, key)
	raw, err := json.Marshal(value)
	return append(dst, raw...), err
}

func appendFieldName(dst []byte, first *bool, key string) []byte {
	if !*first {
		dst = append(dst, ',')
	}
	*first = false
	dst = strconv.AppendQuote(dst, key)
	return append(dst, ':')
}

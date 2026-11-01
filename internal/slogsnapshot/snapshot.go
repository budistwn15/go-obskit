package slogsnapshot

import (
	"encoding/json"
	"log/slog"
	"reflect"
)

// Record snapshots mutable attributes and returns a conservative encoded-size estimate.
func Record(rec slog.Record) (slog.Record, int) {
	size := len(rec.Message)*6 + 96
	mutable := false
	rec.Attrs(func(attr slog.Attr) bool {
		size += attrSize(attr)
		mutable = mutable || needsSnapshot(attr.Value.Resolve())
		return true
	})
	if !mutable {
		return rec.Clone(), size
	}
	out := slog.NewRecord(rec.Time, rec.Level, rec.Message, rec.PC)
	rec.Attrs(func(attr slog.Attr) bool {
		out.AddAttrs(snapshotAttr(attr))
		return true
	})
	return out, size
}

// Value snapshots mutable configuration values owned by the caller.
func Value(value any) any {
	if value == nil {
		return nil
	}
	kind := reflect.TypeOf(value).Kind()
	if kind != reflect.Map && kind != reflect.Slice && kind != reflect.Ptr && kind != reflect.Interface {
		return value
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return "[UNAVAILABLE]"
	}
	return json.RawMessage(raw)
}

func snapshotAttr(attr slog.Attr) slog.Attr {
	value := attr.Value.Resolve()
	if value.Kind() == slog.KindGroup {
		group := value.Group()
		for i := range group {
			group[i] = snapshotAttr(group[i])
		}
		return slog.Group(attr.Key, attrsToAny(group)...)
	}
	if !needsSnapshot(value) {
		return slog.Attr{Key: attr.Key, Value: value}
	}
	if err, ok := value.Any().(error); ok {
		return slog.String(attr.Key, err.Error())
	}
	raw, err := json.Marshal(value.Any())
	if err != nil {
		return slog.String(attr.Key, "[UNAVAILABLE]")
	}
	return slog.Any(attr.Key, json.RawMessage(raw))
}

func attrsToAny(attrs []slog.Attr) []any {
	out := make([]any, len(attrs))
	for i := range attrs {
		out[i] = attrs[i]
	}
	return out
}

func needsSnapshot(value slog.Value) bool {
	if value.Kind() == slog.KindGroup {
		for _, attr := range value.Group() {
			if needsSnapshot(attr.Value.Resolve()) {
				return true
			}
		}
		return false
	}
	if value.Kind() != slog.KindAny || value.Any() == nil {
		return false
	}
	kind := reflect.TypeOf(value.Any()).Kind()
	return kind == reflect.Map || kind == reflect.Slice || kind == reflect.Ptr || kind == reflect.Interface
}

func attrSize(attr slog.Attr) int {
	value := attr.Value.Resolve()
	size := len(attr.Key)*6 + 8
	switch value.Kind() {
	case slog.KindString:
		return size + len(value.String())*6
	case slog.KindGroup:
		for _, nested := range value.Group() {
			size += attrSize(nested)
		}
		return size
	case slog.KindAny:
		raw, err := json.Marshal(value.Any())
		if err == nil {
			return size + len(raw)
		}
	}
	return size + 32
}

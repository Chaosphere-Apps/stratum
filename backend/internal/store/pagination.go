package store

import (
	"context"
	"strconv"
	"strings"
)

const (
	DefaultPageLimit = 25
	MaxPageLimit     = 100
)

type PageOptions struct {
	Query  string
	Cursor string
	Limit  int
}

type PageInfo struct {
	NextCursor string `json:"nextCursor,omitempty"`
	HasMore    bool   `json:"hasMore"`
	Limit      int    `json:"limit"`
}

func NormalizePageOptions(options PageOptions) PageOptions {
	options.Query = strings.TrimSpace(options.Query)
	options.Cursor = strings.TrimSpace(options.Cursor)
	if options.Limit <= 0 {
		options.Limit = DefaultPageLimit
	}
	if options.Limit > MaxPageLimit {
		options.Limit = MaxPageLimit
	}
	return options
}

func OffsetFromCursor(cursor string) int {
	offset, err := strconv.Atoi(strings.TrimSpace(cursor))
	if err != nil || offset < 0 {
		return 0
	}
	return offset
}

func CursorFromOffset(offset int) string {
	if offset <= 0 {
		return ""
	}
	return strconv.Itoa(offset)
}

func PageFromSlice[T any](items []T, options PageOptions) ([]T, PageInfo) {
	options = NormalizePageOptions(options)
	offset := OffsetFromCursor(options.Cursor)
	if offset >= len(items) {
		return []T{}, PageInfo{Limit: options.Limit}
	}
	end := offset + options.Limit
	if end > len(items) {
		end = len(items)
	}
	info := PageInfo{Limit: options.Limit}
	if end < len(items) {
		info.HasMore = true
		info.NextCursor = CursorFromOffset(end)
	}
	return items[offset:end], info
}

func pageFromFetched[T any](items []T, options PageOptions, offset int) ([]T, PageInfo) {
	options = NormalizePageOptions(options)
	info := PageInfo{Limit: options.Limit}
	if len(items) > options.Limit {
		items = items[:options.Limit]
		info.HasMore = true
		info.NextCursor = CursorFromOffset(offset + options.Limit)
	}
	return items, info
}

func listAllPages[T any](ctx context.Context, list func(context.Context, PageOptions) ([]T, PageInfo, error)) ([]T, error) {
	items := []T{}
	cursor := ""
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		page, info, err := list(ctx, PageOptions{Cursor: cursor, Limit: MaxPageLimit})
		if err != nil {
			return nil, err
		}
		items = append(items, page...)
		if !info.HasMore {
			return items, nil
		}
		cursor = info.NextCursor
	}
}

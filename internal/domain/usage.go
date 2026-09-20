package domain

import (
	"errors"
	"fmt"
)

type Usage struct {
	InputTokens      Value[int64] `json:"input_tokens"`
	OutputTokens     Value[int64] `json:"output_tokens"`
	ReasoningTokens  Value[int64] `json:"reasoning_tokens"`
	CacheReadTokens  Value[int64] `json:"cache_read_tokens"`
	CacheWriteTokens Value[int64] `json:"cache_write_tokens"`
}

func (u *Usage) Validate() error {
	if u == nil {
		return nil
	}
	var errs []error
	fields := []struct {
		name string
		v    Value[int64]
	}{
		{"input_tokens", u.InputTokens},
		{"output_tokens", u.OutputTokens},
		{"reasoning_tokens", u.ReasoningTokens},
		{"cache_read_tokens", u.CacheReadTokens},
		{"cache_write_tokens", u.CacheWriteTokens},
	}
	for _, f := range fields {
		if err := f.v.Validate(); err != nil {
			errs = append(errs, fmt.Errorf("usage.%s: %w", f.name, err))
		}
	}
	return errors.Join(errs...)
}

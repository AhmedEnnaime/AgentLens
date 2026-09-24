package domain

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
)

type ContentKind uint8

const (
	ContentKindSQLiteRow ContentKind = iota + 1
)

func (k ContentKind) String() string {
	switch k {
	case ContentKindSQLiteRow:
		return "sqlite-row"
	default:
		return fmt.Sprintf("content_kind(%d)", uint8(k))
	}
}

func (k ContentKind) known() bool {
	switch k {
	case ContentKindSQLiteRow:
		return true
	}
	return false
}

func (k ContentKind) MarshalJSON() ([]byte, error) {
	if !k.known() {
		return nil, fmt.Errorf("cannot marshal unknown content kind %d", uint8(k))
	}
	return json.Marshal(k.String())
}

func (k *ContentKind) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	for v := ContentKindSQLiteRow; v <= ContentKindSQLiteRow; v++ {
		if v.String() == s {
			*k = v
			return nil
		}
	}
	return fmt.Errorf("unknown content kind %q", s)
}

type ContentRef struct {
	Kind   ContentKind `json:"kind"`
	Path   string      `json:"path"`
	Table  string      `json:"table"`
	RowID  string      `json:"row_id"`
	SHA256 string      `json:"sha256"`
	Bytes  int64       `json:"bytes"`
}

var sha256HexRe = regexp.MustCompile(`^[0-9a-f]{64}$`)

func (r ContentRef) Validate() error {
	if !r.Kind.known() {
		return fmt.Errorf("content ref: unknown kind %d", uint8(r.Kind))
	}
	if r.Path == "" {
		return fmt.Errorf("content ref: empty path")
	}
	if r.Table == "" {
		return fmt.Errorf("content ref: empty table")
	}
	if r.RowID == "" {
		return fmt.Errorf("content ref: empty row id")
	}
	if !sha256HexRe.MatchString(r.SHA256) {
		return fmt.Errorf("content ref: sha256 must be 64 lowercase hex characters")
	}
	return nil
}

func AsContentRef(v any) (ContentRef, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return ContentRef{}, fmt.Errorf("content ref: marshal value: %w", err)
	}
	var ref ContentRef
	if err := json.Unmarshal(data, &ref); err != nil {
		return ContentRef{}, fmt.Errorf("content ref: decode: %w", err)
	}
	if err := ref.Validate(); err != nil {
		return ContentRef{}, err
	}
	return ref, nil
}

type ContentStatus uint8

const (
	ContentResolved ContentStatus = iota + 1
	ContentUnavailable
	ContentChanged
)

func (s ContentStatus) String() string {
	switch s {
	case ContentResolved:
		return "resolved"
	case ContentUnavailable:
		return "unavailable"
	case ContentChanged:
		return "changed"
	default:
		return fmt.Sprintf("content_status(%d)", uint8(s))
	}
}

func (s ContentStatus) known() bool {
	switch s {
	case ContentResolved, ContentUnavailable, ContentChanged:
		return true
	}
	return false
}

func (s ContentStatus) MarshalJSON() ([]byte, error) {
	if !s.known() {
		return nil, fmt.Errorf("cannot marshal unknown content status %d", uint8(s))
	}
	return json.Marshal(s.String())
}

func (s *ContentStatus) UnmarshalJSON(data []byte) error {
	var str string
	if err := json.Unmarshal(data, &str); err != nil {
		return err
	}
	for v := ContentResolved; v <= ContentChanged; v++ {
		if v.String() == str {
			*s = v
			return nil
		}
	}
	return fmt.Errorf("unknown content status %q", str)
}

type ResolvedContent struct {
	Status ContentStatus `json:"status"`
	Text   string        `json:"text,omitempty"`
}

type ContentResolver interface {
	ResolveContent(ctx context.Context, ref ContentRef) (ResolvedContent, error)
}

func ContentSHA256(raw json.RawMessage) string {
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

type ContentResolverContract struct {
	Resolved    ContentRef
	Unavailable ContentRef
	Changed     ContentRef
	Failing     ContentRef
}

func RunContentResolverContract(resolver ContentResolver, c ContentResolverContract) error {
	var errs []error
	errs = append(errs, contractResolved(resolver, c.Resolved))
	errs = append(errs, contractUnavailable(resolver, c.Unavailable))
	errs = append(errs, contractChanged(resolver, c.Changed))
	errs = append(errs, contractOperational(resolver, c.Failing))
	errs = append(errs, contractInvalidRef(resolver))
	return errors.Join(errs...)
}

func contractResolved(resolver ContentResolver, ref ContentRef) error {
	got, err := resolver.ResolveContent(context.Background(), ref)
	if err != nil {
		return fmt.Errorf("resolved: operational error: %w", err)
	}
	if got.Status != ContentResolved {
		return fmt.Errorf("resolved: status %s, want resolved", got.Status)
	}
	if got.Text == "" {
		return fmt.Errorf("resolved: empty text")
	}
	return nil
}

func contractUnavailable(resolver ContentResolver, ref ContentRef) error {
	got, err := resolver.ResolveContent(context.Background(), ref)
	if err != nil {
		return fmt.Errorf("unavailable: operational error: %w", err)
	}
	if got.Status != ContentUnavailable {
		return fmt.Errorf("unavailable: status %s, want unavailable", got.Status)
	}
	if got.Text != "" {
		return fmt.Errorf("unavailable: non-empty text %q", got.Text)
	}
	return nil
}

func contractChanged(resolver ContentResolver, ref ContentRef) error {
	got, err := resolver.ResolveContent(context.Background(), ref)
	if err != nil {
		return fmt.Errorf("changed: operational error: %w", err)
	}
	if got.Status != ContentChanged {
		return fmt.Errorf("changed: status %s, want changed", got.Status)
	}
	if got.Text != "" {
		return fmt.Errorf("changed: non-empty text %q", got.Text)
	}
	return nil
}

func contractOperational(resolver ContentResolver, ref ContentRef) error {
	_, err := resolver.ResolveContent(context.Background(), ref)
	if err == nil {
		return fmt.Errorf("operational: expected error, got none")
	}
	return nil
}

func contractInvalidRef(resolver ContentResolver) error {
	got, err := resolver.ResolveContent(context.Background(), ContentRef{})
	if err != nil {
		return fmt.Errorf("invalid ref: operational error: %w", err)
	}
	if got.Status == ContentResolved {
		return fmt.Errorf("invalid ref: must not resolve")
	}
	if got.Text != "" {
		return fmt.Errorf("invalid ref: non-empty text %q", got.Text)
	}
	return nil
}

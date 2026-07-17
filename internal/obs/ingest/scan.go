// Package ingest parses local agent transcripts into model records.
//
// Parsing semantics (incremental byte-offset resume, per-line tolerance) are
// derived from ai-observer (MIT) — see NOTICE.
package ingest

import (
	"bufio"
	"io"
)

// maxLineBytes bounds one JSONL record. Real transcripts run to a few MB per line
// when a tool returns a large payload; bufio's 64KB default would reject them.
const maxLineBytes = 16 << 20

// ScanState is what a parser carries across resumes of the same growing file.
type ScanState struct {
	Offset       int64          `json:"offset"`
	UnknownTypes map[string]int `json:"unknowntypes,omitempty"`
	Seen         map[string]int `json:"seen,omitempty"`
}

// NoteUnknown records a record type the parser does not model. Drift shows up as
// a count rather than as silently missing data.
func (s *ScanState) NoteUnknown(kind string) {
	if s.UnknownTypes == nil {
		s.UnknownTypes = map[string]int{}
	}
	s.UnknownTypes[kind]++
}

// FirstSeen reports whether key is new, and marks it. Usage is deduped on
// message.id: one JSONL record per content block repeats the same usage object.
func (s *ScanState) FirstSeen(key string) bool {
	if key == "" {
		return true
	}
	if s.Seen == nil {
		s.Seen = map[string]int{}
	}
	if _, ok := s.Seen[key]; ok {
		return false
	}
	s.Seen[key] = 1
	return true
}

// ScanLines feeds each complete line from r to fn, returning the offset of the
// last byte committed. A trailing line without "\n" is left uncommitted: the file
// is being appended to right now, and half a JSON record is not a record.
// The second return is the count of oversized lines skipped, so the caller can
// record drift rather than lose them silently — the module's contract is that
// unhandled data shows up as a number, never as a gap.
//
// Memory is bounded: a line is buffered only up to maxLineBytes, then the rest is
// discarded to the next newline. A corrupt file with a multi-GB run before a
// newline is skipped, not OOM'd — ReadString would have grown an unbounded string.
func ScanLines(r io.Reader, start int64, fn func(line []byte) error) (int64, int, error) {
	br := bufio.NewReaderSize(r, 64<<10)
	offset := start
	oversized := 0
	var buf []byte
	for {
		chunk, err := br.ReadSlice('\n')
		complete := err == nil // a full line terminated by '\n'
		if err == bufio.ErrBufferFull {
			err = nil // partial: more of this line follows
		}
		if len(buf) < maxLineBytes {
			room := maxLineBytes - len(buf)
			if len(chunk) < room {
				room = len(chunk)
			}
			buf = append(buf, chunk[:room]...)
		}
		lineLen := len(chunk)
		if !complete {
			// keep reading the same line; only measure/commit once terminated
			if err == io.EOF {
				return offset, oversized, nil // trailing partial line, not committed
			}
			if err != nil {
				return offset, oversized, err
			}
			// track full on-disk length so the offset stays byte-accurate
			offset += int64(lineLen)
			continue
		}

		if len(buf) >= maxLineBytes {
			oversized++
		} else {
			if cbErr := fn(dropCR(buf)); cbErr != nil {
				return offset, oversized, cbErr
			}
		}
		offset += int64(lineLen)
		buf = buf[:0]

		if err == io.EOF {
			return offset, oversized, nil
		}
		if err != nil {
			return offset, oversized, err
		}
	}
}

func dropCR(b []byte) []byte {
	b = trimTrailing(b, '\n')
	b = trimTrailing(b, '\r')
	return b
}

func trimTrailing(b []byte, c byte) []byte {
	if len(b) > 0 && b[len(b)-1] == c {
		return b[:len(b)-1]
	}
	return b
}

/**
 * Component: Notes ID Generator
 * Block-UUID: e5f6a7b8-c9d0-1234-efab-234567890123
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Generates note_<uuid-v7> style identifiers for notes using timestamp ordering and UUID randomness.
 * Language: Go
 * Created-at: 2026-06-21T00:00:00Z
 * Authors: MiMo-v2.5-pro (v1.0.0)
 */


package notes

import (
	"crypto/rand"
	"fmt"
	"time"

	"github.com/google/uuid"
)

func NewNoteID(now time.Time) (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}

	ms := uint64(now.UnixMilli())
	b[0] = byte(ms >> 40)
	b[1] = byte(ms >> 32)
	b[2] = byte(ms >> 24)
	b[3] = byte(ms >> 16)
	b[4] = byte(ms >> 8)
	b[5] = byte(ms)
	b[6] = (b[6] & 0x0f) | 0x70
	b[8] = (b[8] & 0x3f) | 0x80

	return fmt.Sprintf("note_%s", uuid.UUID(b).String()), nil
}

/**
 * Component: Lessons ID Generator
 * Block-UUID: 14843360-a2c9-40b9-afc6-e40d73cf44f9
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Generates lsn_<uuid-v7> style identifiers for committed lessons using timestamp ordering and UUID randomness.
 * Language: Go
 * Created-at: 2026-06-12T12:44:13Z
 * Authors: Codex GPT-5 (v1.0.0)
 */


package lessons

import (
	"crypto/rand"
	"fmt"
	"time"

	"github.com/google/uuid"
)

func NewLessonID(now time.Time) (string, error) {
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

	return fmt.Sprintf("lsn_%s", uuid.UUID(b).String()), nil
}

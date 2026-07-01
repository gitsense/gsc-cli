/**
 * Component: Rules ID Generator
 * Block-UUID: c3d4e5f6-a7b8-9012-cdef-123456789012
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Generates rule_<uuid-v7> style identifiers for rules using timestamp ordering and UUID randomness.
 * Language: Go
 * Created-at: 2026-06-20T19:00:00Z
 * Authors: MiMo-v2.5-pro (v1.0.0)
 */


package rules

import (
	"crypto/rand"
	"fmt"
	"time"

	"github.com/google/uuid"
)

func NewRuleID(now time.Time) (string, error) {
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

	return fmt.Sprintf("rule_%s", uuid.UUID(b).String()), nil
}

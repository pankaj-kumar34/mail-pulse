package main

import (
	"fmt"
	"strconv"
	"testing"
)

// benchStore builds a synthetic store roughly the size of a 71MB Postfix log
// (~600K lines).
func benchStore(tb testing.TB) *logStore {
	tb.Helper()
	s := newLogStore()
	statuses := []string{"sent", "deferred", "bounced", "active", "removed"}
	for i := 0; i < 600_000; i++ {
		s.add(&ParsedLogEntry{
			ID:        strconv.Itoa(i),
			Service:   "smtp",
			Timestamp: int64(1700000000000 + i*1000),
			QueueID:   fmt.Sprintf("Q%08X", i),
			From:      "sender@ibm.com",
			To:        fmt.Sprintf("rcpt%d@bigcorp.com", i%5000),
			Status:    statuses[i%5],
			IsError:   i%100 == 0,
			IsWarning: i%50 == 0,
			Delay:     float64(i%300) / 10,
		})
	}
	return s
}

// BenchmarkGetMessagesPageFlip: after the first load, flipping pages must not
// rescan the store — rows are cached per (since, search, status, sort) key.
func BenchmarkGetMessagesPageFlip(b *testing.B) {
	s := benchStore(b)
	s.getMessages(0, 1, 50, "", "", "time", "desc") // warm the row cache
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.getMessages(0, 1+(i%5), 50, "", "", "time", "desc")
	}
}

// BenchmarkGetMessagesCold: the cost of one full filtered+sorted pass.
func BenchmarkGetMessagesCold(b *testing.B) {
	s := benchStore(b)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.getMessages(0, 1, 50, "rcpt42", "", "time", "desc")
	}
}

package main

import (
	"math"
	"regexp"
	"sort"
	"strings"
	"time"
)

// ──────────────────────────────────────────────
// Query methods on logStore
// All accept since=0 to mean "all time".
// ──────────────────────────────────────────────

// getMetrics computes delivery metrics for entries with timestamp >= since.
func (s *logStore) getMetrics(since int64) Metrics {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var (
		totalSent, totalDeferred, totalBounced, totalRemoved int
		errorCount, warningCount                             int
		delaySum, delayCount                                 float64
		senderDomains    = make(map[string]int)
		recipientDomains = make(map[string]int)
		qmgrActive       = make(map[int64]int)
		qmgrRemoved      = make(map[int64]int)
	)

	for i := range s.logs {
		e := &s.logs[i]
		if since > 0 && e.Timestamp < since { continue }
		if s.isNDR(e) { continue }

		min1 := e.Timestamp / 60000
		switch e.Status {
		case "sent":     totalSent++
		case "deferred": totalDeferred++
		case "bounced":  totalBounced++
		case "removed":  totalRemoved++; qmgrRemoved[min1]++
		case "active":   qmgrActive[min1]++
		}
		if e.IsError   { errorCount++ }
		if e.IsWarning { warningCount++ }
		if e.From != "" { senderDomains[extractDomain(e.From)]++ }
		if e.To   != "" { recipientDomains[extractDomain(e.To)]++ }
		// Only smtp lines have delays breakdown — qmgr active lines skew averages
		if e.Delay > 0 && e.Delays != nil {
			delayCount++
			delaySum += e.Delay
		}
	}

	avgDelay := 0.0
	if delayCount > 0 { avgDelay = delaySum / delayCount }
	total := totalSent + totalDeferred + totalBounced
	successRate := 0.0
	if total > 0 { successRate = float64(totalSent) / float64(total) * 100 }

	return Metrics{
		TotalSent: totalSent, TotalDeferred: totalDeferred,
		TotalBounced: totalBounced, TotalRemoved: totalRemoved,
		ErrorCount: errorCount, WarningCount: warningCount,
		AverageDelay: avgDelay, DeliverySuccessRate: successRate,
		TopSenderDomains:    topN(senderDomains, 20),
		TopRecipientDomains: topN(recipientDomains, 20),
		QmgrActiveHistory:   sortedCountSeries(qmgrActive),
		QmgrRemovedHistory:  sortedCountSeries(qmgrRemoved),
	}
}

// getVolumeInsights computes hourly volume, defer/bounce reasons and failing domains.
func (s *logStore) getVolumeInsights(since int64) VolumeInsights {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sentByHour     := make(map[int64]int)
	deferredByHour := make(map[int64]int)
	bouncedByHour  := make(map[int64]int)
	deferReasons   := make(map[string]int)
	bounceReasons  := make(map[string]int)
	domainSent     := make(map[string]int)
	domainDeferred := make(map[string]int)
	domainBounced  := make(map[string]int)

	for i := range s.logs {
		e := &s.logs[i]
		if since > 0 && e.Timestamp < since { continue }
		if s.isNDR(e) { continue }

		hour1 := e.Timestamp / 3600000
		recip := extractDomain(e.To)
		switch e.Status {
		case "sent":
			sentByHour[hour1]++
			if recip != "" { domainSent[recip]++ }
		case "deferred":
			deferredByHour[hour1]++
			if recip != "" { domainDeferred[recip]++ }
			if r := extractDeferReason(e.RawLog); r != "" { deferReasons[r]++ }
		case "bounced":
			bouncedByHour[hour1]++
			if recip != "" { domainBounced[recip]++ }
			if r := extractBounceReason(e.RawLog); r != "" { bounceReasons[r]++ }
		}
	}

	// Hourly volume series
	hourSet := make(map[int64]struct{})
	for h := range sentByHour     { hourSet[h] = struct{}{} }
	for h := range deferredByHour { hourSet[h] = struct{}{} }
	for h := range bouncedByHour  { hourSet[h] = struct{}{} }
	hours := make([]int64, 0, len(hourSet))
	for h := range hourSet { hours = append(hours, h) }
	sort.Slice(hours, func(i, j int) bool { return hours[i] < hours[j] })
	hourly := make([]HourlyVolume, 0, len(hours))
	for _, h := range hours {
		hourly = append(hourly, HourlyVolume{
			Hour:     h * 3600,
			Sent:     sentByHour[h],
			Deferred: deferredByHour[h],
			Bounced:  bouncedByHour[h],
		})
	}

	// Reasons helper
	type rc struct{ r string; c int }
	toDeferReasons := func(m map[string]int) []DeferReason {
		total := 0
		for _, c := range m { total += c }
		pairs := make([]rc, 0, len(m))
		for r, c := range m { pairs = append(pairs, rc{r, c}) }
		sort.Slice(pairs, func(i, j int) bool { return pairs[i].c > pairs[j].c })
		out := make([]DeferReason, 0, len(pairs))
		for _, p := range pairs {
			pct := 0.0
			if total > 0 { pct = float64(p.c) / float64(total) * 100 }
			out = append(out, DeferReason{Reason: p.r, Count: p.c, Percent: pct})
		}
		return out
	}

	// Failing domains (top 20 by fail rate, min 5 messages)
	domSet := make(map[string]struct{})
	for d := range domainSent     { domSet[d] = struct{}{} }
	for d := range domainDeferred { domSet[d] = struct{}{} }
	for d := range domainBounced  { domSet[d] = struct{}{} }
	deliveries := make([]DomainDelivery, 0, len(domSet))
	for d := range domSet {
		sent, deferred, bounced := domainSent[d], domainDeferred[d], domainBounced[d]
		total := sent + deferred + bounced
		if total < 5 { continue }
		failRate := float64(deferred+bounced) / float64(total) * 100
		deliveries = append(deliveries, DomainDelivery{
			Domain: d, Sent: sent, Deferred: deferred,
			Bounced: bounced, Total: total, FailRate: failRate,
		})
	}
	sort.Slice(deliveries, func(i, j int) bool { return deliveries[i].FailRate > deliveries[j].FailRate })
	if len(deliveries) > 20 { deliveries = deliveries[:20] }

	return VolumeInsights{
		Hourly:         hourly,
		DeferReasons:   toDeferReasons(deferReasons),
		BounceReasons:  toDeferReasons(bounceReasons),
		FailingDomains: deliveries,
	}
}

// getDelayInsights computes delay statistics for entries with timestamp >= since.
func (s *logStore) getDelayInsights(since int64) DelayInsights {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var (
		delayCount                                     int
		delaySumB, delaySumIQ, delaySumCS, delaySumTX float64
		maxDelay                                       float64
		minDelay                                       = math.MaxFloat64
		distBuckets                                    [7]int
		delayByMin = make(map[int64][]float64)
		bqByMin    = make(map[int64][]float64)
		iqByMin    = make(map[int64][]float64)
		cssByMin   = make(map[int64][]float64)
		txByMin    = make(map[int64][]float64)
	)

	for i := range s.logs {
		e := &s.logs[i]
		if since > 0 && e.Timestamp < since { continue }
		if s.isNDR(e) { continue }
		if e.Delay <= 0 || e.Delays == nil { continue }

		d := e.Delay
		delayCount++
		if d > maxDelay { maxDelay = d }
		if d < minDelay { minDelay = d }
		delaySumB  += e.Delays.BeforeQmgr
		delaySumIQ += e.Delays.InQmgr
		delaySumCS += e.Delays.ConnSetup
		delaySumTX += e.Delays.Transmission
		switch {
		case d < 0.1: distBuckets[0]++
		case d < 0.5: distBuckets[1]++
		case d < 1:   distBuckets[2]++
		case d < 2:   distBuckets[3]++
		case d < 5:   distBuckets[4]++
		case d < 10:  distBuckets[5]++
		default:      distBuckets[6]++
		}
		min1 := e.Timestamp / 60000
		delayByMin[min1] = append(delayByMin[min1], d)
		bqByMin[min1]    = append(bqByMin[min1],    e.Delays.BeforeQmgr)
		iqByMin[min1]    = append(iqByMin[min1],    e.Delays.InQmgr)
		cssByMin[min1]   = append(cssByMin[min1],   e.Delays.ConnSetup)
		txByMin[min1]    = append(txByMin[min1],    e.Delays.Transmission)
	}

	n := float64(delayCount)
	avgBefore, avgInQ, avgConn, avgTx, avgTotal := 0.0, 0.0, 0.0, 0.0, 0.0
	if n > 0 {
		avgBefore = delaySumB / n;  avgInQ = delaySumIQ / n
		avgConn   = delaySumCS / n; avgTx  = delaySumTX / n
		avgTotal  = (delaySumB + delaySumIQ + delaySumCS + delaySumTX) / n
	}
	if minDelay == math.MaxFloat64 { minDelay = 0 }

	labels := []string{"<0.1s", "0.1-0.5s", "0.5-1s", "1-2s", "2-5s", "5-10s", "≥10s"}
	dist := make([]DelayDistribution, 7)
	distTotal := 0
	for _, c := range distBuckets { distTotal += c }
	for i, c := range distBuckets {
		pct := 0.0
		if distTotal > 0 { pct = float64(c) / float64(distTotal) * 100 }
		dist[i] = DelayDistribution{Range: labels[i], Count: c, Percentage: pct}
	}

	minMaxSlice := func(m map[int64][]float64) (float64, float64) {
		mn, mx := math.MaxFloat64, 0.0
		for _, vs := range m {
			for _, v := range vs {
				if v < mn { mn = v }
				if v > mx { mx = v }
			}
		}
		if mn == math.MaxFloat64 { mn = 0 }
		return mn, mx
	}
	minBQ, maxBQ := minMaxSlice(bqByMin)
	minIQ, maxIQ := minMaxSlice(iqByMin)
	minCS, maxCS := minMaxSlice(cssByMin)
	minTX, maxTX := minMaxSlice(txByMin)

	return DelayInsights{
		AverageBeforeQmgr: avgBefore, AverageInQmgr: avgInQ,
		AverageConnSetup: avgConn, AverageTransmission: avgTx, AverageDelay: avgTotal,
		MaxDelay: maxDelay, MinDelay: minDelay,
		MinBeforeQmgr: minBQ, MaxBeforeQmgr: maxBQ,
		MinInQmgr: minIQ, MaxInQmgr: maxIQ,
		MinConnSetup: minCS, MaxConnSetup: maxCS,
		MinTransmission: minTX, MaxTransmission: maxTX,
		DelayDistribution: dist,
		DelayTrend:        sortedTimeSeries(delayByMin),
		BeforeQmgrTrend:   sortedTimeSeries(bqByMin),
		InQmgrTrend:       sortedTimeSeries(iqByMin),
		ConnSetupTrend:    sortedTimeSeries(cssByMin),
		TransmissionTrend: sortedTimeSeries(txByMin),
	}
}

// getMessages returns grouped delivery rows (one per queue ID + recipient).
func (s *logStore) getMessages(page, limit int, search, statusFilter, sortBy, sortOrder string) PaginatedMessages {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// First pass: collect from+firstSeen for every queue ID
	type qmeta struct {
		from      string
		firstSeen int64
	}
	qmap := make(map[string]*qmeta, 1024)
	for i := range s.logs {
		e := &s.logs[i]
		if e.QueueID == "" { continue }
		if _, ok := qmap[e.QueueID]; !ok {
			qmap[e.QueueID] = &qmeta{}
		}
		m := qmap[e.QueueID]
		if e.From != "" && m.from == "" {
			m.from = e.From
		}
		if e.Status == "active" && (m.firstSeen == 0 || e.Timestamp < m.firstSeen) {
			m.firstSeen = e.Timestamp
		}
	}

	// Second pass: one row per smtp delivery line
	var rows []MessageRow
	searchLow := strings.ToLower(search)
	for i := range s.logs {
		e := &s.logs[i]
		if e.QueueID == "" || e.To == "" { continue }
		if e.Status == "" || e.Status == "active" || e.Status == "removed" { continue }
		if s.isNDR(e) { continue }

		m := qmap[e.QueueID]
		from := ""
		if m != nil { from = m.from }
		if from == "<>" { continue }

		processTime := 0.0
		if m != nil && m.firstSeen > 0 && e.Timestamp > m.firstSeen {
			processTime = float64(e.Timestamp-m.firstSeen) / 1000.0
		} else if e.Delay > 0 {
			processTime = e.Delay
		}

		row := MessageRow{
			QueueID: e.QueueID, Time: e.Timestamp,
			From: from, To: e.To,
			Status: e.Status, Delay: e.Delay, ProcessTime: processTime,
		}

		if statusFilter != "" && row.Status != statusFilter { continue }
		if searchLow != "" {
			if !strings.Contains(strings.ToLower(row.QueueID+" "+row.From+" "+row.To), searchLow) { continue }
		}
		rows = append(rows, row)
	}

	sort.SliceStable(rows, func(i, j int) bool {
		a, b := rows[i], rows[j]
		var less bool
		switch sortBy {
		case "queueId":     less = a.QueueID < b.QueueID
		case "status":      less = a.Status < b.Status
		case "from":        less = a.From < b.From
		case "to":          less = a.To < b.To
		case "delay":       less = a.Delay < b.Delay
		case "processTime": less = a.ProcessTime < b.ProcessTime
		default:            less = a.Time < b.Time
		}
		if sortOrder == "asc" { return less }
		return !less
	})

	total := len(rows)
	totalPages := (total + limit - 1) / limit
	if totalPages < 1 { totalPages = 1 }
	if page < 1 { page = 1 }
	if page > totalPages { page = totalPages }
	start := (page - 1) * limit
	end := start + limit
	if end > total { end = total }

	return PaginatedMessages{
		Rows: rows[start:end],
		Pagination: PaginationInfo{
			Page: page, Limit: limit, Total: total, TotalPages: totalPages,
			HasNext: page < totalPages, HasPrev: page > 1,
		},
	}
}

// getPaginated returns all log entries with optional filtering and sorting.
func (s *logStore) getPaginated(page, limit int, search, status, sortBy, sortOrder string) PaginatedLogs {
	s.mu.RLock()
	defer s.mu.RUnlock()

	searchLower := strings.ToLower(search)
	filtered := make([]ParsedLogEntry, 0, len(s.logs))
	for _, e := range s.logs {
		if status != "" && e.Status != status { continue }
		if searchLower != "" {
			if !strings.Contains(strings.ToLower(e.QueueID), searchLower) &&
				!strings.Contains(strings.ToLower(e.From), searchLower) &&
				!strings.Contains(strings.ToLower(e.To), searchLower) &&
				!strings.Contains(strings.ToLower(e.Status), searchLower) {
				continue
			}
		}
		filtered = append(filtered, e)
	}

	sort.SliceStable(filtered, func(i, j int) bool {
		var less bool
		switch sortBy {
		case "status": less = filtered[i].Status < filtered[j].Status
		case "from":   less = filtered[i].From < filtered[j].From
		case "to":     less = filtered[i].To < filtered[j].To
		case "delay":  less = filtered[i].Delay < filtered[j].Delay
		default:       less = filtered[i].Timestamp < filtered[j].Timestamp
		}
		if sortOrder == "desc" { return !less }
		return less
	})

	total := len(filtered)
	totalPages := (total + limit - 1) / limit
	if totalPages == 0 { totalPages = 1 }
	if page < 1 { page = 1 }
	if page > totalPages { page = totalPages }
	start := (page - 1) * limit
	end := start + limit
	if end > total { end = total }

	return PaginatedLogs{
		Logs: filtered[start:end],
		Pagination: PaginationInfo{
			Page: page, Limit: limit, Total: total, TotalPages: totalPages,
			HasNext: page < totalPages, HasPrev: page > 1,
		},
	}
}

// getErrorsPaginated returns error/warning entries with optional type filter.
func (s *logStore) getErrorsPaginated(page, limit int, errType, sortOrder string) PaginatedLogs {
	s.mu.RLock()
	defer s.mu.RUnlock()

	filtered := make([]ParsedLogEntry, 0)
	for _, e := range s.logs {
		if !e.IsError && !e.IsWarning { continue }
		switch errType {
		case "errors":
			if !e.IsError { continue }
		case "warnings":
			if !e.IsWarning { continue }
		}
		filtered = append(filtered, e)
	}

	sort.SliceStable(filtered, func(i, j int) bool {
		if sortOrder == "asc" { return filtered[i].Timestamp < filtered[j].Timestamp }
		return filtered[i].Timestamp > filtered[j].Timestamp
	})

	total := len(filtered)
	totalPages := (total + limit - 1) / limit
	if totalPages == 0 { totalPages = 1 }
	if page < 1 { page = 1 }
	if page > totalPages { page = totalPages }
	start := (page - 1) * limit
	end := start + limit
	if end > total { end = total }

	return PaginatedLogs{
		Logs: filtered[start:end],
		Pagination: PaginationInfo{
			Page: page, Limit: limit, Total: total, TotalPages: totalPages,
			HasNext: page < totalPages, HasPrev: page > 1,
		},
	}
}

// getRecipientDrilldown returns per-address delivery stats for a given domain.
// since=0 means all time.
func (s *logStore) getRecipientDrilldown(domain string, since int64) RecipientDrilldown {
	s.mu.RLock()
	defer s.mu.RUnlock()

	type stat struct {
		sent, deferred, bounced int
		lastSeen                int64
		lastStatus              string
		lastReason              string
	}
	stats := make(map[string]*stat)

	for i := range s.logs {
		e := &s.logs[i]
		if since > 0 && e.Timestamp < since { continue }
		if s.isNDR(e) { continue }
		if e.To == "" { continue }
		if e.Status == "" || e.Status == "active" || e.Status == "removed" { continue }
		if extractDomain(e.To) != domain { continue }

		st, ok := stats[e.To]
		if !ok {
			st = &stat{}
			stats[e.To] = st
		}
		switch e.Status {
		case "sent":     st.sent++
		case "deferred": st.deferred++
		case "bounced":  st.bounced++
		}
		if e.Timestamp > st.lastSeen {
			st.lastSeen   = e.Timestamp
			st.lastStatus = e.Status
			switch e.Status {
			case "deferred":
				st.lastReason = extractDeferReason(e.RawLog)
			case "bounced":
				st.lastReason = extractBounceReason(e.RawLog)
			default:
				st.lastReason = ""
			}
		}
	}

	recipients := make([]RecipientStat, 0, len(stats))
	var domSent, domDeferred, domBounced int
	for addr, st := range stats {
		total := st.sent + st.deferred + st.bounced
		failRate := 0.0
		if total > 0 { failRate = float64(st.deferred+st.bounced) / float64(total) * 100 }
		recipients = append(recipients, RecipientStat{
			Address:    addr,
			Domain:     domain,
			Sent:       st.sent,
			Deferred:   st.deferred,
			Bounced:    st.bounced,
			Total:      total,
			FailRate:   failRate,
			LastSeen:   st.lastSeen,
			LastStatus: st.lastStatus,
			LastReason: st.lastReason,
		})
		domSent     += st.sent
		domDeferred += st.deferred
		domBounced  += st.bounced
	}

	sort.Slice(recipients, func(i, j int) bool {
		fi := recipients[i].Deferred + recipients[i].Bounced
		fj := recipients[j].Deferred + recipients[j].Bounced
		if fi != fj { return fi > fj }
		return recipients[i].Total > recipients[j].Total
	})

	domTotal := domSent + domDeferred + domBounced
	domFailRate := 0.0
	if domTotal > 0 { domFailRate = float64(domDeferred+domBounced) / float64(domTotal) * 100 }

	return RecipientDrilldown{
		Domain:     domain,
		Recipients: recipients,
		Sent:       domSent,
		Deferred:   domDeferred,
		Bounced:    domBounced,
		Total:      domTotal,
		FailRate:   domFailRate,
	}
}

// getQueueAgeDistribution builds a histogram of delivery delays (queue age)
// for all smtp delivery lines with timestamp >= since.
func (s *logStore) getQueueAgeDistribution(since int64) QueueAgeDistribution {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Buckets: <10s, 10-30s, 30s-2m, 2-10m, 10-30m, 30m-1h, 1-6h, 6-24h, >24h
	type bucket struct {
		label    string
		min, max float64 // seconds; max=-1 means unbounded
	}
	defs := []bucket{
		{"< 10s",   0,       10},
		{"10–30s",  10,      30},
		{"30s–2m",  30,      120},
		{"2–10m",   120,     600},
		{"10–30m",  600,     1800},
		{"30m–1h",  1800,    3600},
		{"1–6h",    3600,    21600},
		{"6–24h",   21600,   86400},
		{"> 24h",   86400,   -1},
	}
	counts := make([]int, len(defs))
	var delays []float64
	var maxD float64

	for i := range s.logs {
		e := &s.logs[i]
		if since > 0 && e.Timestamp < since { continue }
		if s.isNDR(e) { continue }
		// Only smtp lines with a final status and a delay value
		if e.Delay <= 0 { continue }
		if e.Status != "sent" && e.Status != "deferred" && e.Status != "bounced" { continue }

		d := e.Delay
		delays = append(delays, d)
		if d > maxD { maxD = d }
		for j, b := range defs {
			if d >= b.min && (b.max < 0 || d < b.max) {
				counts[j]++
				break
			}
		}
	}

	total := len(delays)
	buckets := make([]QueueAgeBucket, len(defs))
	for i, b := range defs {
		pct := 0.0
		if total > 0 { pct = float64(counts[i]) / float64(total) * 100 }
		buckets[i] = QueueAgeBucket{Label: b.label, Min: b.min, Max: b.max, Count: counts[i], Percent: pct}
	}

	// Percentiles — sort a copy
	median, p95 := 0.0, 0.0
	if total > 0 {
		sorted := make([]float64, len(delays))
		copy(sorted, delays)
		sort.Float64s(sorted)
		median = sorted[total/2]
		p95    = sorted[int(float64(total)*0.95)]
	}

	return QueueAgeDistribution{
		Buckets:    buckets,
		Total:      total,
		MedianSecs: median,
		P95Secs:    p95,
		MaxSecs:    maxD,
	}
}

// getRelayInsights returns per-relay delivery stats categorized as throttling/blocking/healthy.
func (s *logStore) getRelayInsights(since int64) RelayInsights {
	s.mu.RLock()
	defer s.mu.RUnlock()

	type stat struct {
		sent, deferred, bounced int
		dsnCounts               map[string]int
		lastMsg                 string
		lastMsgTs               int64
	}
	stats := make(map[string]*stat)

	for i := range s.logs {
		e := &s.logs[i]
		if since > 0 && e.Timestamp < since { continue }
		if s.isNDR(e) { continue }
		if e.Relay == "" || e.Relay == "none" { continue }
		if e.Status != "sent" && e.Status != "deferred" && e.Status != "bounced" { continue }

		st, ok := stats[e.Relay]
		if !ok {
			st = &stat{dsnCounts: make(map[string]int)}
			stats[e.Relay] = st
		}
		switch e.Status {
		case "sent":     st.sent++
		case "deferred": st.deferred++
		case "bounced":  st.bounced++
		}
		if e.DSN != "" {
			st.dsnCounts[e.DSN]++
		}
		// track most recent non-empty SMTP message
		if e.SMTPMsg != "" && e.Timestamp > st.lastMsgTs {
			st.lastMsg   = e.SMTPMsg
			st.lastMsgTs = e.Timestamp
		}
	}

	// blocking DSN prefixes — 5.7.x policy, 5.4.1 access denied, 5.x.x permanent
	isBlockDSN := func(dsn string) bool {
		return strings.HasPrefix(dsn, "5.7") || dsn == "5.4.1" || strings.HasPrefix(dsn, "5.")
	}

	var throttling, blocking, healthy []RelayStat
	for relay, st := range stats {
		total := st.sent + st.deferred + st.bounced
		if total == 0 { continue }
		failRate := float64(st.deferred+st.bounced) / float64(total) * 100

		// top DSN
		topDSN, topCount := "", 0
		for dsn, c := range st.dsnCounts {
			if c > topCount { topDSN, topCount = dsn, c }
		}

		rs := RelayStat{
			Relay: relay, Sent: st.sent, Deferred: st.deferred, Bounced: st.bounced,
			Total: total, FailRate: failRate, TopDSN: topDSN, LastMessage: st.lastMsg,
		}

		switch {
		case st.deferred > 0 && float64(st.deferred)/float64(total) > 0.3:
			rs.Kind = "throttling"
			throttling = append(throttling, rs)
		case st.bounced > 0 && isBlockDSN(topDSN) && float64(st.bounced)/float64(total) > 0.1:
			rs.Kind = "blocking"
			blocking = append(blocking, rs)
		default:
			rs.Kind = "ok"
			healthy = append(healthy, rs)
		}
	}

	sortRelays := func(list []RelayStat) {
		sort.Slice(list, func(i, j int) bool {
			return (list[i].Deferred + list[i].Bounced) > (list[j].Deferred + list[j].Bounced)
		})
	}
	sortRelays(throttling)
	sortRelays(blocking)
	sortRelays(healthy)

	if throttling == nil { throttling = []RelayStat{} }
	if blocking   == nil { blocking   = []RelayStat{} }
	if healthy    == nil { healthy    = []RelayStat{} }

	return RelayInsights{Throttling: throttling, Blocking: blocking, Healthy: healthy}
}

// RelayMessage is one delivery attempt for a relay drilldown.
type RelayMessage struct {
	Timestamp int64  `json:"timestamp"`
	To        string `json:"to"`
	Status    string `json:"status"`
	DSN       string `json:"dsn"`
	Message   string `json:"message"`
}

// getRelayMessages returns all delivery attempts for a given relay+status.
// status="" means all statuses.
func (s *logStore) getRelayMessages(relay, status string, since int64) []RelayMessage {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var out []RelayMessage
	for i := range s.logs {
		e := &s.logs[i]
		if since > 0 && e.Timestamp < since { continue }
		if s.isNDR(e) { continue }
		if e.Relay != relay { continue }
		if status != "" && e.Status != status { continue }
		if e.Status != "sent" && e.Status != "deferred" && e.Status != "bounced" { continue }
		out = append(out, RelayMessage{
			Timestamp: e.Timestamp,
			To:        e.To,
			Status:    e.Status,
			DSN:       e.DSN,
			Message:   e.SMTPMsg,
		})
	}
	// sort newest first
	sort.Slice(out, func(i, j int) bool { return out[i].Timestamp > out[j].Timestamp })
	return out
}

// getSenderStats returns per-sender delivery stats, sorted by total volume.
func (s *logStore) getSenderStats(since int64) []SenderStat {
	s.mu.RLock()
	defer s.mu.RUnlock()

	type stat struct{ sent, deferred, bounced int }
	stats := make(map[string]*stat)

	for i := range s.logs {
		e := &s.logs[i]
		if since > 0 && e.Timestamp < since { continue }
		if s.isNDR(e) { continue }
		if e.Status != "sent" && e.Status != "deferred" && e.Status != "bounced" { continue }

		// Resolve sender: use e.From if present, else look up via queueID
		addr := e.From
		if (addr == "" || addr == "<>") && e.QueueID != "" {
			addr = s.queueFrom[e.QueueID]
		}
		if addr == "" || addr == "<>" { continue }

		st, ok := stats[addr]
		if !ok { st = &stat{}; stats[addr] = st }
		switch e.Status {
		case "sent":     st.sent++
		case "deferred": st.deferred++
		case "bounced":  st.bounced++
		}
	}

	out := make([]SenderStat, 0, len(stats))
	for addr, st := range stats {
		total := st.sent + st.deferred + st.bounced
		failRate := 0.0
		if total > 0 { failRate = float64(st.deferred+st.bounced) / float64(total) * 100 }
		out = append(out, SenderStat{Address: addr, Sent: st.sent, Deferred: st.deferred, Bounced: st.bounced, Total: total, FailRate: failRate})
	}

	// Keep only senders with at least one failure, sorted by fail count desc
	filtered := out[:0]
	for _, s := range out {
		if s.Deferred+s.Bounced > 0 {
			filtered = append(filtered, s)
		}
	}
	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].Deferred+filtered[i].Bounced > filtered[j].Deferred+filtered[j].Bounced
	})
	return filtered
}

// ReasonMessage is one delivery attempt matching a reason filter.
type ReasonMessage struct {
	Timestamp int64  `json:"timestamp"`
	QueueID   string `json:"queueId"`
	From      string `json:"from"`
	To        string `json:"to"`
	Status    string `json:"status"`
	DSN       string `json:"dsn"`
	Reason    string `json:"reason"`
	Message   string `json:"message"`
}

// getMessagesByReason returns all delivery attempts whose extracted reason matches.
// status = "deferred" | "bounced"
func (s *logStore) getMessagesByReason(reason, status string, since int64) []ReasonMessage {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var out []ReasonMessage
	for i := range s.logs {
		e := &s.logs[i]
		if since > 0 && e.Timestamp < since { continue }
		if s.isNDR(e) { continue }
		if e.Status != status { continue }

		var extracted string
		switch status {
		case "deferred":
			extracted = extractDeferReason(e.RawLog)
		case "bounced":
			extracted = extractBounceReason(e.RawLog)
		}
		if extracted != reason { continue }

		out = append(out, ReasonMessage{
			Timestamp: e.Timestamp,
			QueueID:   e.QueueID,
			From:      e.From,
			To:        e.To,
			Status:    e.Status,
			DSN:       e.DSN,
			Reason:    extracted,
			Message:   e.SMTPMsg,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Timestamp > out[j].Timestamp })
	return out
}

// getMessagesBySender returns all delivery attempts from a given sender address.
// status = "deferred" | "bounced" | "" (all)
func (s *logStore) getMessagesBySender(from, status string, since int64) []ReasonMessage {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var out []ReasonMessage
	for i := range s.logs {
		e := &s.logs[i]
		if since > 0 && e.Timestamp < since { continue }
		if s.isNDR(e) { continue }
		if e.Status != "sent" && e.Status != "deferred" && e.Status != "bounced" { continue }

		// Resolve sender via queueFrom if e.From is empty
		addr := e.From
		if (addr == "" || addr == "<>") && e.QueueID != "" {
			addr = s.queueFrom[e.QueueID]
		}
		if addr != from { continue }
		if status == "failed" {
			if e.Status != "deferred" && e.Status != "bounced" { continue }
		} else if status != "" && e.Status != status { continue }

		var reason string
		switch e.Status {
		case "deferred": reason = extractDeferReason(e.RawLog)
		case "bounced":  reason = extractBounceReason(e.RawLog)
		}

		out = append(out, ReasonMessage{
			Timestamp: e.Timestamp,
			QueueID:   e.QueueID,
			From:      addr,
			To:        e.To,
			Status:    e.Status,
			DSN:       e.DSN,
			Reason:    reason,
			Message:   e.SMTPMsg,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Timestamp > out[j].Timestamp })
	return out
}

// ── IP Reputation ──────────────────────────────────────────────────────────

var (
	// "Mail from <IP> was refused due to the sender IP found in ERS-DUL"
	blockERSPattern = regexp.MustCompile(`Mail from <([\d.]+)> was refused due to the sender IP found in (ERS[^\s.]+)`)
	// "Client host [IP] blocked using <Service Name>"
	blockClientPattern = regexp.MustCompile(`Client host \[([\d.]+)\] blocked using ([^.(]+)`)
)

func extractBlockEvent(rawLog string) (ourIP, service string, ok bool) {
	if m := blockERSPattern.FindStringSubmatch(rawLog); m != nil {
		return m[1], "TrendMicro " + strings.TrimSpace(m[2]), true
	}
	if m := blockClientPattern.FindStringSubmatch(rawLog); m != nil {
		svc := strings.TrimSpace(m[2])
		// Normalize vendor name variants
		svcLower := strings.ToLower(svc)
		if strings.Contains(svcLower, "trend") {
			svc = "TrendMicro Email Reputation"
		}
		return m[1], svc, true
	}
	return "", "", false
}

func (s *logStore) getIPReputationReport(since int64) IPReputationReport {
	s.mu.RLock()
	defer s.mu.RUnlock()

	type key struct{ ip, service string }
	type stat struct {
		count   int
		domains map[string]struct{}
	}
	stats := make(map[key]*stat)

	for i := range s.logs {
		e := &s.logs[i]
		if since > 0 && e.Timestamp < since { continue }
		if e.Status != "bounced" && e.Status != "deferred" { continue }

		ourIP, service, ok := extractBlockEvent(e.RawLog)
		if !ok { continue }

		domain := ""
		if at := strings.LastIndex(e.To, "@"); at >= 0 {
			domain = e.To[at+1:]
		}

		k := key{ourIP, service}
		if stats[k] == nil {
			stats[k] = &stat{domains: make(map[string]struct{})}
		}
		stats[k].count++
		if domain != "" {
			stats[k].domains[domain] = struct{}{}
		}
	}

	out := make([]BlocklistStat, 0, len(stats))
	for k, v := range stats {
		domains := make([]string, 0, len(v.domains))
		for d := range v.domains {
			domains = append(domains, d)
		}
		sort.Strings(domains)
		out = append(out, BlocklistStat{
			OurIP:   k.ip,
			Service: k.service,
			Count:   v.count,
			Domains: domains,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Count > out[j].Count })
	return IPReputationReport{Stats: out}
}

func (s *logStore) getBlockEvents(ourIP, service string, since int64) []BlockEvent {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var out []BlockEvent
	for i := range s.logs {
		e := &s.logs[i]
		if since > 0 && e.Timestamp < since { continue }
		if e.Status != "bounced" && e.Status != "deferred" { continue }

		ip, svc, ok := extractBlockEvent(e.RawLog)
		if !ok { continue }
		if ip != ourIP || svc != service { continue }

		domain := ""
		if at := strings.LastIndex(e.To, "@"); at >= 0 {
			domain = e.To[at+1:]
		}
		out = append(out, BlockEvent{
			Timestamp:   e.Timestamp,
			QueueID:     e.QueueID,
			OurIP:       ip,
			Service:     svc,
			Destination: domain,
			Recipient:   e.To,
			Message:     e.SMTPMsg,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Timestamp > out[j].Timestamp })
	return out
}

// ── Heatmap ────────────────────────────────────────────────────────────────

func (s *logStore) getHeatmap(since int64) []HeatmapCell {
	s.mu.RLock()
	defer s.mu.RUnlock()

	type key struct{ date string; hour int }
	type cell struct{ sent, defer_, bounce int }
	grid := make(map[key]*cell)

	for i := range s.logs {
		e := &s.logs[i]
		if since > 0 && e.Timestamp < since { continue }
		if s.isNDR(e) { continue }
		if e.Status != "sent" && e.Status != "deferred" && e.Status != "bounced" { continue }

		t := time.UnixMilli(e.Timestamp)
		k := key{date: t.Format("Jan 02"), hour: t.Hour()}
		c := grid[k]
		if c == nil { c = &cell{}; grid[k] = c }
		switch e.Status {
		case "sent":     c.sent++
		case "deferred": c.defer_++
		case "bounced":  c.bounce++
		}
	}

	out := make([]HeatmapCell, 0, len(grid))
	for k, c := range grid {
		out = append(out, HeatmapCell{
			Date:   k.date,
			Hour:   k.hour,
			Sent:   c.sent,
			Defer:  c.defer_,
			Bounce: c.bounce,
			Total:  c.sent + c.defer_ + c.bounce,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Date != out[j].Date { return out[i].Date < out[j].Date }
		return out[i].Hour < out[j].Hour
	})
	return out
}

// ── Retry Timeline ─────────────────────────────────────────────────────────

func (s *logStore) getRetryTimeline(queueID string) RetryTimeline {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var attempts []RetryAttempt
	from, to := "", ""
	finalStatus := ""

	for i := range s.logs {
		e := &s.logs[i]
		if e.QueueID != queueID { continue }
		// Only delivery attempts (skip qmgr active/removed lines)
		if e.Status != "sent" && e.Status != "deferred" && e.Status != "bounced" { continue }
		if e.From != "" && e.From != "<>" && from == "" { from = e.From }
		if e.To != "" && to == "" { to = e.To }
		attempts = append(attempts, RetryAttempt{
			Timestamp: e.Timestamp,
			Status:    e.Status,
			DSN:       e.DSN,
			Relay:     e.Relay,
			Message:   e.SMTPMsg,
		})
		if e.Status == "sent" || e.Status == "bounced" {
			finalStatus = e.Status
		}
	}

	// Resolve from via queueFrom if missing
	if from == "" {
		from = s.queueFrom[queueID]
	}
	if finalStatus == "" && len(attempts) > 0 {
		finalStatus = attempts[len(attempts)-1].Status
	}

	sort.Slice(attempts, func(i, j int) bool { return attempts[i].Timestamp < attempts[j].Timestamp })

	// Fill in delayMs (gap since previous attempt)
	for i := range attempts {
		if i > 0 {
			attempts[i].DelayMs = attempts[i].Timestamp - attempts[i-1].Timestamp
		}
	}

	return RetryTimeline{
		QueueID:     queueID,
		From:        from,
		To:          to,
		Attempts:    attempts,
		FinalStatus: finalStatus,
	}
}

// getTopDeferredQueues returns queue IDs with the most deferred attempts (for retry timeline UI).
func (s *logStore) getTopDeferredQueues(since int64, limit int) []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	counts := make(map[string]int)
	for i := range s.logs {
		e := &s.logs[i]
		if since > 0 && e.Timestamp < since { continue }
		if s.isNDR(e) { continue }
		if e.Status == "deferred" { counts[e.QueueID]++ }
	}

	type kv struct{ id string; n int }
	var sorted []kv
	for id, n := range counts { sorted = append(sorted, kv{id, n}) }
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].n > sorted[j].n })

	out := make([]string, 0, limit)
	for i, kv := range sorted {
		if i >= limit { break }
		out = append(out, kv.id)
	}
	return out
}

// ── Queue Summary ──────────────────────────────────────────────────────────

func (s *logStore) getQueueSummaries(since int64, filterFrom string) []QueueSummary {
	s.mu.RLock()
	defer s.mu.RUnlock()

	type acc struct {
		firstSeen int64
		from      string
		tos       map[string]struct{}
		size      int64
		sent, deferred, bounced int
	}

	queues := make(map[string]*acc)

	for i := range s.logs {
		e := &s.logs[i]
		if since > 0 && e.Timestamp < since { continue }
		if s.isNDR(e) { continue }

		// Resolve sender
		from := e.From
		if (from == "" || from == "<>") && e.QueueID != "" {
			from = s.queueFrom[e.QueueID]
		}
		if from == "" || from == "<>" { continue }
		if filterFrom != "" && from != filterFrom { continue }

		q := queues[e.QueueID]
		if q == nil {
			q = &acc{tos: make(map[string]struct{})}
			queues[e.QueueID] = q
		}

		if q.firstSeen == 0 || e.Timestamp < q.firstSeen { q.firstSeen = e.Timestamp }
		if q.from == "" { q.from = from }
		if e.To != "" { q.tos[e.To] = struct{}{} }
		if e.Size > 0 && q.size == 0 { q.size = e.Size }

		switch e.Status {
		case "sent":     q.sent++
		case "deferred": q.deferred++
		case "bounced":  q.bounced++
		}
	}

	out := make([]QueueSummary, 0, len(queues))
	for qid, q := range queues {
		if q.firstSeen == 0 { continue }
		tos := make([]string, 0, len(q.tos))
		for t := range q.tos { tos = append(tos, t) }
		sort.Strings(tos)

		finalStatus := "deferred"
		if q.sent > 0    { finalStatus = "sent" }
		if q.bounced > 0 { finalStatus = "bounced" }

		out = append(out, QueueSummary{
			Timestamp:   q.firstSeen,
			QueueID:     qid,
			From:        q.from,
			Recipients:  tos,
			RecipCount:  len(tos),
			Size:        q.size,
			FinalStatus: finalStatus,
			Sent:        q.sent,
			Deferred:    q.deferred,
			Bounced:     q.bounced,
		})
	}

	sort.Slice(out, func(i, j int) bool { return out[i].Timestamp > out[j].Timestamp })
	return out
}

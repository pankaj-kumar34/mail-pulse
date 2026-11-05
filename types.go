package main

// ──────────────────────────────────────────────
// Domain types shared across the whole app
// ──────────────────────────────────────────────

type DelayBreakdown struct {
	BeforeQmgr   float64 `json:"beforeQmgr"`
	InQmgr       float64 `json:"inQmgr"`
	ConnSetup    float64 `json:"connSetup"`
	Transmission float64 `json:"transmission"`
}

type ParsedLogEntry struct {
	ID        string          `json:"id"`
	Timestamp int64           `json:"timestamp"` // Unix ms
	Service   string          `json:"service"`
	QueueID   string          `json:"queueId,omitempty"`
	From      string          `json:"from,omitempty"`
	To        string          `json:"to,omitempty"`
	Relay     string          `json:"relay,omitempty"`
	DSN       string          `json:"dsn,omitempty"`
	SMTPMsg   string          `json:"smtpMsg,omitempty"`
	Status    string          `json:"status,omitempty"`
	Delay     float64         `json:"delay,omitempty"`
	Delays    *DelayBreakdown `json:"delays,omitempty"`
	RawLog    string          `json:"rawLog,omitempty"`
	IsError   bool            `json:"isError"`
	IsWarning bool            `json:"isWarning"`
	Size      int64           `json:"size,omitempty"`  // from qmgr: size=N
	Nrcpt     int             `json:"nrcpt,omitempty"` // from qmgr: nrcpt=N
}

type DomainCount struct {
	Domain string `json:"domain"`
	Count  int    `json:"count"`
}

type DomainDelivery struct {
	Domain   string  `json:"domain"`
	Sent     int     `json:"sent"`
	Deferred int     `json:"deferred"`
	Bounced  int     `json:"bounced"`
	Total    int     `json:"total"`
	FailRate float64 `json:"failRate"` // (deferred+bounced)/total %
}

type DeferReason struct {
	Reason  string  `json:"reason"`
	Count   int     `json:"count"`
	Percent float64 `json:"percent"`
}

type HourlyVolume struct {
	Hour     int64 `json:"hour"`     // unix timestamp of hour start (milliseconds)
	Sent     int   `json:"sent"`
	Deferred int   `json:"deferred"`
	Bounced  int   `json:"bounced"`
}

type VolumeInsights struct {
	Hourly         []HourlyVolume   `json:"hourly"`
	DeferReasons   []DeferReason    `json:"deferReasons"`
	BounceReasons  []DeferReason    `json:"bounceReasons"` // reuse same shape
	FailingDomains []DomainDelivery `json:"failingDomains"`
}

type TimeSeriesPoint struct {
	Timestamp int64   `json:"timestamp"`
	Value     float64 `json:"value"`
}

type DelayDistribution struct {
	Range      string  `json:"range"`
	Count      int     `json:"count"`
	Percentage float64 `json:"percentage"`
}

type DelayInsights struct {
	AverageBeforeQmgr   float64             `json:"averageBeforeQmgr"`
	AverageInQmgr       float64             `json:"averageInQmgr"`
	AverageConnSetup    float64             `json:"averageConnSetup"`
	AverageTransmission float64             `json:"averageTransmission"`
	MaxDelay            float64             `json:"maxDelay"`
	MinDelay            float64             `json:"minDelay"`
	AverageDelay        float64             `json:"averageDelay"`
	MinBeforeQmgr       float64             `json:"minBeforeQmgr"`
	MaxBeforeQmgr       float64             `json:"maxBeforeQmgr"`
	MinInQmgr           float64             `json:"minInQmgr"`
	MaxInQmgr           float64             `json:"maxInQmgr"`
	MinConnSetup        float64             `json:"minConnSetup"`
	MaxConnSetup        float64             `json:"maxConnSetup"`
	MinTransmission     float64             `json:"minTransmission"`
	MaxTransmission     float64             `json:"maxTransmission"`
	DelayDistribution   []DelayDistribution `json:"delayDistribution"`
	DelayTrend          []TimeSeriesPoint   `json:"delayTrend"`
	BeforeQmgrTrend     []TimeSeriesPoint   `json:"beforeQmgrTrend"`
	InQmgrTrend         []TimeSeriesPoint   `json:"inQmgrTrend"`
	ConnSetupTrend      []TimeSeriesPoint   `json:"connSetupTrend"`
	TransmissionTrend   []TimeSeriesPoint   `json:"transmissionTrend"`
}

type Metrics struct {
	TotalSent           int               `json:"totalSent"`
	TotalDeferred       int               `json:"totalDeferred"`
	TotalBounced        int               `json:"totalBounced"`
	TotalRemoved        int               `json:"totalRemoved"`
	ErrorCount          int               `json:"errorCount"`
	WarningCount        int               `json:"warningCount"`
	AverageDelay        float64           `json:"averageDelay"`
	DeliverySuccessRate float64           `json:"deliverySuccessRate"` // % of sent out of (sent+deferred+bounced)
	TopSenderDomains    []DomainCount     `json:"topSenderDomains"`
	TopRecipientDomains []DomainCount     `json:"topRecipientDomains"`
	QmgrActiveHistory   []TimeSeriesPoint `json:"qmgrActiveHistory"`
	QmgrRemovedHistory  []TimeSeriesPoint `json:"qmgrRemovedHistory"`
}

type PaginatedLogs struct {
	Logs       []ParsedLogEntry `json:"logs"`
	Pagination PaginationInfo   `json:"pagination"`
}

// MessageRow is one grouped delivery attempt: one queue ID + one recipient.
type MessageRow struct {
	QueueID     string  `json:"queueId"`
	Time        int64   `json:"time"`        // timestamp of final status line (ms)
	From        string  `json:"from"`
	To          string  `json:"to"`
	Status      string  `json:"status"`      // final status: sent/deferred/bounced/removed/active
	Delay       float64 `json:"delay"`       // seconds
	ProcessTime float64 `json:"processTime"` // time from queue entry to final status (seconds)
}

type PaginatedMessages struct {
	Rows       []MessageRow   `json:"rows"`
	Pagination PaginationInfo `json:"pagination"`
}

type PaginationInfo struct {
	Page       int  `json:"page"`
	Limit      int  `json:"limit"`
	Total      int  `json:"total"`
	TotalPages int  `json:"totalPages"`
	HasNext    bool `json:"hasNext"`
	HasPrev    bool `json:"hasPrev"`
}

type MailQueueRecipient struct {
	Address     string `json:"address"`
	DelayReason string `json:"delay_reason,omitempty"`
}

type MailQueueEntry struct {
	QueueName    string               `json:"queue_name"`
	QueueID      string               `json:"queue_id"`
	ArrivalTime  int64                `json:"arrival_time"`
	MessageSize  int64                `json:"message_size"`
	ForcedExpire bool                 `json:"forced_expire"`
	Sender       string               `json:"sender"`
	Recipients   []MailQueueRecipient `json:"recipients"`
}

type PaginatedQueue struct {
	Entries    []MailQueueEntry `json:"entries"`
	Pagination PaginationInfo   `json:"pagination"`
	PodID      string           `json:"podId"`
}

type SearchMatch struct {
	LineNumber int    `json:"lineNumber"`
	Text       string `json:"text"`
}

// QueueAgeBucket is one bucket in the delay distribution histogram.
type QueueAgeBucket struct {
	Label   string  `json:"label"`
	Min     float64 `json:"min"`   // seconds
	Max     float64 `json:"max"`   // seconds (-1 = unbounded)
	Count   int     `json:"count"`
	Percent float64 `json:"percent"`
}

// QueueAgeDistribution is the result of GetQueueAgeDistribution.
type QueueAgeDistribution struct {
	Buckets    []QueueAgeBucket `json:"buckets"`
	Total      int              `json:"total"`
	MedianSecs float64          `json:"medianSecs"`
	P95Secs    float64          `json:"p95Secs"`
	MaxSecs    float64          `json:"maxSecs"`
}

// RecipientStat is delivery stats for a single recipient address.
type RecipientStat struct {
	Address   string  `json:"address"`
	Domain    string  `json:"domain"`
	Sent      int     `json:"sent"`
	Deferred  int     `json:"deferred"`
	Bounced   int     `json:"bounced"`
	Total     int     `json:"total"`
	FailRate  float64 `json:"failRate"`
	LastSeen  int64   `json:"lastSeen"`  // unix ms of most recent delivery attempt
	LastStatus string `json:"lastStatus"` // most recent status
	LastReason string `json:"lastReason"` // extracted defer/bounce reason
}

// RecipientDrilldown is the result of GetRecipientDrilldown.
type RecipientDrilldown struct {
	Domain     string          `json:"domain"`
	Recipients []RecipientStat `json:"recipients"`
	// Domain-level totals
	Sent      int     `json:"sent"`
	Deferred  int     `json:"deferred"`
	Bounced   int     `json:"bounced"`
	Total     int     `json:"total"`
	FailRate  float64 `json:"failRate"`
}

// IndexFileResult is the typed result of IndexFile.
type IndexFileResult struct {
	Total int    `json:"total"`
	Error string `json:"error,omitempty"`
}

// GetLinesResult is the typed result of GetLines.
type GetLinesResult struct {
	Lines []string `json:"lines"`
	Total int      `json:"total"`
}

// SearchLinesResult is the typed result of SearchLines.
type SearchLinesResult struct {
	Matches []SearchMatch `json:"matches"`
	Total   int           `json:"total"`
}

// PollNewLinesResult is the typed result of PollNewLines.
type PollNewLinesResult struct {
	Lines    []string `json:"lines"`
	Total    int      `json:"total"`
	NewCount int      `json:"newCount"`
}

// RelayStat holds delivery stats for a single relay host.
type RelayStat struct {
	Relay       string  `json:"relay"`
	Sent        int     `json:"sent"`
	Deferred    int     `json:"deferred"`
	Bounced     int     `json:"bounced"`
	Total       int     `json:"total"`
	FailRate    float64 `json:"failRate"`
	Kind        string  `json:"kind"` // "throttling" | "blocking" | "ok"
	TopDSN      string  `json:"topDsn"`
	LastMessage string  `json:"lastMessage"` // full SMTP response text
}

// RelayInsights is the result of GetRelayInsights.
type RelayInsights struct {
	Throttling []RelayStat `json:"throttling"` // relays deferring the most
	Blocking   []RelayStat `json:"blocking"`   // relays bouncing with policy/block codes
	Healthy    []RelayStat `json:"healthy"`    // relays with good delivery
}

// SenderStat holds delivery stats for a single sender address.
type SenderStat struct {
	Address  string  `json:"address"`
	Sent     int     `json:"sent"`
	Deferred int     `json:"deferred"`
	Bounced  int     `json:"bounced"`
	Total    int     `json:"total"`
	FailRate float64 `json:"failRate"`
}

// BlockEvent is a single IP-reputation block event.
type BlockEvent struct {
	Timestamp   int64  `json:"timestamp"`
	QueueID     string `json:"queueId"`
	OurIP       string `json:"ourIp"`
	Service     string `json:"service"`    // e.g. "TrendMicro ERS-DUL"
	Destination string `json:"destination"` // domain
	Recipient   string `json:"recipient"`
	Message     string `json:"message"`
}

// BlocklistStat summarises blocks by a single service/IP pair.
type BlocklistStat struct {
	OurIP   string   `json:"ourIp"`
	Service string   `json:"service"`
	Count   int      `json:"count"`
	Domains []string `json:"domains"` // unique affected domains
}

// IPReputationReport is the full report returned to the frontend.
type IPReputationReport struct {
	Stats []BlocklistStat `json:"stats"`
}

// HeatmapCell is one cell in the hourly volume heatmap.
type HeatmapCell struct {
	Date      string `json:"date"`      // "Nov 03" — display only
	DateEpoch int64  `json:"dateEpoch"` // unix ms of midnight for correct chronological sort
	Hour      int    `json:"hour"`
	Sent      int    `json:"sent"`
	Defer     int    `json:"defer"`
	Bounce    int    `json:"bounce"`
	Total     int    `json:"total"`
}

// RetryAttempt is one delivery attempt for a queue ID.
type RetryAttempt struct {
	Timestamp int64  `json:"timestamp"`
	Status    string `json:"status"`
	DSN       string `json:"dsn"`
	Relay     string `json:"relay"`
	Message   string `json:"message"`
	DelayMs   int64  `json:"delayMs"` // time since previous attempt
}

// RetryTimeline is the full retry history for a queue ID.
type RetryTimeline struct {
	QueueID   string         `json:"queueId"`
	From      string         `json:"from"`
	To        string         `json:"to"`
	Attempts  []RetryAttempt `json:"attempts"`
	FinalStatus string       `json:"finalStatus"`
}

// QueueSummary is a per-queue-ID summary: one row per queue, with all recipients aggregated.
type QueueSummary struct {
	Timestamp    int64    `json:"timestamp"`    // first seen
	QueueID      string   `json:"queueId"`
	From         string   `json:"from"`
	Recipients   []string `json:"recipients"`   // all unique To addresses
	RecipCount   int      `json:"recipCount"`
	Size         int64    `json:"size"`         // bytes
	FinalStatus  string   `json:"finalStatus"`  // sent/deferred/bounced
	Sent         int      `json:"sent"`
	Deferred     int      `json:"deferred"`
	Bounced      int      `json:"bounced"`
}

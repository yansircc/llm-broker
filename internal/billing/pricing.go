package billing

import (
	"encoding/json"

	"github.com/yansircc/llm-broker/internal/domain"
	"github.com/yansircc/llm-broker/internal/driver"
)

const microsPerMillion = int64(1_000_000)

type PriceSnapshot struct {
	Model                       string `json:"model"`
	InputMicrosPerMillion       int64  `json:"input_micros_per_million"`
	OutputMicrosPerMillion      int64  `json:"output_micros_per_million"`
	CacheReadMicrosPerMillion   int64  `json:"cache_read_micros_per_million"`
	CacheCreateMicrosPerMillion int64  `json:"cache_create_micros_per_million"`
}

func Snapshot(price *domain.ModelPrice) PriceSnapshot {
	if price == nil {
		return PriceSnapshot{}
	}
	return PriceSnapshot{
		Model:                       price.Model,
		InputMicrosPerMillion:       price.InputMicrosPerMillion,
		OutputMicrosPerMillion:      price.OutputMicrosPerMillion,
		CacheReadMicrosPerMillion:   price.CacheReadMicrosPerMillion,
		CacheCreateMicrosPerMillion: price.CacheCreateMicrosPerMillion,
	}
}

func SnapshotJSON(snapshot PriceSnapshot) string {
	data, _ := json.Marshal(snapshot)
	return string(data)
}

func ChargeMicros(usage *driver.Usage, price *domain.ModelPrice) int64 {
	if usage == nil || price == nil {
		return 0
	}
	return tokenCharge(usage.InputTokens, price.InputMicrosPerMillion) +
		tokenCharge(usage.OutputTokens, price.OutputMicrosPerMillion) +
		tokenCharge(usage.CacheReadTokens, price.CacheReadMicrosPerMillion) +
		tokenCharge(usage.CacheCreateTokens, price.CacheCreateMicrosPerMillion)
}

func tokenCharge(tokens int, microsPerMillionTokens int64) int64 {
	if tokens <= 0 || microsPerMillionTokens <= 0 {
		return 0
	}
	n := int64(tokens) * microsPerMillionTokens
	return (n + microsPerMillion - 1) / microsPerMillion
}

package hook

import (
	"fmt"
	"gogogot/internal/infra/utils"
	"gogogot/internal/llm/types"
	"strings"
)

func CalcCost(inputPerM, outputPerM float64, inputTokens, outputTokens int) float64 {
	return float64(inputTokens)/1_000_000*inputPerM +
		float64(outputTokens)/1_000_000*outputPerM
}

func textFromBlocks(blocks []types.ContentBlock) string {
	var sb strings.Builder
	for _, b := range blocks {
		if b.Type == "text" && b.Text != "" {
			if sb.Len() > 0 {
				sb.WriteByte(' ')
			}
			sb.WriteString(b.Text)
		}
	}
	return sb.String()
}

func blockTypeSummary(blocks []types.ContentBlock) string {
	seen := make(map[string]int)
	for _, b := range blocks {
		seen[b.Type]++
	}
	var parts []string
	for t, n := range seen {
		if n == 1 {
			parts = append(parts, t)
		} else {
			parts = append(parts, fmt.Sprintf("%s×%d", t, n))
		}
	}
	return strings.Join(parts, ",")
}

func truncateStr(s string, max int) string {
	return utils.Truncate(s, max, "…")
}
